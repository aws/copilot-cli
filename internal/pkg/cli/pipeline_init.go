// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store/secretsmanager"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/command"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/version"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/aws/amazon-ecs-cli-v2/templates"
	"github.com/gobuffalo/packd"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	pipelineAddEnvPrompt              = "Would you like to add an environment to your pipeline?"
	pipelineAddMoreEnvPrompt          = "Would you like to add another environment to your pipeline?"
	pipelineAddEnvHelpPrompt          = "Adds an environment that corresponds to a deployment stage in your pipeline. Environments are added sequentially."
	pipelineAddMoreEnvHelpPrompt      = "Adds another environment that corresponds to a deployment stage in your pipeline. Environments are added sequentially."
	pipelineSelectEnvPrompt           = "Which environment would you like to add to your pipeline?"
	pipelineSelectGitHubURLPrompt     = "Which GitHub repository would you like to use for your application?"
	pipelineSelectGitHubURLHelpPrompt = `The GitHub repository linked to your workspace.
Pushing to this repository will trigger your pipeline build stage.
Please enter full repository URL, e.g. "https://github.com/myCompany/myRepo", or the owner/rep, e.g. "myCompany/myRepo"`
)

const (
	buildspecTemplatePath = "cicd/buildspec.yml"
	githubURL             = "github.com"
	masterBranch          = "master"
)

var (
	// Filled in via the -ldflags flag at compile time to support pipeline buildspec CLI pulling.
	binaryS3BucketPath string
)

var errNoEnvsInProject = errors.New("there were no more environments found that can be added to your pipeline. Please run `dw_run.sh env init` to create a new environment")

// InitPipelineOpts holds the configuration needed to create a new pipeilne
type InitPipelineOpts struct {
	// Fields with matching flags.
	Environments      []string
	GitHubOwner       string
	GitHubRepo        string
	GitHubURL         string
	GitHubAccessToken string
	GitBranch         string
	PipelineFilename  string
	// TODO add pipeline file (to write to different file than pipeline.yml?)

	// Interfaces to interact with dependencies.
	workspace      archer.ManifestIO
	secretsmanager archer.SecretsManager
	box            packd.Box
	runner         runner

	// Outputs stored on successful actions.
	manifestPath  string
	buildspecPath string
	secretName    string

	// Caches variables
	projectEnvs []string
	repoURLs    []string
	fsUtils     *afero.Afero
	buffer      bytes.Buffer

	*GlobalOpts
}

// NewInitPipelineOpts returns a new InitPipelineOpts struct.
func NewInitPipelineOpts() *InitPipelineOpts {
	return &InitPipelineOpts{
		runner:     command.New(),
		fsUtils:    &afero.Afero{Fs: afero.NewOsFs()},
		GlobalOpts: NewGlobalOpts(),
	}
}

// Ask prompts for fields that are required but not passed in.
func (opts *InitPipelineOpts) Ask() error {
	var err error
	if len(opts.Environments) == 0 {
		if err = opts.selectEnvironments(); err != nil {
			return err
		}
	}

	if opts.GitHubURL == "" {
		if err = opts.selectGitHubURL(); err != nil {
			return err
		}
	}
	if opts.GitHubOwner, opts.GitHubRepo, err = opts.parseOwnerRepoName(opts.GitHubURL); err != nil {
		return err
	}

	if opts.GitHubAccessToken == "" {
		if err = opts.getGitHubAccessToken(); err != nil {
			return err
		}
	}

	if opts.GitBranch == "" {
		opts.GitBranch = masterBranch
	}

	return nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (opts *InitPipelineOpts) Validate() error {
	// TODO add validation for flags
	if opts.ProjectName() == "" {
		return errNoProjectInWorkspace
	}

	return nil
}

// Execute writes the pipeline manifest file.
func (opts *InitPipelineOpts) Execute() error {
	secretName := opts.createSecretName()
	_, err := opts.secretsmanager.CreateSecret(secretName, opts.GitHubAccessToken)

	if err != nil {
		var existsErr *secretsmanager.ErrSecretAlreadyExists
		if !errors.As(err, &existsErr) {
			return err
		}
		log.Successf("Secret already exists for %s! Do nothing.\n", color.HighlightUserInput(opts.GitHubRepo))
	}
	opts.secretName = secretName

	// write pipeline.yml file, populate with:
	//   - github repo as source
	//   - stage names (environments)
	//   - enable/disable transition to prod envs

	manifestPath, err := opts.createPipelineManifest()
	if err != nil {
		return err
	}
	opts.manifestPath = manifestPath

	buildspecPath, err := opts.createBuildspec()
	if err != nil {
		return err
	}
	opts.buildspecPath = buildspecPath

	log.Successf("Wrote the pipeline manifest for %s at '%s'\n", color.HighlightUserInput(opts.GitHubRepo), color.HighlightResource(relPath(opts.manifestPath)))
	log.Successf("Wrote the buildspec for the pipeline's build stage at '%s'\n", color.HighlightResource(relPath(opts.buildspecPath)))
	log.Infoln("The manifest contains configurations for your CodePipeline resources, such as your pipeline stages and build steps.")
	log.Infoln("The buildspec contains the commands to build and push your container images to your ECR repositories.")

	// TODO deploy manifest file

	return nil
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (opts *InitPipelineOpts) RecommendedActions() []string {
	return []string{
		fmt.Sprintf("Update the %s phase of your buildspec to unit test your applications before pushing the images.", color.HighlightResource("build")),
		fmt.Sprint("Update your pipeline manifest to add additional stages."),
		fmt.Sprintf("Run %s to deploy your pipeline for the repository.", color.HighlightCode("dw_run.sh pipeline update")),
	}
}

func (opts *InitPipelineOpts) createSecretName() string {
	return fmt.Sprintf("github-token-%s-%s", opts.projectName, opts.GitHubRepo)
}

func (opts *InitPipelineOpts) createPipelineName() string {
	return fmt.Sprintf("pipeline-%s-%s-%s", opts.projectName, opts.GitHubOwner, opts.GitHubRepo)
}

func (opts *InitPipelineOpts) createPipelineProvider() (manifest.Provider, error) {
	config := &manifest.GitHubProperties{
		OwnerAndRepository:    "https://" + githubURL + "/" + opts.GitHubOwner + "/" + opts.GitHubRepo,
		Branch:                opts.GitBranch,
		GithubSecretIdKeyName: opts.secretName,
	}

	return manifest.NewProvider(config)
}

func (opts *InitPipelineOpts) createPipelineManifest() (string, error) {
	// TODO change this to flag
	pipelineName := opts.createPipelineName()
	provider, err := opts.createPipelineProvider()
	if err != nil {
		return "", fmt.Errorf("could not create pipeline: %w", err)
	}

	manifest, err := manifest.CreatePipeline(pipelineName, provider, opts.Environments)
	if err != nil {
		return "", fmt.Errorf("generate a manifest: %w", err)
	}

	manifestBytes, err := manifest.Marshal()
	if err != nil {
		return "", fmt.Errorf("marshal manifest: %w", err)
	}
	manifestPath, err := opts.workspace.WriteFile(manifestBytes, workspace.PipelineFileName)
	if err != nil {
		return "", fmt.Errorf("write file %s to workspace: %w", workspace.PipelineFileName, err)
	}

	return manifestPath, nil
}

func (opts *InitPipelineOpts) createBuildspec() (string, error) {
	content, err := opts.box.FindString(buildspecTemplatePath)
	if err != nil {
		return "", fmt.Errorf("find template for %s: %w", buildspecTemplatePath, err)
	}

	tmpl, err := template.New("cicd-buildspec").Parse(content)
	if err != nil {
		return "", err
	}
	buf := bytes.Buffer{}
	type cicdBuildspecTemplate struct {
		BinaryS3BucketPath string
		Version            string
	}
	if err := tmpl.Execute(&buf, cicdBuildspecTemplate{BinaryS3BucketPath: binaryS3BucketPath, Version: version.Version}); err != nil {
		return "", err
	}

	path, err := opts.workspace.WriteFile(buf.Bytes(), workspace.BuildspecFileName)
	if err != nil {
		return "", fmt.Errorf("write file %s to workspace: %w", workspace.BuildspecFileName, err)
	}
	return path, nil
}

func (opts *InitPipelineOpts) selectEnvironments() error {
	for {
		promptMsg := pipelineAddEnvPrompt
		promptHelpMsg := pipelineAddEnvHelpPrompt
		if len(opts.Environments) > 0 {
			promptMsg = pipelineAddMoreEnvPrompt
			promptHelpMsg = pipelineAddMoreEnvHelpPrompt
		}
		addEnv, err := opts.prompt.Confirm(promptMsg, promptHelpMsg)
		if err != nil {
			return fmt.Errorf("confirm adding an environment: %w", err)
		}
		if !addEnv {
			break
		}
		if err := opts.selectEnvironment(); err != nil {
			return fmt.Errorf("add environment: %w", err)
		}
		if len(opts.listAvailableEnvironments()) == 0 {
			break
		}
	}

	return nil
}

func (opts *InitPipelineOpts) listAvailableEnvironments() []string {
	var envs []string
	for _, env := range opts.projectEnvs {
		// Check if environment has already been added to pipeline
		if opts.envCanBeAdded(env) {
			envs = append(envs, env)
		}
	}

	return envs
}

func (opts *InitPipelineOpts) envCanBeAdded(selectedEnv string) bool {
	for _, env := range opts.Environments {
		if selectedEnv == env {
			return false
		}
	}

	return true
}

func (opts *InitPipelineOpts) selectEnvironment() error {
	envs := opts.listAvailableEnvironments()

	if len(envs) == 0 && len(opts.Environments) != 0 {
		log.Infoln("There are no more environments to add.")
		return nil
	}

	env, err := opts.prompt.SelectOne(
		pipelineSelectEnvPrompt,
		"Environment to be added as the next stage in your pipeline.",
		envs,
	)

	if err != nil {
		return err
	}

	opts.Environments = append(opts.Environments, env)

	return nil
}

// relPath returns the full path relative to the current working directory.
// If there is an error during the process, returns the full path.
func relPath(fullPath string) string {
	wkdir, err := os.Getwd()
	if err != nil {
		return fullPath
	}
	relPath, err := filepath.Rel(wkdir, fullPath)
	if err != nil {
		return fullPath
	}
	return relPath
}

func (opts *InitPipelineOpts) selectGitHubURL() error {
	url, err := opts.prompt.SelectOne(
		pipelineSelectGitHubURLPrompt,
		pipelineSelectGitHubURLHelpPrompt,
		opts.repoURLs,
	)
	if err != nil {
		return fmt.Errorf("select GitHub URL: %w", err)
	}
	opts.GitHubURL = url

	return nil
}

func (opts *InitPipelineOpts) parseOwnerRepoName(url string) (string, string, error) {
	regexPattern := regexp.MustCompile(`.*(github.com)(:|\/)`)
	parsedURL := strings.TrimPrefix(url, regexPattern.FindString(url))
	ownerRepo := strings.Split(parsedURL, string(os.PathSeparator))
	if len(ownerRepo) != 2 {
		return "", "", fmt.Errorf("unable to parse the GitHub repository owner and name from %s: please pass the repository URL with the format `--url https://github.com/{owner}/{repositoryName}`", url)
	}
	return ownerRepo[0], ownerRepo[1], nil
}

// examples:
// efekarakus	git@github.com:efekarakus/grit.git (fetch)
// efekarakus	https://github.com/karakuse/grit.git (fetch)
// origin	    https://github.com/koke/grit (fetch)
// koke       git://github.com/koke/grit.git (push)
func (opts *InitPipelineOpts) parseGitRemoteResult(s string) ([]string, error) {
	var urls []string
	urlSet := make(map[string]bool)
	items := strings.Split(s, "\n")
	for _, item := range items {
		if !strings.Contains(item, githubURL) {
			continue
		}
		cols := strings.Split(item, "\t")
		url := strings.TrimSpace(strings.TrimSuffix(strings.Split(cols[1], " ")[0], ".git"))
		urlSet[url] = true
	}
	for url := range urlSet {
		urls = append(urls, url)
	}
	return urls, nil
}

func (opts *InitPipelineOpts) getGitHubAccessToken() error {
	token, err := opts.prompt.GetSecret(
		fmt.Sprintf("Please enter your GitHub Personal Access Token for your repository: %s", opts.GitHubRepo),
		fmt.Sprintf(`The personal access token for the GitHub repository linked to your workspace. For more information on how to create a personal access token, please refer to: https://help.github.com/en/enterprise/2.17/user/authenticating-to-github/creating-a-personal-access-token-for-the-command-line.`),
	)

	if err != nil {
		return fmt.Errorf("get GitHub access token: %w", err)
	}

	opts.GitHubAccessToken = token

	return nil
}

func (opts *InitPipelineOpts) getEnvNames() ([]string, error) {
	store, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to environment datastore: %w", err)
	}

	envs, err := store.ListEnvironments(opts.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("could not list environments for project %s: %w", opts.ProjectName(), err)
	}

	if len(envs) == 0 {
		return nil, errNoEnvsInProject
	}

	var envNames []string
	for _, env := range envs {
		envNames = append(envNames, env.Name)
	}

	return envNames, nil
}

// BuildPipelineInitCmd build the command for creating a new pipeline.
func BuildPipelineInitCmd() *cobra.Command {
	opts := NewInitPipelineOpts()
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a pipeline for applications in your workspace.",
		Long:  `Creates a pipeline for the applications in your workspace, using the environments associated with the applications.`,
		Example: `
  Create a pipeline for the applications in your workspace:
	/code $ dw_run.sh pipeline init \
	  /code  --github-url https://github.com/gitHubUserName/myFrontendApp.git \
	  /code  --github-access-token file://myGitHubToken \
	  /code  --environments "dev,prod" \
	  /code  --deploy`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if err := opts.Validate(); err != nil {
				return err
			}

			// TODO: move these logic to a method
			projectEnvs, err := opts.getEnvNames()
			if err != nil {
				return fmt.Errorf("couldn't get environments: %w", err)
			}
			if len(projectEnvs) == 0 {
				return errNoEnvsInProject
			}
			opts.projectEnvs = projectEnvs

			ws, err := workspace.New()
			if err != nil {
				return fmt.Errorf("workspace cannot be created: %w", err)
			}
			opts.workspace = ws

			secretsmanager, err := secretsmanager.NewStore()
			if err != nil {
				return fmt.Errorf("couldn't create secrets manager: %w", err)
			}
			opts.secretsmanager = secretsmanager
			opts.box = templates.Box()

			err = opts.runner.Run("git", []string{"remote", "-v"}, command.Stdout(&opts.buffer))
			if err != nil {
				return fmt.Errorf("get remote repository info: %w, run `git remote add` first please", err)
			}
			urls, err := opts.parseGitRemoteResult(strings.TrimSpace(opts.buffer.String()))
			if err != nil {
				return err
			}
			opts.repoURLs = urls
			opts.buffer.Reset()

			return nil
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
		PostRunE: func(cmd *cobra.Command, args []string) error {
			log.Infoln()
			log.Infoln("Recommended follow-up actions:")
			for _, followup := range opts.RecommendedActions() {
				log.Infof("- %s\n", followup)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&opts.GitHubURL, githubURLFlag, githubURLFlagShort, "", githubURLFlagDescription)
	cmd.Flags().StringVarP(&opts.GitHubAccessToken, githubAccessTokenFlag, githubAccessTokenFlagShort, "", githubAccessTokenFlagDescription)
	cmd.Flags().StringVarP(&opts.GitBranch, gitBranchFlag, gitBranchFlagShort, "", gitBranchFlagDescription)
	cmd.Flags().StringSliceVarP(&opts.Environments, envsFlag, envsFlagShort, []string{}, pipelineEnvsFlagDescription)

	return cmd
}

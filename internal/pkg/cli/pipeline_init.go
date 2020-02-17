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
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/secretsmanager"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
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

var errNoEnvsInProject = errors.New("there were no more environments found that can be added to your pipeline. Please run `ecs-preview env init` to create a new environment")

type initPipelineVars struct {
	Environments      []string
	GitHubOwner       string
	GitHubRepo        string
	GitHubURL         string
	GitHubAccessToken string
	GitBranch         string
	*GlobalOpts
}

// binaryBuffer is a bytes.Buffer that implements the encoding.BinaryMarshaler interface.
type binaryBuffer struct {
	*bytes.Buffer
}

// MarshalBinary returns the bytes of the underlying buffer.
func (b binaryBuffer) MarshalBinary() ([]byte, error) {
	return b.Bytes(), nil
}

type initPipelineOpts struct {
	// TODO add pipeline file (to write to different file than pipeline.yml?)
	initPipelineVars
	// Interfaces to interact with dependencies.
	workspace      wsPipelineWriter
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
}

func newInitPipelineOpts(vars initPipelineVars) (*initPipelineOpts, error) {
	opts := &initPipelineOpts{
		initPipelineVars: vars,
		runner:           command.New(),
		fsUtils:          &afero.Afero{Fs: afero.NewOsFs()},
	}
	projectEnvs, err := opts.getEnvNames()
	if err != nil {
		return nil, fmt.Errorf("couldn't get environments: %w", err)
	}
	if len(projectEnvs) == 0 {
		return nil, errNoEnvsInProject
	}
	opts.projectEnvs = projectEnvs

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}
	opts.workspace = ws

	secretsmanager, err := secretsmanager.NewStore()
	if err != nil {
		return nil, fmt.Errorf("couldn't create secrets manager: %w", err)
	}
	opts.secretsmanager = secretsmanager
	opts.box = templates.Box()

	err = opts.runner.Run("git", []string{"remote", "-v"}, command.Stdout(&opts.buffer))
	if err != nil {
		return nil, fmt.Errorf("get remote repository info: %w, run `git remote add` first please", err)
	}
	urls, err := opts.parseGitRemoteResult(strings.TrimSpace(opts.buffer.String()))
	if err != nil {
		return nil, err
	}
	opts.repoURLs = urls
	opts.buffer.Reset()

	return opts, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *initPipelineOpts) Validate() error {
	// TODO add validation for flags
	if o.ProjectName() == "" {
		return errNoProjectInWorkspace
	}

	return nil
}

// Ask prompts for fields that are required but not passed in.
func (o *initPipelineOpts) Ask() error {
	var err error
	if len(o.Environments) == 0 {
		if err = o.selectEnvironments(); err != nil {
			return err
		}
	}

	if o.GitHubURL == "" {
		if err = o.selectGitHubURL(); err != nil {
			return err
		}
	}
	if o.GitHubOwner, o.GitHubRepo, err = o.parseOwnerRepoName(o.GitHubURL); err != nil {
		return err
	}

	if o.GitHubAccessToken == "" {
		if err = o.getGitHubAccessToken(); err != nil {
			return err
		}
	}

	if o.GitBranch == "" {
		o.GitBranch = masterBranch
	}

	return nil
}

// Execute writes the pipeline manifest file.
func (o *initPipelineOpts) Execute() error {
	secretName := o.createSecretName()
	_, err := o.secretsmanager.CreateSecret(secretName, o.GitHubAccessToken)

	if err != nil {
		var existsErr *secretsmanager.ErrSecretAlreadyExists
		if !errors.As(err, &existsErr) {
			return err
		}
		log.Successf("Secret already exists for %s! Do nothing.\n", color.HighlightUserInput(o.GitHubRepo))
	} else {
		log.Successf("Created the secret %s for pipeline source stage!\n", color.HighlightUserInput(secretName))
	}
	o.secretName = secretName

	// write pipeline.yml file, populate with:
	//   - github repo as source
	//   - stage names (environments)
	//   - enable/disable transition to prod envs

	manifestPath, err := o.createPipelineManifest()
	if err != nil {
		return err
	}
	o.manifestPath = manifestPath

	buildspecPath, err := o.createBuildspec()
	if err != nil {
		return err
	}
	o.buildspecPath = buildspecPath

	log.Successf("Wrote the pipeline manifest for %s at '%s'\n", color.HighlightUserInput(o.GitHubRepo), color.HighlightResource(relPath(o.manifestPath)))
	log.Successf("Wrote the buildspec for the pipeline's build stage at '%s'\n", color.HighlightResource(relPath(o.buildspecPath)))
	log.Infoln("The manifest contains configurations for your CodePipeline resources, such as your pipeline stages and build steps.")
	log.Infoln("The buildspec contains the commands to build and push your container images to your ECR repositories.")

	// TODO deploy manifest file

	return nil
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *initPipelineOpts) RecommendedActions() []string {
	return []string{
		fmt.Sprintf("Update the %s phase of your buildspec to unit test your applications before pushing the images.", color.HighlightResource("build")),
		fmt.Sprint("Update your pipeline manifest to add additional stages."),
		fmt.Sprintf("Run %s to deploy your pipeline for the repository.", color.HighlightCode("ecs-preview pipeline update")),
	}
}

func (o *initPipelineOpts) createSecretName() string {
	return fmt.Sprintf("github-token-%s-%s", o.projectName, o.GitHubRepo)
}

func (o *initPipelineOpts) createPipelineName() string {
	return fmt.Sprintf("pipeline-%s-%s-%s", o.projectName, o.GitHubOwner, o.GitHubRepo)
}

func (o *initPipelineOpts) createPipelineProvider() (manifest.Provider, error) {
	config := &manifest.GitHubProperties{
		OwnerAndRepository:    "https://" + githubURL + "/" + o.GitHubOwner + "/" + o.GitHubRepo,
		Branch:                o.GitBranch,
		GithubSecretIdKeyName: o.secretName,
	}

	return manifest.NewProvider(config)
}

func (o *initPipelineOpts) createPipelineManifest() (string, error) {
	// TODO change this to flag
	pipelineName := o.createPipelineName()
	provider, err := o.createPipelineProvider()
	if err != nil {
		return "", fmt.Errorf("could not create pipeline: %w", err)
	}

	manifest, err := manifest.CreatePipeline(pipelineName, provider, o.Environments)
	if err != nil {
		return "", fmt.Errorf("generate a manifest: %w", err)
	}

	manifestPath, err := o.workspace.WritePipelineManifest(manifest)
	if err != nil {
		return "", err
	}

	return manifestPath, nil
}

func (o *initPipelineOpts) createBuildspec() (string, error) {
	content, err := o.box.FindString(buildspecTemplatePath)
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

	// TODO remove binaryBuffer after https://github.com/aws/amazon-ecs-cli-v2/issues/661
	path, err := o.workspace.WritePipelineBuildspec(binaryBuffer{Buffer: &buf})
	if err != nil {
		return "", fmt.Errorf("write buildspec to workspace: %w", err)
	}
	return path, nil
}

func (o *initPipelineOpts) selectEnvironments() error {
	for {
		promptMsg := pipelineAddEnvPrompt
		promptHelpMsg := pipelineAddEnvHelpPrompt
		if len(o.Environments) > 0 {
			promptMsg = pipelineAddMoreEnvPrompt
			promptHelpMsg = pipelineAddMoreEnvHelpPrompt
		}
		addEnv, err := o.prompt.Confirm(promptMsg, promptHelpMsg)
		if err != nil {
			return fmt.Errorf("confirm adding an environment: %w", err)
		}
		if !addEnv {
			break
		}
		if err := o.selectEnvironment(); err != nil {
			return fmt.Errorf("add environment: %w", err)
		}
		if len(o.listAvailableEnvironments()) == 0 {
			break
		}
	}

	return nil
}

func (o *initPipelineOpts) listAvailableEnvironments() []string {
	var envs []string
	for _, env := range o.projectEnvs {
		// Check if environment has already been added to pipeline
		if o.envCanBeAdded(env) {
			envs = append(envs, env)
		}
	}

	return envs
}

func (o *initPipelineOpts) envCanBeAdded(selectedEnv string) bool {
	for _, env := range o.Environments {
		if selectedEnv == env {
			return false
		}
	}

	return true
}

func (o *initPipelineOpts) selectEnvironment() error {
	envs := o.listAvailableEnvironments()

	if len(envs) == 0 && len(o.Environments) != 0 {
		log.Infoln("There are no more environments to add.")
		return nil
	}

	env, err := o.prompt.SelectOne(
		pipelineSelectEnvPrompt,
		"Environment to be added as the next stage in your pipeline.",
		envs,
	)

	if err != nil {
		return err
	}

	o.Environments = append(o.Environments, env)

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

func (o *initPipelineOpts) selectGitHubURL() error {
	url, err := o.prompt.SelectOne(
		pipelineSelectGitHubURLPrompt,
		pipelineSelectGitHubURLHelpPrompt,
		o.repoURLs,
	)
	if err != nil {
		return fmt.Errorf("select GitHub URL: %w", err)
	}
	o.GitHubURL = url

	return nil
}

func (o *initPipelineOpts) parseOwnerRepoName(url string) (string, string, error) {
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
func (o *initPipelineOpts) parseGitRemoteResult(s string) ([]string, error) {
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

func (o *initPipelineOpts) getGitHubAccessToken() error {
	token, err := o.prompt.GetSecret(
		fmt.Sprintf("Please enter your GitHub Personal Access Token for your repository: %s", o.GitHubRepo),
		fmt.Sprintf(`The personal access token for the GitHub repository linked to your workspace. For more information on how to create a personal access token, please refer to: https://help.github.com/en/enterprise/2.17/user/authenticating-to-github/creating-a-personal-access-token-for-the-command-line.`),
	)

	if err != nil {
		return fmt.Errorf("get GitHub access token: %w", err)
	}
	// TODO use existing secret (pass in name or ARN?)

	o.GitHubAccessToken = token

	return nil
}

func (o *initPipelineOpts) getEnvNames() ([]string, error) {
	store, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to environment datastore: %w", err)
	}

	envs, err := store.ListEnvironments(o.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("could not list environments for project %s: %w", o.ProjectName(), err)
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
	vars := initPipelineVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a pipeline for applications in your workspace.",
		Long:  `Creates a pipeline for the applications in your workspace, using the environments associated with the applications.`,
		Example: `
  Create a pipeline for the applications in your workspace:
	/code $ ecs-preview pipeline init \
	  /code  --github-url https://github.com/gitHubUserName/myFrontendApp.git \
	  /code  --github-access-token file://myGitHubToken \
	  /code  --environments "stage,prod" \
	  /code  --deploy`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newInitPipelineOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Execute(); err != nil {
				return err
			}
			log.Infoln()
			log.Infoln("Recommended follow-up actions:")
			for _, followup := range opts.RecommendedActions() {
				log.Infof("- %s\n", followup)
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.GitHubURL, githubURLFlag, githubURLFlagShort, "", githubURLFlagDescription)
	cmd.Flags().StringVarP(&vars.GitHubAccessToken, githubAccessTokenFlag, githubAccessTokenFlagShort, "", githubAccessTokenFlagDescription)
	cmd.Flags().StringVarP(&vars.GitBranch, gitBranchFlag, gitBranchFlagShort, "", gitBranchFlagDescription)
	cmd.Flags().StringSliceVarP(&vars.Environments, envsFlag, envsFlagShort, []string{}, pipelineEnvsFlagDescription)

	return cmd
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/secretsmanager"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/command"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	pipelineInitAddEnvPrompt     = "Would you like to add an environment to your pipeline?"
	pipelineInitAddEnvHelpPrompt = "Adds an environment that corresponds to a deployment stage in your pipeline. Environments are added sequentially."

	pipelineInitAddMoreEnvPrompt     = "Would you like to add another environment to your pipeline?"
	pipelineInitAddMoreEnvHelpPrompt = "Adds another environment that corresponds to a deployment stage in your pipeline. Environments are added sequentially."

	pipelineSelectEnvPrompt = "Which environment would you like to add to your pipeline?"

	pipelineSelectGitHubURLPrompt     = "Which GitHub repository would you like to use for your service?"
	pipelineSelectGitHubURLHelpPrompt = `The GitHub repository linked to your workspace.
Pushing to this repository will trigger your pipeline build stage.
Please enter full repository URL, e.g. "https://github.com/myCompany/myRepo", or the owner/rep, e.g. "myCompany/myRepo"`
)

const (
	buildspecTemplatePath = "cicd/buildspec.yml"
	githubURL             = "github.com"
	//ccSubstring           = "codecommit"
	defaultBranch = "main"
)

const (
	fmtSecretName       = "github-token-%s-%s"
	fmtPipelineName     = "pipeline-%s-%s-%s"
	fmtPipelineProvider = "https://%s/%s/%s"
)

var (
	// Filled in via the -ldflags flag at compile time to support pipeline buildspec CLI pulling.
	binaryS3BucketPath string
)

var errNoEnvsInApp = errors.New("there were no more environments found that can be added to your pipeline. Please run `copilot env init` to create a new environment")

type initPipelineVars struct {
	appName           string
	environments      []string
	repoURL           string
	githubOwner       string
	githubRepo        string
	githubAccessToken string
	gitBranch         string
}

type initPipelineOpts struct {
	initPipelineVars
	// Interfaces to interact with dependencies.
	workspace      wsPipelineWriter
	secretsmanager secretsManager
	parser         template.Parser
	runner         runner
	cfnClient      appResourcesGetter
	store          store
	prompt         prompter

	// Outputs stored on successful actions.
	secretName string

	// Caches variables
	envs     []*config.Environment
	repoURLs []string
	fs       *afero.Afero
	buffer   bytes.Buffer
}

type artifactBucket struct {
	BucketName   string
	Region       string
	Environments []string
}

func newInitPipelineOpts(vars initPipelineVars) (*initPipelineOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace client: %w", err)
	}

	secretsmanager, err := secretsmanager.New()
	if err != nil {
		return nil, fmt.Errorf("new secretsmanager client: %w", err)
	}

	p := sessions.NewProvider()
	defaultSession, err := p.Default()
	if err != nil {
		return nil, err
	}

	ssmStore, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store client: %w", err)
	}

	return &initPipelineOpts{
		initPipelineVars: vars,
		workspace:        ws,
		secretsmanager:   secretsmanager,
		parser:           template.New(),
		cfnClient:        cloudformation.New(defaultSession),
		store:            ssmStore,
		prompt:           prompt.New(),
		runner:           command.New(),
		fs:               &afero.Afero{Fs: afero.NewOsFs()},
	}, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *initPipelineOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	if o.appName != "" {
		if _, err := o.store.GetApplication(o.appName); err != nil {
			return err
		}
	}

	if o.repoURL != "" {
		if err := validateDomainName(o.repoURL); err != nil {
			return err
		}
		// TODO: add "&& !strings.Contains(o.repoURL, ccURL)" to 'if' and "and CodeCommit" to error
		if !strings.Contains(o.repoURL, githubURL) {
			return errors.New("Copilot currently accepts only URLs to GitHub repository sources")
		}
	}

	if o.environments != nil {
		for _, env := range o.environments {
			_, err := o.store.GetEnvironment(o.appName, env)
			if err != nil {
				return err
			}
		}
		return nil
	}
	return nil
}

// Ask prompts for fields that are required but not passed in.
func (o *initPipelineOpts) Ask() error {
	if err := o.askEnvs(); err != nil {
		return err
	}
	// TODO: Do a switch/case here after adding more repo providers.
	if err := o.askRepoDetails(); err != nil {
		return err
	}
	return nil
}

// Execute writes the pipeline manifest file.
func (o *initPipelineOpts) Execute() error {
	secretName := o.createSecretName()
	_, err := o.secretsmanager.CreateSecret(secretName, o.githubAccessToken)

	if err != nil {
		var existsErr *secretsmanager.ErrSecretAlreadyExists
		if !errors.As(err, &existsErr) {
			return err
		}
		log.Successf("Secret already exists for %s! Do nothing.\n", color.HighlightUserInput(o.githubRepo))
	} else {
		log.Successf("Created the secret %s for pipeline source stage!\n", color.HighlightUserInput(secretName))
	}
	o.secretName = secretName

	// write pipeline.yml file, populate with:
	//   - github repo as source
	//   - stage names (environments)
	//   - enable/disable transition to prod envs

	err = o.createPipelineManifest()
	if err != nil {
		return err
	}

	err = o.createBuildspec()
	if err != nil {
		return err
	}

	return nil
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *initPipelineOpts) RecommendedActions() []string {
	return []string{
		"Commit and push the generated buildspec and manifest file.",
		fmt.Sprintf("Update the %s phase of your buildspec to unit test your services before pushing the images.", color.HighlightResource("build")),
		"Update your pipeline manifest to add additional stages.",
		fmt.Sprintf("Run %s to deploy your pipeline for the repository.", color.HighlightCode("copilot pipeline update")),
	}
}

func (o *initPipelineOpts) askEnvs() error {
	if len(o.environments) != 0 {
		return nil
	}
	err := o.getEnvs()
	if err != nil {
		return err
	}
	if err = o.selectEnvironments(); err != nil {
		return err
	}
	return nil
}

func (o *initPipelineOpts) getEnvs() error {
	envs, err := o.store.ListEnvironments(o.appName)
	if err != nil {
		return fmt.Errorf("list environments for application %s: %w", o.appName, err)
	}
	if len(envs) == 0 {
		return errNoEnvsInApp
	}
	o.envs = envs
	return nil
}

func (o *initPipelineOpts) selectEnvironments() error {
	for {
		promptMsg := pipelineInitAddEnvPrompt
		promptHelpMsg := pipelineInitAddEnvHelpPrompt
		if len(o.environments) > 0 {
			promptMsg = pipelineInitAddMoreEnvPrompt
			promptHelpMsg = pipelineInitAddMoreEnvHelpPrompt
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
	for _, env := range o.envs {
		// Check if environment has already been added to pipeline
		if o.envCanBeAdded(env.Name) {
			envs = append(envs, env.Name)
		}
	}

	return envs
}

func (o *initPipelineOpts) envCanBeAdded(selectedEnv string) bool {
	for _, env := range o.environments {
		if selectedEnv == env {
			return false
		}
	}

	return true
}

func (o *initPipelineOpts) selectEnvironment() error {
	envs := o.listAvailableEnvironments()

	if len(envs) == 0 && len(o.environments) != 0 {
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

	o.environments = append(o.environments, env)

	return nil
}

func (o *initPipelineOpts) askRepoDetails() error {
	var err error
	if o.repoURL == "" {
		if err = o.fetchRepoURLs(); err != nil {
			return err
		}
		if err = o.selectGitHubURL(); err != nil {
			return err
		}
	}
	if o.githubOwner, o.githubRepo, err = o.parseOwnerRepoName(o.repoURL); err != nil {
		return err
	}
	if o.githubAccessToken == "" {
		if err = o.getGitHubAccessToken(); err != nil {
			return err
		}
	}
	if o.gitBranch == "" {
		o.gitBranch = defaultBranch
	}
	return nil
}

func (o *initPipelineOpts) fetchRepoURLs() error {
	err := o.runner.Run("git", []string{"remote", "-v"}, command.Stdout(&o.buffer))
	if err != nil {
		return fmt.Errorf("get remote repository info: %w, run `git remote add` first please", err)
	}
	urls, err := o.parseGitRemoteResult(strings.TrimSpace(o.buffer.String()))
	if err != nil {
		return err
	}
	o.repoURLs = urls
	o.buffer.Reset()

	return nil
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
	o.repoURL = url

	return nil
}

func (o *initPipelineOpts) parseOwnerRepoName(url string) (string, string, error) {
	regexPattern := regexp.MustCompile(`.*(github.com)(:|\/)`)
	parsedURL := strings.TrimPrefix(url, regexPattern.FindString(url))
	parsedURL = strings.TrimSuffix(parsedURL, ".git")
	ownerRepo := strings.Split(parsedURL, "/")
	if len(ownerRepo) != 2 {
		return "", "", fmt.Errorf("unable to parse the GitHub repository owner and name from %s: please pass the repository URL with the format `--github-url https://github.com/{owner}/{repositoryName}`", url)
	}
	return ownerRepo[0], ownerRepo[1], nil
}

// examples:
// efekarakus	git@github.com:efekarakus/grit.git (fetch)
// efekarakus	https://github.com/karakuse/grit.git (fetch)
// origin	    https://github.com/koke/grit (fetch)
// koke         git://github.com/koke/grit.git (push)
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
		fmt.Sprintf("Please enter your GitHub Personal Access Token for your repository %s:", color.HighlightUserInput(o.githubRepo)),
		`The personal access token for the GitHub repository linked to your workspace. 
For more information, please refer to: https://git.io/JfDFD.`,
	)

	if err != nil {
		return fmt.Errorf("get GitHub access token: %w", err)
	}
	o.githubAccessToken = token
	return nil
}

func (o *initPipelineOpts) createPipelineManifest() error {
	pipelineName := o.createPipelineName()
	provider, err := o.createPipelineProvider()
	if err != nil {
		return fmt.Errorf("create pipeline provider: %w", err)
	}

	var stages []manifest.PipelineStage
	for _, environmentName := range o.environments {
		env, err := o.getEnvConfig(environmentName)
		if err != nil {
			return err
		}
		stage := manifest.PipelineStage{
			Name:             env.Name,
			RequiresApproval: env.Prod,
		}
		stages = append(stages, stage)
	}

	manifest, err := manifest.NewPipelineManifest(pipelineName, provider, stages)
	if err != nil {
		return fmt.Errorf("generate a pipeline manifest: %w", err)
	}

	var manifestExists bool
	manifestPath, err := o.workspace.WritePipelineManifest(manifest)
	if err != nil {
		e, ok := err.(*workspace.ErrFileExists)
		if !ok {
			return fmt.Errorf("write pipeline manifest to workspace: %w", err)
		}
		manifestExists = true
		manifestPath = e.FileName
	}

	manifestPath, err = relPath(manifestPath)
	if err != nil {
		return err
	}

	manifestMsgFmt := "Wrote the pipeline manifest for %s at '%s'\n"
	if manifestExists {
		manifestMsgFmt = "Pipeline manifest file for %s already exists at %s, skipping writing it.\n"
	}
	log.Successf(manifestMsgFmt, color.HighlightUserInput(o.githubRepo), color.HighlightResource(manifestPath))
	log.Infoln("The manifest contains configurations for your CodePipeline resources, such as your pipeline stages and build steps.")
	return nil
}

func (o *initPipelineOpts) createBuildspec() error {
	artifactBuckets, err := o.artifactBuckets()
	if err != nil {
		return err
	}
	content, err := o.parser.Parse(buildspecTemplatePath, struct {
		BinaryS3BucketPath string
		Version            string
		ArtifactBuckets    []artifactBucket
	}{
		BinaryS3BucketPath: binaryS3BucketPath,
		Version:            version.Version,
		ArtifactBuckets:    artifactBuckets,
	})
	if err != nil {
		return err
	}
	buildspecPath, err := o.workspace.WritePipelineBuildspec(content)
	var buildspecExists bool
	if err != nil {
		e, ok := err.(*workspace.ErrFileExists)
		if !ok {
			return fmt.Errorf("write buildspec to workspace: %w", err)
		}
		buildspecExists = true
		buildspecPath = e.FileName
	}
	buildspecMsgFmt := "Wrote the buildspec for the pipeline's build stage at '%s'\n"
	if buildspecExists {
		buildspecMsgFmt = "Buildspec file for pipeline already exists at %s, skipping writing it.\n"
	}
	buildspecPath, err = relPath(buildspecPath)
	if err != nil {
		return err
	}
	log.Successf(buildspecMsgFmt, color.HighlightResource(buildspecPath))
	log.Infoln("The buildspec contains the commands to build and push your container images to your ECR repositories.")

	return nil
}

func (o *initPipelineOpts) createSecretName() string {
	return fmt.Sprintf(fmtSecretName, o.appName, o.githubRepo)
}

func (o *initPipelineOpts) createPipelineName() string {
	return fmt.Sprintf(fmtPipelineName, o.appName, o.githubOwner, o.githubRepo)
}

func (o *initPipelineOpts) createPipelineProvider() (manifest.Provider, error) {
	config := &manifest.GitHubProperties{
		OwnerAndRepository:    fmt.Sprintf(fmtPipelineProvider, githubURL, o.githubOwner, o.githubRepo),
		Branch:                o.gitBranch,
		GithubSecretIdKeyName: o.secretName,
	}
	return manifest.NewProvider(config)
}

func (o *initPipelineOpts) getEnvConfig(environmentName string) (*config.Environment, error) {
	for _, env := range o.envs {
		if env.Name == environmentName {
			return env, nil
		}
	}
	return nil, fmt.Errorf("environment %s in application %s is not found", environmentName, o.appName)
}

func (o *initPipelineOpts) artifactBuckets() ([]artifactBucket, error) {
	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		return nil, fmt.Errorf("get application %s: %w", o.appName, err)
	}
	regionalResources, err := o.cfnClient.GetRegionalAppResources(app)
	if err != nil {
		return nil, fmt.Errorf("get regional application resources: %w", err)
	}

	var buckets []artifactBucket
	for _, resource := range regionalResources {
		var envNames []string
		for _, environment := range o.environments {
			env, err := o.getEnvConfig(environment)
			if err != nil {
				return nil, err
			}
			if env.Region == resource.Region {
				envNames = append(envNames, env.Name)
			}
		}
		bucket := artifactBucket{
			BucketName:   resource.S3Bucket,
			Region:       resource.Region,
			Environments: envNames,
		}
		buckets = append(buckets, bucket)
	}
	return buckets, nil
}

// buildPipelineInitCmd build the command for creating a new pipeline.
func buildPipelineInitCmd() *cobra.Command {
	vars := initPipelineVars{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a pipeline for the services in your workspace.",
		Long:  `Creates a pipeline for the services in your workspace, using the environments associated with the application.`,
		Example: `
  Create a pipeline for the services in your workspace.
  /code $ copilot pipeline init \
  /code  --github-url https://github.com/gitHubUserName/myFrontendApp.git \
  /code  --github-access-token file://myGitHubToken \
  /code  --environments "stage,prod"`,
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
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.repoURL, githubURLFlag, githubURLFlagShort, "", githubURLFlagDescription)
	cmd.Flags().StringVarP(&vars.githubAccessToken, githubAccessTokenFlag, githubAccessTokenFlagShort, "", githubAccessTokenFlagDescription)
	cmd.Flags().StringVarP(&vars.gitBranch, gitBranchFlag, gitBranchFlagShort, "", gitBranchFlagDescription)
	cmd.Flags().StringSliceVarP(&vars.environments, envsFlag, envsFlagShort, []string{}, pipelineEnvsFlagDescription)

	return cmd
}

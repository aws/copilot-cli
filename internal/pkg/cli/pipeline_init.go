// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/dustin/go-humanize/english"

	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/copilot-cli/internal/pkg/exec"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/spf13/cobra"

	"github.com/dustin/go-humanize"

	"github.com/aws/copilot-cli/internal/pkg/term/selector"

	"github.com/aws/copilot-cli/internal/pkg/aws/secretsmanager"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
)

const (
	fmtPipelineInitNamePrompt  = "What would you like to %s this pipeline?"
	pipelineInitNameHelpPrompt = `A unique identifier for your pipeline (e.g., "myRepo-myBranch").`

	pipelineSelectEnvPrompt     = "Which environment would you like to add to your pipeline?"
	pipelineSelectEnvHelpPrompt = "Adds an environment that corresponds to a deployment stage in your pipeline. Environments are added sequentially."

	pipelineSelectURLPrompt     = "Which repository would you like to use for your pipeline?"
	pipelineSelectURLHelpPrompt = `The repository linked to your pipeline.
Pushing to this repository will trigger your pipeline build stage.
Please enter full repository URL, e.g., "https://github.com/myCompany/myRepo", or the owner/rep, e.g., "myCompany/myRepo"`
)

const (
	workloadsPipelineBuildspecTemplatePath    = "cicd/buildspec.yml"
	environmentsPipelineBuildspecTemplatePath = "cicd/env/buildspec.yml"

	fmtPipelineStackName = "pipeline-%s-%s" // Ex: "pipeline-appName-repoName"
	defaultBranch        = deploy.DefaultPipelineBranch
	// For a GitHub repository.
	githubURL     = "github.com"
	fmtGHRepoURL  = "https://%s/%s/%s"   // Ex: "https://github.com/repoOwner/repoName"
	fmtSecretName = "github-token-%s-%s" // Ex: "github-token-appName-repoName"
	// For a CodeCommit repository.
	awsURL       = "aws.amazon.com"
	ccIdentifier = "codecommit"
	fmtCCRepoURL = "https://%s.console.%s/codesuite/codecommit/repositories/%s/browse" // Ex: "https://region.console.aws.amazon.com/codesuite/codecommit/repositories/repoName/browse"
	// For a Bitbucket repository.
	bbURL        = "bitbucket.org"
	fmtBBRepoURL = "https://%s/%s/%s" // Ex: "https://bitbucket.org/repoOwner/repoName"
)

const (
	pipelineTypeWorkloads    = "Workloads"
	pipelineTypeEnvironments = "Environments"
)

var pipelineTypes = []string{pipelineTypeWorkloads, pipelineTypeEnvironments}

var buildspecTemplateFunctions = map[string]interface{}{
	"URLSafeVersion": template.URLSafeVersion,
}

var (
	// Filled in via the -ldflags flag at compile time to support pipeline buildspec CLI pulling.
	binaryS3BucketPath string
)

// Pipeline init errors.
var (
	fmtErrInvalidPipelineProvider = "repository %s must be from a supported provider: %s"
)

type pipelineInitializer interface {
	writeManifest() error
	writeBuildspec() error
}

type workloadPipelineInitializer struct {
	cmd *initPipelineOpts
}

type envPipelineInitializer struct {
	cmd *initPipelineOpts
}

func (ini *workloadPipelineInitializer) writeManifest() error {
	var stages []manifest.PipelineStage
	for _, env := range ini.cmd.envConfigs {
		stage := manifest.PipelineStage{
			Name: env.Name,
		}
		stages = append(stages, stage)
	}
	return ini.cmd.createPipelineManifest(stages)
}

func (ini *workloadPipelineInitializer) writeBuildspec() error {
	if err := ini.cmd.createBuildspec(workloadsPipelineBuildspecTemplatePath); err != nil {
		return err
	}
	log.Debugln(`The buildspec contains the commands to push your container images, and generate CloudFormation templates.
Update the "build" phase to unit test your services before pushing the images.`)
	return nil
}

func (ini *envPipelineInitializer) writeManifest() error {
	var stages []manifest.PipelineStage
	for _, env := range ini.cmd.envConfigs {
		stage := manifest.PipelineStage{
			Name: env.Name,
			Deployments: manifest.Deployments{
				"deploy-env": &manifest.Deployment{
					TemplatePath:   path.Join(deploy.DefaultPipelineArtifactsDir, fmt.Sprintf(envCFNTemplateNameFmt, env.Name)),
					TemplateConfig: path.Join(deploy.DefaultPipelineArtifactsDir, fmt.Sprintf(envCFNTemplateConfigurationNameFmt, env.Name)),
					StackName:      stack.NameForEnv(ini.cmd.appName, env.Name),
				},
			},
		}
		stages = append(stages, stage)
	}
	return ini.cmd.createPipelineManifest(stages)
}

func (ini *envPipelineInitializer) writeBuildspec() error {
	if err := ini.cmd.createBuildspec(environmentsPipelineBuildspecTemplatePath); err != nil {
		return err
	}
	log.Debugln(`The buildspec contains the commands to generate CloudFormation templates for your environments.`)
	return nil
}

func newPipelineInitializer(cmd *initPipelineOpts) pipelineInitializer {
	switch cmd.pipelineType {
	case pipelineTypeWorkloads:
		return &workloadPipelineInitializer{
			cmd: cmd,
		}
	case pipelineTypeEnvironments:
		return &envPipelineInitializer{
			cmd: cmd,
		}
	}
	return nil
}

type initPipelineVars struct {
	appName           string
	name              string // Name of the pipeline
	environments      []string
	repoURL           string
	repoBranch        string
	githubAccessToken string
	pipelineType      string
}

type initPipelineOpts struct {
	initPipelineVars
	// Interfaces to interact with dependencies.
	workspace      wsPipelineIniter
	secretsmanager secretsManager
	parser         template.Parser
	runner         execRunner
	sessProvider   sessionProvider
	cfnClient      appResourcesGetter
	store          store
	prompt         prompter
	sel            pipelineEnvSelector
	pipelineLister deployedPipelineLister

	// Outputs stored on successful actions.
	secret    string
	provider  string
	repoName  string
	repoOwner string
	ccRegion  string

	// Cached variables
	wsAppName    string
	buffer       bytes.Buffer
	envConfigs   []*config.Environment
	manifestPath string // relative path to pipeline's manifest.yml file
}

type artifactBucket struct {
	BucketName   string
	Region       string
	Environments []string
}

func newInitPipelineOpts(vars initPipelineVars) (*initPipelineOpts, error) {
	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}

	p := sessions.ImmutableProvider(sessions.UserAgentExtras("pipeline init"))
	defaultSession, err := p.Default()
	if err != nil {
		return nil, err
	}

	ssmStore := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	prompter := prompt.New()

	wsAppName := tryReadingAppName()
	if vars.appName == "" {
		vars.appName = wsAppName
	}

	return &initPipelineOpts{
		initPipelineVars: vars,
		workspace:        ws,
		secretsmanager:   secretsmanager.New(defaultSession),
		parser:           template.New(),
		sessProvider:     p,
		cfnClient:        cloudformation.New(defaultSession, cloudformation.WithProgressTracker(os.Stderr)),
		store:            ssmStore,
		prompt:           prompter,
		sel:              selector.NewAppEnvSelector(prompter, ssmStore),
		runner:           exec.NewCmd(),
		wsAppName:        wsAppName,
		pipelineLister:   deploy.NewPipelineStore(rg.New(defaultSession)),
	}, nil
}

// Validate returns an error if the optional flag values passed by the user are invalid.
func (o *initPipelineOpts) Validate() error {
	return nil
}

// Ask prompts for required fields that are not passed in and validates them.
func (o *initPipelineOpts) Ask() error {
	// This command must be executed in the app's workspace because the pipeline manifest and buildspec will be created and stored.
	if err := validateWorkspaceApp(o.wsAppName, o.appName, o.store); err != nil {
		return err
	}
	o.appName = o.wsAppName

	if err := o.askOrValidateURL(); err != nil {
		return err
	}

	if err := o.parseRepoDetails(); err != nil {
		return err
	}

	if o.repoBranch == "" {
		o.getBranch()
	}

	if err := o.askOrValidatePipelineName(); err != nil {
		return err
	}

	if err := o.askOrValidatePipelineType(); err != nil {
		return err
	}

	if err := o.validateDuplicatePipeline(); err != nil {
		return err
	}

	if len(o.environments) == 0 {
		if err := o.askEnvs(); err != nil {
			return err
		}
	}
	if err := o.validateEnvs(); err != nil {
		return err
	}

	return nil
}

// Execute writes the pipeline manifest file.
func (o *initPipelineOpts) Execute() error {
	if o.provider == manifest.GithubV1ProviderName {
		if err := o.storeGitHubAccessToken(); err != nil {
			return err
		}
	}
	log.Infoln()
	ini := newPipelineInitializer(o)
	if err := ini.writeManifest(); err != nil {
		return err
	}
	if err := ini.writeBuildspec(); err != nil {
		return err
	}
	return nil
}

// RequiredActions returns follow-up actions the user must take after successfully executing the command.
func (o *initPipelineOpts) RequiredActions() []string {
	return []string{
		fmt.Sprintf("Commit and push the %s directory to your repository.", color.HighlightResource("copilot/")),
		fmt.Sprintf("Run %s to create your pipeline.", color.HighlightCode("copilot pipeline deploy")),
	}
}

// validateDuplicatePipeline checks that the pipeline name isn't already used
// by another pipeline to reduce potential confusion with a legacy pipeline.
func (o *initPipelineOpts) validateDuplicatePipeline() error {
	var allPipelines []string

	localPipelines, err := o.workspace.ListPipelines()
	if err != nil {
		return fmt.Errorf("get local pipelines: %w", err)
	}
	for _, pipeline := range localPipelines {
		allPipelines = append(allPipelines, pipeline.Name)
	}

	deployedPipelines, err := o.pipelineLister.ListDeployedPipelines(o.appName)
	if err != nil {
		return fmt.Errorf("list deployed pipelines for app %s: %w", o.appName, err)
	}
	for _, pipeline := range deployedPipelines {
		allPipelines = append(allPipelines, pipeline.Name)
	}

	fullName := fmt.Sprintf(fmtPipelineStackName, o.appName, o.name)
	for _, pipeline := range allPipelines {
		if strings.EqualFold(pipeline, o.name) || strings.EqualFold(pipeline, fullName) {
			log.Warningf(`You already have a pipeline named '%s'.
To deploy the existing pipeline, run %s.
To recreate the pipeline, run %s,
	optionally delete your pipeline.yml/manifest.yml and/or buildspec.yml file(s),
	then run %s.
If you have manually deleted your pipeline.yml/manifest.yml and/or buildspec.yml file(s) 
	for the existing pipeline, Copilot will now generate new default file(s).
To create an additional pipeline, run "copilot pipeline init" again, but with a new pipeline name.
`, o.name, fmt.Sprintf(`"copilot pipeline deploy --name %s"`, o.name), fmt.Sprintf(`"copilot pipeline delete --name %s"`, o.name), fmt.Sprintf(`"copilot pipeline init --name %s"`, o.name))
			return nil
		}
	}
	return nil
}

func (o *initPipelineOpts) askOrValidatePipelineName() error {
	if o.name == "" {
		return o.askPipelineName()
	}

	return validatePipelineName(o.name, o.appName)
}

func (o *initPipelineOpts) askOrValidateURL() error {
	if o.repoURL == "" {
		return o.selectURL()
	}

	return o.validateURL(o.repoURL)
}

func (o *initPipelineOpts) askPipelineName() error {
	promptOpts := []prompt.PromptConfig{
		prompt.WithFinalMessage("Pipeline name:"),
	}

	// Only show suggestion if [repo]-[branch] is a valid pipeline name.
	suggestion := strings.ToLower(fmt.Sprintf("%s-%s", o.repoName, o.repoBranch))
	if err := validatePipelineName(suggestion, o.appName); err == nil {
		promptOpts = append(promptOpts, prompt.WithDefaultInput(suggestion))
	}

	name, err := o.prompt.Get(fmt.Sprintf(fmtPipelineInitNamePrompt, color.Emphasize("name")),
		pipelineInitNameHelpPrompt,
		func(val interface{}) error {
			return validatePipelineName(val, o.appName)
		}, promptOpts...)
	if err != nil {
		return fmt.Errorf("get pipeline name: %w", err)
	}

	o.name = name
	return nil
}

func (o *initPipelineOpts) askOrValidatePipelineType() error {
	if o.pipelineType != "" {
		for _, typ := range pipelineTypes {
			if o.pipelineType == typ {
				return nil
			}
		}
		return fmt.Errorf("invalid pipeline type %q; must be one of %s", o.pipelineType, english.WordSeries(applyAll(pipelineTypes, strconv.Quote), "or"))
	}

	typ, err := o.prompt.SelectOption("What type of continuous delivery pipeline is this?",
		"A pipeline can be set up to deploy either your workloads or your environments",
		[]prompt.Option{
			{
				Value: pipelineTypeWorkloads,
				Hint:  "Deploy the services or jobs in your workspace",
			},
			{
				Value: pipelineTypeEnvironments,
				Hint:  "Deploy the environments in your workspace",
			},
		})
	if err != nil {
		return fmt.Errorf("prompt for pipeline type: %w", err)
	}
	o.pipelineType = typ
	return nil
}

func (o *initPipelineOpts) validateURL(url string) error {
	// Note: no longer calling `validateDomainName` because if users use git-remote-codecommit
	// (the HTTPS (GRC) protocol) to connect to CodeCommit, the url does not have any periods.
	if !strings.Contains(url, githubURL) && !strings.Contains(url, ccIdentifier) && !strings.Contains(url, bbURL) {
		return fmt.Errorf(fmtErrInvalidPipelineProvider, url, english.WordSeries(manifest.PipelineProviders, "or"))
	}
	return nil
}

// To avoid duplicating calls to GetEnvironment, validate and get config in the same step.
func (o *initPipelineOpts) validateEnvs() error {
	var envConfigs []*config.Environment
	for _, env := range o.environments {
		config, err := o.store.GetEnvironment(o.appName, env)
		if err != nil {
			return fmt.Errorf("validate environment %s: %w", env, err)
		}
		envConfigs = append(envConfigs, config)
	}
	o.envConfigs = envConfigs
	return nil
}

func (o *initPipelineOpts) askEnvs() error {
	envs, err := o.sel.Environments(pipelineSelectEnvPrompt, pipelineSelectEnvHelpPrompt, o.appName, func(order int) prompt.PromptConfig {
		return prompt.WithFinalMessage(fmt.Sprintf("%s stage:", humanize.Ordinal(order)))
	})
	if err != nil {
		return fmt.Errorf("select environments: %w", err)
	}

	o.environments = envs
	return nil
}

func (o *initPipelineOpts) parseRepoDetails() error {
	switch {
	case strings.Contains(o.repoURL, githubURL):
		return o.parseGitHubRepoDetails()
	case strings.Contains(o.repoURL, ccIdentifier):
		return o.parseCodeCommitRepoDetails()
	case strings.Contains(o.repoURL, bbURL):
		return o.parseBitbucketRepoDetails()
	default:
		return fmt.Errorf(fmtErrInvalidPipelineProvider, o.repoURL, english.WordSeries(manifest.PipelineProviders, "or"))
	}
}

// getBranch fetches the user's current branch as a best-guess of which branch they want their pipeline to follow. If err, insert default branch name.
func (o *initPipelineOpts) getBranch() {
	// Fetches local git branch.
	err := o.runner.Run("git", []string{"rev-parse", "--abbrev-ref", "HEAD"}, exec.Stdout(&o.buffer))
	o.repoBranch = strings.TrimSpace(o.buffer.String())
	if err != nil {
		o.repoBranch = defaultBranch
	}
	if strings.TrimSpace(o.buffer.String()) == "" {
		o.repoBranch = defaultBranch
	}
	o.buffer.Reset()
	log.Infof(`Your pipeline will follow branch '%s'.
`, color.HighlightUserInput(o.repoBranch))
}

func (o *initPipelineOpts) parseGitHubRepoDetails() error {
	// If the user uses a flag to specify a GitHub access token,
	// GitHub version 1 (not CSC) is the provider.
	o.provider = manifest.GithubProviderName
	if o.githubAccessToken != "" {
		o.provider = manifest.GithubV1ProviderName
	}

	repoDetails, err := ghRepoURL(o.repoURL).parse()
	if err != nil {
		return err
	}
	o.repoName = repoDetails.name
	o.repoOwner = repoDetails.owner

	return nil
}

func (o *initPipelineOpts) parseCodeCommitRepoDetails() error {
	o.provider = manifest.CodeCommitProviderName
	repoDetails, err := ccRepoURL(o.repoURL).parse()
	if err != nil {
		return err
	}
	o.repoName = repoDetails.name
	o.ccRegion = repoDetails.region

	// If the CodeCommit region is different than that of the app, pipeline init errors out.
	sess, err := o.sessProvider.Default()
	if err != nil {
		return fmt.Errorf("retrieve default session: %w", err)
	}
	region := aws.StringValue(sess.Config.Region)
	if o.ccRegion == "" {
		o.ccRegion = region
	}
	if o.ccRegion != region {
		return fmt.Errorf("repository %s is in %s, but app %s is in %s; they must be in the same region", o.repoName, o.ccRegion, o.appName, region)
	}

	return nil
}

func (o *initPipelineOpts) parseBitbucketRepoDetails() error {
	o.provider = manifest.BitbucketProviderName
	repoDetails, err := bbRepoURL(o.repoURL).parse()
	if err != nil {
		return err
	}
	o.repoName = repoDetails.name
	o.repoOwner = repoDetails.owner

	return nil
}

func (o *initPipelineOpts) selectURL() error {
	// Fetches and parses all remote repositories.
	err := o.runner.Run("git", []string{"remote", "-v"}, exec.Stdout(&o.buffer))
	if err != nil {
		return fmt.Errorf("get remote repository info: %w; make sure you have installed Git and are in a Git repository", err)
	}
	urls, err := o.parseGitRemoteResult(strings.TrimSpace(o.buffer.String()))
	if err != nil {
		return err
	}
	o.buffer.Reset()

	// If there is only one returned URL, set it rather than prompt to select.
	if len(urls) == 1 {
		log.Infof(`Only one git remote detected. Your pipeline will follow '%s'.
`, color.HighlightUserInput(urls[0]))
		o.repoURL = urls[0]
		return nil
	}

	// Prompts user to select a repo URL.
	url, err := o.prompt.SelectOne(
		pipelineSelectURLPrompt,
		pipelineSelectURLHelpPrompt,
		urls,
		prompt.WithFinalMessage("Repository URL:"),
	)
	if err != nil {
		return fmt.Errorf("select URL: %w", err)
	}
	o.repoURL = url

	return nil
}

// examples:
// efekarakus	git@github.com:efekarakus/grit.git (fetch)
// efekarakus	https://github.com/karakuse/grit.git (fetch)
// origin	    https://github.com/koke/grit (fetch)
// koke         git://github.com/koke/grit.git (push)

// https	https://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample (fetch)
// fed		codecommit::us-west-2://aws-sample (fetch)
// ssh		ssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample (push)
// bbhttps	https://huanjani@bitbucket.org/huanjani/aws-copilot-sample-service.git (fetch)
// bbssh	ssh://git@bitbucket.org:teamsinspace/documentation-tests.git (fetch)

// parseGitRemoteResults returns just the trimmed middle column (url) of the `git remote -v` results,
// and skips urls from unsupported sources.
func (o *initPipelineOpts) parseGitRemoteResult(s string) ([]string, error) {
	var urls []string
	urlSet := make(map[string]bool)
	items := strings.Split(s, "\n")
	for _, item := range items {
		if !strings.Contains(item, githubURL) && !strings.Contains(item, ccIdentifier) && !strings.Contains(item, bbURL) {
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

type ghRepoURL string
type ghRepoDetails struct {
	name  string
	owner string
}
type ccRepoURL string
type ccRepoDetails struct {
	name   string
	region string
}

type bbRepoURL string
type bbRepoDetails struct {
	name  string
	owner string
}

func (url ghRepoURL) parse() (ghRepoDetails, error) {
	urlString := string(url)
	regexPattern := regexp.MustCompile(`.*(github.com)(:|\/)`)
	parsedURL := strings.TrimPrefix(urlString, regexPattern.FindString(urlString))
	parsedURL = strings.TrimSuffix(parsedURL, ".git")
	ownerRepo := strings.Split(parsedURL, "/")
	if len(ownerRepo) != 2 {
		return ghRepoDetails{}, fmt.Errorf("unable to parse the GitHub repository owner and name from %s: please pass the repository URL with the format `--url https://github.com/{owner}/{repositoryName}`", url)
	}
	return ghRepoDetails{
		name:  ownerRepo[1],
		owner: ownerRepo[0],
	}, nil
}

func (url ccRepoURL) parse() (ccRepoDetails, error) {
	urlString := string(url)
	var region string
	// Parse region.
	switch {
	case strings.HasPrefix(urlString, "https://") || strings.HasPrefix(urlString, "ssh://"):
		parsedURL := strings.Split(urlString, ".")
		region = parsedURL[1]
	case strings.HasPrefix(urlString, "codecommit::"):
		parsedURL := strings.Split(urlString, ":")
		region = parsedURL[2]
	case strings.HasPrefix(urlString, "codecommit://"):
		// Use default profile region.
	default:
		return ccRepoDetails{}, fmt.Errorf("unknown CodeCommit URL format: %s", url)
	}
	if region != "" {
		// Double-check that parsed results is a valid region. Source: https://www.regextester.com/109163
		match, _ := regexp.MatchString(`(us(-gov)?|ap|ca|cn|eu|sa)-(central|(north|south)?(east|west)?)-\d`, region)
		if !match {
			return ccRepoDetails{}, fmt.Errorf("unable to parse the AWS region from %s", url)
		}
	}

	// Parse repo name.
	parsedForRepo := strings.Split(urlString, "/")
	if len(parsedForRepo) < 2 {
		return ccRepoDetails{}, fmt.Errorf("unable to parse the CodeCommit repository name from %s", url)
	}
	repoName := parsedForRepo[len(parsedForRepo)-1]

	return ccRepoDetails{
		name:   repoName,
		region: region,
	}, nil
}

// Bitbucket URLs, post-parseGitRemoteResults(), may look like:
// https://username@bitbucket.org/teamsinspace/documentation-tests
// ssh://git@bitbucket.org:teamsinspace/documentation-tests
func (url bbRepoURL) parse() (bbRepoDetails, error) {
	urlString := string(url)
	splitURL := strings.Split(urlString, "/")
	if len(splitURL) < 2 {
		return bbRepoDetails{}, fmt.Errorf("unable to parse the Bitbucket repository name from %s", url)
	}
	repoName := splitURL[len(splitURL)-1]
	// rather than check for the SSH prefix, split on colon here; HTTPS version will be unaffected.
	splitRepoOwner := strings.Split(splitURL[len(splitURL)-2], ":")
	repoOwner := splitRepoOwner[len(splitRepoOwner)-1]

	return bbRepoDetails{
		name:  repoName,
		owner: repoOwner,
	}, nil
}

func (o *initPipelineOpts) storeGitHubAccessToken() error {
	secretName := o.secretName()
	_, err := o.secretsmanager.CreateSecret(secretName, o.githubAccessToken)

	if err != nil {
		var existsErr *secretsmanager.ErrSecretAlreadyExists
		if !errors.As(err, &existsErr) {
			return err
		}
		log.Successf("Secret already exists for %s! Do nothing.\n", color.HighlightUserInput(o.repoName))
	} else {
		log.Successf("Created the secret %s for pipeline source stage!\n", color.HighlightUserInput(secretName))
	}
	o.secret = secretName
	return nil
}

func (o *initPipelineOpts) createPipelineManifest(stages []manifest.PipelineStage) error {
	provider, err := o.pipelineProvider()
	if err != nil {
		return err
	}
	manifest, err := manifest.NewPipeline(o.name, provider, stages)
	if err != nil {
		return fmt.Errorf("generate a pipeline manifest: %w", err)
	}

	var manifestExists bool
	o.manifestPath, err = o.workspace.WritePipelineManifest(manifest, o.name)
	if err != nil {
		e, ok := err.(*workspace.ErrFileExists)
		if !ok {
			return fmt.Errorf("write pipeline manifest to workspace: %w", err)
		}
		manifestExists = true
		o.manifestPath = e.FileName
	}

	mftPath := displayPath(o.manifestPath)

	o.manifestPath, err = o.workspace.Rel(o.manifestPath)
	if err != nil {
		return err
	}

	if manifestExists {
		log.Infof(`Pipeline manifest file for %s already exists at %s, skipping writing it.
Previously set repository URL, branch, and environment stages will remain.
`, color.HighlightUserInput(o.repoName), color.HighlightResource(mftPath))
	} else {
		log.Successf("Wrote the pipeline manifest for %s at '%s'\n", color.HighlightUserInput(o.repoName), color.HighlightResource(mftPath))
	}
	log.Debug(`The manifest contains configurations for your pipeline.
Update the file to add stages, change the tracked branch, add test commands or manual approval actions.
`)
	return nil
}

func (o *initPipelineOpts) createBuildspec(buildSpecTemplatePath string) error {
	artifactBuckets, err := o.artifactBuckets()
	if err != nil {
		return err
	}
	content, err := o.parser.Parse(buildSpecTemplatePath, struct {
		BinaryS3BucketPath string
		Version            string
		ManifestPath       string
		ArtifactBuckets    []artifactBucket
	}{
		BinaryS3BucketPath: binaryS3BucketPath,
		Version:            version.Version,
		ManifestPath:       filepath.ToSlash(o.manifestPath), // The manifest path must be rendered in the buildspec with '/' instead of os-specific separator.
		ArtifactBuckets:    artifactBuckets,
	}, template.WithFuncs(buildspecTemplateFunctions))
	if err != nil {
		return err
	}
	buildspecPath, err := o.workspace.WritePipelineBuildspec(content, o.name)
	var buildspecExists bool
	if err != nil {
		e, ok := err.(*workspace.ErrFileExists)
		if !ok {
			return fmt.Errorf("write buildspec to workspace: %w", err)
		}
		buildspecExists = true
		buildspecPath = e.FileName
	}
	buildspecPath = displayPath(buildspecPath)
	if buildspecExists {
		log.Infof(`Buildspec file for pipeline already exists at %s, skipping writing it.
Previously set config will remain.
`, color.HighlightResource(buildspecPath))
		return nil
	}
	log.Successf("Wrote the buildspec for the pipeline's build stage at '%s'\n", color.HighlightResource(buildspecPath))
	return nil
}

func (o *initPipelineOpts) secretName() string {
	return fmt.Sprintf(fmtSecretName, o.appName, o.repoName)
}

func (o *initPipelineOpts) pipelineProvider() (manifest.Provider, error) {
	var config interface{}
	switch o.provider {
	case manifest.GithubV1ProviderName:
		config = &manifest.GitHubV1Properties{
			RepositoryURL:         fmt.Sprintf(fmtGHRepoURL, githubURL, o.repoOwner, o.repoName),
			Branch:                o.repoBranch,
			GithubSecretIdKeyName: o.secret,
		}
	case manifest.GithubProviderName:
		config = &manifest.GitHubProperties{
			RepositoryURL: fmt.Sprintf(fmtGHRepoURL, githubURL, o.repoOwner, o.repoName),
			Branch:        o.repoBranch,
		}
	case manifest.CodeCommitProviderName:
		config = &manifest.CodeCommitProperties{
			RepositoryURL: fmt.Sprintf(fmtCCRepoURL, o.ccRegion, awsURL, o.repoName),
			Branch:        o.repoBranch,
		}
	case manifest.BitbucketProviderName:
		config = &manifest.BitbucketProperties{
			RepositoryURL: fmt.Sprintf(fmtBBRepoURL, bbURL, o.repoOwner, o.repoName),
			Branch:        o.repoBranch,
		}
	default:
		return nil, fmt.Errorf("unable to create pipeline source provider for %s", o.repoName)
	}
	return manifest.NewProvider(config)
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
		for _, env := range o.envConfigs {
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
  /code  --name frontend-main \
  /code  --url https://github.com/gitHubUserName/frontend.git \
  /code  --git-branch main \
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
			log.Infoln("Required follow-up actions:")
			for _, followup := range opts.RequiredActions() {
				log.Infof("- %s\n", followup)
			}
			return nil
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, "", appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", pipelineFlagDescription)
	cmd.Flags().StringVar(&vars.repoURL, githubURLFlag, "", githubURLFlagDescription)
	_ = cmd.Flags().MarkHidden(githubURLFlag)
	cmd.Flags().StringVarP(&vars.repoURL, repoURLFlag, repoURLFlagShort, "", repoURLFlagDescription)
	cmd.Flags().StringVarP(&vars.githubAccessToken, githubAccessTokenFlag, githubAccessTokenFlagShort, "", githubAccessTokenFlagDescription)
	_ = cmd.Flags().MarkHidden(githubAccessTokenFlag)
	cmd.Flags().StringVarP(&vars.repoBranch, gitBranchFlag, gitBranchFlagShort, "", gitBranchFlagDescription)
	cmd.Flags().StringSliceVarP(&vars.environments, envsFlag, envsFlagShort, []string{}, pipelineEnvsFlagDescription)
	cmd.Flags().StringVarP(&vars.pipelineType, pipelineTypeFlag, pipelineTypeShort, "", pipelineTypeFlagDescription)
	return cmd
}

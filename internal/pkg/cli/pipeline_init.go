// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

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
	pipelineSelectEnvPrompt     = "Which environment would you like to add to your pipeline?"
	pipelineSelectEnvHelpPrompt = "Adds an environment that corresponds to a deployment stage in your pipeline. Environments are added sequentially."

	pipelineSelectURLPrompt     = "Which repository would you like to use for your pipeline?"
	pipelineSelectURLHelpPrompt = `The repository linked to your pipeline.
Pushing to this repository will trigger your pipeline build stage.
Please enter full repository URL, e.g. "https://github.com/myCompany/myRepo", or the owner/rep, e.g. "myCompany/myRepo"`
	pipelineSelectBranchPrompt     = "Which branch would you like to use for your pipeline?"
	pipelineSelectBranchHelpPrompt = "The specific branch of your chosen repository that your pipeline will follow."
)

const (
	buildspecTemplatePath = "cicd/buildspec.yml"
	fmtPipelineName       = "pipeline-%s-%s" // Ex: "pipeline-appName-repoName"
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

var (
	// Filled in via the -ldflags flag at compile time to support pipeline buildspec CLI pulling.
	binaryS3BucketPath string
)

type initPipelineVars struct {
	appName           string
	environments      []string
	repoURL           string
	repoBranch        string
	githubAccessToken string
}

type initPipelineOpts struct {
	initPipelineVars
	// Interfaces to interact with dependencies.
	workspace      wsPipelineWriter
	secretsmanager secretsManager
	parser         template.Parser
	runner         runner
	sessProvider   sessionProvider
	cfnClient      appResourcesGetter
	store          store
	prompt         prompter
	sel            pipelineSelector

	// Outputs stored on successful actions.
	secret    string
	provider  string
	repoName  string
	repoOwner string
	repoShortName     string
	ccRegion  string

	// Caches variables
	fs           *afero.Afero
	repoBuffer   bytes.Buffer // Using two separate buffers for ease of testing.
	branchBuffer bytes.Buffer
	envConfigs   []*config.Environment
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

	prompter := prompt.New()

	return &initPipelineOpts{
		initPipelineVars: vars,
		workspace:        ws,
		secretsmanager:   secretsmanager,
		parser:           template.New(),
		sessProvider:     p,
		cfnClient:        cloudformation.New(defaultSession),
		store:            ssmStore,
		prompt:           prompter,
		sel:              selector.NewSelect(prompter, ssmStore),
		runner:           exec.NewCmd(),
		fs:               &afero.Afero{Fs: afero.NewOsFs()},
	}, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *initPipelineOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	if _, err := o.store.GetApplication(o.appName); err != nil {
		return err
	}

	// URL flag and branch flag must both be set or not.
	if URL, branch := o.repoURL == "", o.repoBranch == ""; URL != branch {
		return fmt.Errorf(`must specify either both '--%s' and '--%s' or neither`, repoURLFlag, gitBranchFlag)
	}
	if o.repoURL != "" {
		// Validate if URL of accepted source type (GitHub, Bitbucket, CodeCommit).
		if err := o.validateURLType(o.repoURL); err != nil {
			return err
		}
		// Validate if URL is an existent git remote.
		if err := o.validateURLExists(o.repoURL); err != nil {
			return err
		}
	}
	// Validate that branch exists for the given git remote.
	if o.repoBranch != "" {
		if err := o.validateBranch(); err != nil {
			return err
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
	if err := o.askRepository(); err != nil {
		return err
	}
	if err := o.askBranch(); err != nil {
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

	// write pipeline.yml file, populate with:
	//   - git repo as source
	//   - stage names (environments)
	//   - enable/disable transition to prod envs
	if err := o.createPipelineManifest(); err != nil {
		return err
	}
	if err := o.createBuildspec(); err != nil {
		return err
	}
	return nil
}

// RequiredActions returns follow-up actions the user must take after successfully executing the command.
func (o *initPipelineOpts) RequiredActions() []string {
	return []string{
		fmt.Sprintf("Commit and push the %s, %s, and %s files of your %s directory to your repository.", color.HighlightResource("buildspec.yml"), color.HighlightResource("pipeline.yml"), color.HighlightResource(".workspace"), color.HighlightResource("copilot")),
		fmt.Sprintf("Run %s to create your pipeline.", color.HighlightCode("copilot pipeline deploy")),
	}
}

func (o *initPipelineOpts) validateURLType(url string) error {
	// Note: no longer calling `validateDomainName` because if users use git-remote-codecommit
	// (the HTTPS (GRC) protocol) to connect to CodeCommit, the url does not have any periods.
	if !strings.Contains(url, githubURL) && !strings.Contains(url, ccIdentifier) && !strings.Contains(url, bbURL) {
		return fmt.Errorf("must be a URL to a supported provider (%s)", strings.Join(manifest.PipelineProviders, ", "))
	}
	return nil
}

func (o *initPipelineOpts) validateURLExists(url string) error {
	repos, err := o.fetchAndParseURLs()
	if err != nil {
		return fmt.Errorf("fetch and parse URLs: %w", err)
	}
	if _, ok := repos[url]; !ok {
		return fmt.Errorf("URL '%s' is not a local git remote; please check that you're in the correct directory", o.repoURL)
	}
	o.repoShortName = repos[url]
	return nil
}

func (o *initPipelineOpts) validateBranch() error {
	// URL has already been checked to exist and be valid; repoShortName already set by validateURLExists.
	// Fetches and parses all branches associated with the chosen repo.
	err := o.runner.Run("git", []string{"branch", "-a", "-l", o.repoShortName + "/*"}, exec.Stdout(&o.branchBuffer))
	if err != nil {
		return fmt.Errorf("get repo branch info: %w", err)
	}
	branches, err := o.parseGitBranchResults(strings.TrimSpace(o.branchBuffer.String()))
	if err != nil {
		return fmt.Errorf("parse 'git branch' results: %w", err)
	}
	o.branchBuffer.Reset()

	for _, branch := range branches {
		if branch == o.repoBranch {
			return nil
		}
	}
	return fmt.Errorf("branch %s not found for repo %s", o.repoBranch, o.repoShortName)
}

func (o *initPipelineOpts) fetchAndParseURLs() (map[string]string, error) {
	// Fetches and parses all remote repositories.
	err := o.runner.Run("git", []string{"remote", "-v"}, exec.Stdout(&o.repoBuffer))
	if err != nil {
		return nil, fmt.Errorf("get remote repository info: %w; make sure you have installed Git and are in a Git repository", err)
	}
	repos := o.parseGitRemoteResult(strings.TrimSpace(o.repoBuffer.String()))
	o.repoBuffer.Reset()

	return repos, nil
}

func (o *initPipelineOpts) askEnvs() error {
	if len(o.environments) == 0 {
		envs, err := o.sel.Environments(pipelineSelectEnvPrompt, pipelineSelectEnvHelpPrompt, o.appName, func(order int) prompt.PromptConfig {
			return prompt.WithFinalMessage(fmt.Sprintf("%s stage:", humanize.Ordinal(order)))
		})
		if err != nil {
			return fmt.Errorf("select environments: %w", err)
		}
		o.environments = envs
	}

	var envConfigs []*config.Environment
	for _, environment := range o.environments {
		envConfig, err := o.store.GetEnvironment(o.appName, environment)
		if err != nil {
			return fmt.Errorf("get config of environment: %w", err)
		}
		envConfigs = append(envConfigs, envConfig)
	}
	o.envConfigs = envConfigs

	return nil
}

func (o *initPipelineOpts) askRepository() error {
	var err error
	if o.repoURL == "" {
		if err = o.selectURL(); err != nil {
			return err
		}
	}

	switch {
	case strings.Contains(o.repoURL, githubURL):
		return o.askGitHubRepoDetails()
	case strings.Contains(o.repoURL, ccIdentifier):
		return o.parseCodeCommitRepoDetails()
	case strings.Contains(o.repoURL, bbURL):
		return o.parseBitbucketRepoDetails()
	}
	return nil
}

func (o *initPipelineOpts) askBranch() error {
	var err error
	if o.repoBranch == "" {
		if err = o.selectBranch(); err != nil {
			return err
		}
	}
	return nil
}

func (o *initPipelineOpts) askGitHubRepoDetails() error {
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
	repos, err := o.fetchAndParseURLs()
	if err != nil {
		return fmt.Errorf("fetch and parse URLs: %w", err)
	}

	var formattedRepos []string
	for url, name := range repos {
		formattedRepos = append(formattedRepos, fmt.Sprintf("%s: %s", name, url))
	}

	// Prompts user to select a repo URL.
	url, err := o.prompt.SelectOne(
		pipelineSelectURLPrompt,
		pipelineSelectURLHelpPrompt,
		formattedRepos,
		prompt.WithFinalMessage("Repository:"),
	)
	if err != nil {
		return fmt.Errorf("select URL: %w", err)
	}
	repoParts := strings.Split(url, ": ")
	if err := o.validateURLType(repoParts[1]); err != nil {
		return err
	}
	o.repoURL = repoParts[1]
	o.repoShortName = repoParts[0]

	return nil
}

func (o *initPipelineOpts) selectBranch() error {
	// Fetches and parses all branches associated with the chosen repo.
	err := o.runner.Run("git", []string{"branch", "-a", "-l", o.repoShortName + "/*"}, exec.Stdout(&o.branchBuffer))
	if err != nil {
		return fmt.Errorf("get repo branch info: %w", err)
	}
	branches, err := o.parseGitBranchResults(strings.TrimSpace(o.branchBuffer.String()))
	if err != nil {
		return fmt.Errorf("parse git branch results: %w", err)
	}
	o.branchBuffer.Reset()

	branch, err := o.prompt.SelectOne(
		pipelineSelectBranchPrompt,
		pipelineSelectBranchHelpPrompt,
		branches,
		prompt.WithFinalMessage("Repository branch:"),
	)
	if err != nil {
		return fmt.Errorf("select branch: %w", err)
	}
	o.repoBranch = branch

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

// parseGitRemoteResults returns just the first (shortname) and second (url) columns of the `git remote -v` results as a map (url: name), and skips urls from unsupported sources.
func (o *initPipelineOpts) parseGitRemoteResult(s string) map[string]string {
	repos := make(map[string]string)
	items := strings.Split(s, "\n")
	for _, item := range items {
		if !strings.Contains(item, githubURL) && !strings.Contains(item, ccIdentifier) && !strings.Contains(item, bbURL) {
			continue
		}
		cols := strings.Split(item, "\t")
		URL := strings.TrimSpace(strings.Split(cols[1], " ")[0])
		repos[URL] = cols[0]
	}
	return repos
}

func (o *initPipelineOpts) parseGitBranchResults(s string) ([]string, error) {
	var branches []string
	items := strings.Split(s, "\n")
	for _, item := range items {
		elements := strings.Split(item, "/")
		if len(elements) < 3 {
			return nil, fmt.Errorf("unparsable format")
		}
		branchName := elements[2]
		branches = append(branches, branchName)
	}
	return branches, nil
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

func (o *initPipelineOpts) createPipelineManifest() error {
	pipelineName := o.pipelineName()

	provider, err := o.pipelineProvider()
	if err != nil {
		return err
	}

	var stages []manifest.PipelineStage
	for _, env := range o.envConfigs {

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
	log.Successf(manifestMsgFmt, color.HighlightUserInput(o.repoName), color.HighlightResource(manifestPath))
	log.Infof(`The manifest contains configurations for your CodePipeline resources, such as your pipeline stages and build steps.
Update the file to add additional stages, change the branch to be tracked, or add test commands or manual approval actions.
`)
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
	log.Infof(`The buildspec contains the commands to build and push your container images to your ECR repositories.
Update the %s phase to unit test your services before pushing the images.
`, color.HighlightResource("build"))

	return nil
}

func (o *initPipelineOpts) secretName() string {
	return fmt.Sprintf(fmtSecretName, o.appName, o.repoName)
}

func (o *initPipelineOpts) pipelineName() string {
	name := fmt.Sprintf(fmtPipelineName, o.appName, o.repoName)
	if len(name) <= 100 {
		return name
	}
	return name[:100]
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
  /code  --url https://github.com/gitHubUserName/myFrontendApp.git \
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
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVar(&vars.repoURL, githubURLFlag, "", githubURLFlagDescription)
	_ = cmd.Flags().MarkHidden(githubURLFlag)
	cmd.Flags().StringVarP(&vars.repoURL, repoURLFlag, repoURLFlagShort, "", repoURLFlagDescription)
	cmd.Flags().StringVarP(&vars.githubAccessToken, githubAccessTokenFlag, githubAccessTokenFlagShort, "", githubAccessTokenFlagDescription)
	_ = cmd.Flags().MarkHidden(githubAccessTokenFlag)
	cmd.Flags().StringVarP(&vars.repoBranch, gitBranchFlag, gitBranchFlagShort, "", gitBranchFlagDescription)
	cmd.Flags().StringSliceVarP(&vars.environments, envsFlag, envsFlagShort, []string{}, pipelineEnvsFlagDescription)

	return cmd
}

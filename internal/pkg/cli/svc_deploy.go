// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"golang.org/x/mod/semver"

	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/tags"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/repository"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
)

var aliasUsedWithoutDomainFriendlyText = fmt.Sprintf("To use %s, your application must be associated with a domain: %s.\n",
	color.HighlightCode("http.alias"),
	color.HighlightCode("copilot app init --domain example.com"))

type deployWkldVars struct {
	appName        string
	name           string
	envName        string
	imageTag       string
	resourceTags   map[string]string
	forceNewUpdate bool

	// To facilitate unit tests.
	clientConfigured bool
}

type uploadCustomResourcesOpts struct {
	uploader      customResourcesUploader
	newS3Uploader func() (uploader, error)
}

type deploySvcOpts struct {
	deployWkldVars

	store              store
	deployStore        *deploy.Store
	ws                 wsWlDirReader
	fs                 *afero.Afero
	imageBuilderPusher imageBuilderPusher
	unmarshal          func([]byte) (manifest.WorkloadManifest, error)
	newInterpolator    func(app, env string) interpolator
	s3                 uploader
	cmd                runner
	addons             templater
	svcCFN             serviceDeployer
	newSvcUpdater      func(func(*session.Session) clideploy.ServiceForceUpdater) clideploy.ServiceForceUpdater
	sessProvider       sessionProvider
	envUpgradeCmd      actionCommand
	appVersionGetter   versionGetter
	endpointGetter     endpointGetter
	snsTopicGetter     deployedEnvironmentLister
	envDescriber       envDescriber
	newSvcDeployer     func(*deploySvcOpts) (workloadDeployer, error)

	spinner progress
	sel     wsSelector
	prompt  prompter

	// cached variables
	targetApp       *config.Application
	targetEnv       *config.Environment
	svcType         string
	appliedManifest interface{}
	appResources    *stack.AppRegionalResources
	rootUserARN     string
	rdSvcAlias      string
	subscriptions   []manifest.TopicSubscription

	uploadOpts *uploadCustomResourcesOpts
}

func newSvcDeployOpts(vars deployWkldVars) (*deploySvcOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}
	deployStore, err := deploy.NewStore(store)
	if err != nil {
		return nil, fmt.Errorf("new deploy store: %w", err)
	}
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	prompter := prompt.New()
	opts := &deploySvcOpts{
		deployWkldVars: vars,

		store:           store,
		deployStore:     deployStore,
		ws:              ws,
		fs:              &afero.Afero{Fs: afero.NewOsFs()},
		unmarshal:       manifest.UnmarshalWorkload,
		spinner:         termprogress.NewSpinner(log.DiagnosticWriter),
		sel:             selector.NewWorkspaceSelect(prompter, store, ws),
		prompt:          prompter,
		newInterpolator: newManifestInterpolator,
		cmd:             exec.NewCmd(),
		sessProvider:    sessions.NewProvider(),
		snsTopicGetter:  deployStore,
		newSvcDeployer: func(o *deploySvcOpts) (workloadDeployer, error) {
			deployer, err := clideploy.NewWorkloadDeployer(&clideploy.WorkloadDeployerInput{
				Name:     o.name,
				App:      o.targetApp,
				Env:      o.targetEnv,
				ImageTag: o.imageTag,
				S3Bucket: o.appResources.S3Bucket,
				Mft:      o.appliedManifest,
			})
			if err != nil {
				return nil, fmt.Errorf("initiate workload deployer: %w", err)
			}
			return deployer, nil
		},
	}
	return opts, err
}

func newManifestInterpolator(app, env string) interpolator {
	return manifest.NewInterpolator(app, env)
}

// Validate returns an error if the user inputs are invalid.
func (o *deploySvcOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	if o.name != "" {
		if err := o.validateSvcName(); err != nil {
			return err
		}
	}
	if o.envName != "" {
		if err := o.validateEnvName(); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts the user for any required fields that are not provided.
func (o *deploySvcOpts) Ask() error {
	if err := o.askSvcName(); err != nil {
		return err
	}
	if err := o.askEnvName(); err != nil {
		return err
	}
	return nil
}

// Execute builds and pushes the container image for the service,
func (o *deploySvcOpts) Execute() error {
	if !o.clientConfigured {
		if err := o.configureClients(); err != nil {
			return err
		}
	}
	if err := o.envUpgradeCmd.Execute(); err != nil {
		return fmt.Errorf(`execute "env upgrade --app %s --name %s": %v`, o.appName, o.envName, err)
	}
	mft, err := workloadManifest(&workloadManifestInput{
		name:         o.name,
		appName:      o.appName,
		envName:      o.envName,
		interpolator: o.newInterpolator(o.appName, o.envName),
		ws:           o.ws,
		unmarshal:    o.unmarshal,
	})
	if err != nil {
		return err
	}
	o.appliedManifest = mft
	deployer, err := o.newSvcDeployer(o)
	if err != nil {
		return err
	}
	uploadOut, err := deployer.UploadArtifacts(&clideploy.UploadArtifactsInput{
		FS:                 o.fs,
		Uploader:           o.s3,
		Templater:          o.addons,
		ImageBuilderPusher: o.imageBuilderPusher,
	})
	if err != nil {
		return fmt.Errorf("upload deploy resources for service %s: %w", o.name, err)
	}
	deployOut, err := deployer.DeployWorkload(&clideploy.DeployWorkloadInput{
		ImageDigest:            uploadOut.ImageDigest,
		EnvFileARN:             uploadOut.EnvFileARN,
		AddonsURL:              uploadOut.AddonsURL,
		RootUserARN:            o.rootUserARN,
		Tags:                   tags.Merge(o.targetApp.Tags, o.resourceTags),
		ImageRepoURL:           o.appResources.RepositoryURLs[o.name],
		ForceNewUpdate:         o.forceNewUpdate,
		CustomResourceUploader: o.uploadOpts.uploader,
		S3Uploader:             o.s3,
		SNSTopicsLister:        o.snsTopicGetter,
		ServiceDeployer:        o.svcCFN,
		NewSvcUpdater:          o.newSvcUpdater,
		AppVersionGetter:       o.appVersionGetter,
		PublicCIDRBlocksGetter: o.envDescriber,
		EndpointGetter:         o.endpointGetter,
		Spinner:                o.spinner,
		Now:                    time.Now,
	})
	if err != nil {
		return fmt.Errorf("deploy service %s to environment %s: %w", o.name, o.envName, err)
	}
	o.rdSvcAlias = deployOut.RDWSAlias
	o.subscriptions = deployOut.Subscriptions

	log.Successf("Deployed service %s.\n", color.HighlightUserInput(o.name))
	return nil
}

// RecommendActions returns follow-up actions the user can take after successfully executing the command.
func (o *deploySvcOpts) RecommendActions() error {
	var recommendations []string
	uriRecs, err := o.uriRecommendedActions()
	if err != nil {
		return err
	}
	recommendations = append(recommendations, uriRecs...)
	recommendations = append(recommendations, o.publishRecommendedActions()...)
	recommendations = append(recommendations, o.subscribeRecommendedActions()...)
	logRecommendedActions(recommendations)
	return nil
}

func (o *deploySvcOpts) validateSvcName() error {
	names, err := o.ws.ListServices()
	if err != nil {
		return fmt.Errorf("list services in the workspace: %w", err)
	}
	for _, name := range names {
		if o.name == name {
			return nil
		}
	}
	return fmt.Errorf("service %s not found in the workspace", color.HighlightUserInput(o.name))
}

func (o *deploySvcOpts) validateEnvName() error {
	if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
		return fmt.Errorf("get environment %s configuration: %w", o.envName, err)
	}
	return nil
}

func (o *deploySvcOpts) askSvcName() error {
	if o.name != "" {
		return nil
	}

	name, err := o.sel.Service("Select a service in your workspace", "")
	if err != nil {
		return fmt.Errorf("select service: %w", err)
	}
	o.name = name
	return nil
}

func (o *deploySvcOpts) askEnvName() error {
	if o.envName != "" {
		return nil
	}

	name, err := o.sel.Environment("Select an environment", "", o.appName)
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.envName = name
	return nil
}

func (o *deploySvcOpts) configureClients() error {
	o.imageTag = imageTagFromGit(o.cmd, o.imageTag) // Best effort assign git tag.
	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		return fmt.Errorf("get application %s configuration: %w", o.appName, err)
	}
	o.targetApp = app
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return fmt.Errorf("get environment %s configuration: %w", o.envName, err)
	}
	o.targetEnv = env
	svc, err := o.store.GetService(o.appName, o.name)
	if err != nil {
		return fmt.Errorf("get service %s configuration: %w", o.name, err)
	}
	o.svcType = svc.Type

	defaultSessEnvRegion, err := o.sessProvider.DefaultWithRegion(env.Region)
	if err != nil {
		return fmt.Errorf("create ECR session with region %s: %w", env.Region, err)
	}

	envSession, err := o.sessProvider.FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return fmt.Errorf("assuming environment manager role: %w", err)
	}

	d, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
		App:         o.appName,
		Env:         o.envName,
		ConfigStore: o.store,
		DeployStore: o.deployStore,
	})
	if err != nil {
		return fmt.Errorf("create describer for environment %s in application %s: %w", o.envName, o.appName, err)
	}
	o.envDescriber = d

	// ECR client against tools account profile AND target environment region.
	repoName := fmt.Sprintf("%s/%s", o.appName, o.name)
	registry := ecr.New(defaultSessEnvRegion)
	o.imageBuilderPusher, err = repository.New(repoName, registry)
	if err != nil {
		return fmt.Errorf("initiate image builder pusher: %w", err)
	}

	o.s3 = s3.New(envSession)

	o.newSvcUpdater = func(f func(*session.Session) clideploy.ServiceForceUpdater) clideploy.ServiceForceUpdater {
		return f(envSession)
	}

	// CF client against env account profile AND target environment region.
	o.svcCFN = cloudformation.New(envSession)

	o.endpointGetter, err = describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
		App:         o.appName,
		Env:         o.envName,
		ConfigStore: o.store,
	})
	if err != nil {
		return fmt.Errorf("initiate env describer: %w", err)
	}
	addonsSvc, err := addon.New(o.name)
	if err != nil {
		return fmt.Errorf("initiate addons service: %w", err)
	}
	o.addons = addonsSvc

	// client to retrieve an application's resources created with CloudFormation.
	defaultSess, err := o.sessProvider.Default()
	if err != nil {
		return fmt.Errorf("create default session: %w", err)
	}

	resources, err := cloudformation.New(defaultSess).GetAppResourcesByRegion(app, env.Region)
	if err != nil {
		return fmt.Errorf("get application %s resources from region %s: %w", app.Name, env.Region, err)
	}
	o.appResources = resources

	cmd, err := newEnvUpgradeOpts(envUpgradeVars{
		appName: o.appName,
		name:    env.Name,
	})
	if err != nil {
		return fmt.Errorf("new env upgrade command: %v", err)
	}
	o.envUpgradeCmd = cmd

	// client to retrieve caller identity.
	caller, err := identity.New(defaultSess).Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}
	o.rootUserARN = caller.RootUserARN

	o.uploadOpts = &uploadCustomResourcesOpts{
		uploader: template.New(),
		newS3Uploader: func() (uploader, error) {
			envRegion := env.Region
			sess, err := o.sessProvider.DefaultWithRegion(env.Region)
			if err != nil {
				return nil, fmt.Errorf("create session with region %s: %w", envRegion, err)
			}
			s3Client := s3.New(sess)
			return s3Client, nil
		},
	}

	versionGetter, err := describe.NewAppDescriber(o.appName)
	if err != nil {
		return fmt.Errorf("new app describer for application %s: %w", o.appName, err)
	}
	o.appVersionGetter = versionGetter

	return nil
}

type workloadManifestInput struct {
	name         string
	appName      string
	envName      string
	ws           wsWlDirReader
	interpolator interpolator
	unmarshal    func([]byte) (manifest.WorkloadManifest, error)
}

func workloadManifest(in *workloadManifestInput) (interface{}, error) {
	raw, err := in.ws.ReadWorkloadManifest(in.name)
	if err != nil {
		return nil, fmt.Errorf("read manifest file for %s: %w", in.name, err)
	}
	interpolated, err := in.interpolator.Interpolate(string(raw))
	if err != nil {
		return nil, fmt.Errorf("interpolate environment variables for %s manifest: %w", in.name, err)
	}
	mft, err := in.unmarshal([]byte(interpolated))
	if err != nil {
		return nil, fmt.Errorf("unmarshal service %s manifest: %w", in.name, err)
	}
	envMft, err := mft.ApplyEnv(in.envName)
	if err != nil {
		return nil, fmt.Errorf("apply environment %s override: %s", in.envName, err)
	}
	if err := envMft.Validate(); err != nil {
		return nil, fmt.Errorf("validate manifest against environment %s: %s", in.envName, err)
	}
	return envMft, nil
}

func uploadRDWSCustomResources(o *uploadCustomResourcesOpts, appEnvResources *stack.AppRegionalResources) (map[string]string, error) {
	s3Client, err := o.newS3Uploader()
	if err != nil {
		return nil, err
	}

	urls, err := o.uploader.UploadRequestDrivenWebServiceCustomResources(func(key string, objects ...s3.NamedBinary) (string, error) {
		return s3Client.ZipAndUpload(appEnvResources.S3Bucket, key, objects...)
	})
	if err != nil {
		return nil, fmt.Errorf("upload custom resources to bucket %s: %w", appEnvResources.S3Bucket, err)
	}

	return urls, nil
}

func validateLBSvcAlias(aliases manifest.Alias, app *config.Application, envName string) error {
	if aliases.IsEmpty() {
		return nil
	}
	aliasList, err := aliases.ToStringSlice()
	if err != nil {
		return fmt.Errorf(`convert 'http.alias' to string slice: %w`, err)
	}
	for _, alias := range aliasList {
		// Alias should be within either env, app, or root hosted zone.
		var regEnvHostedZone, regAppHostedZone, regRootHostedZone *regexp.Regexp
		var err error
		if regEnvHostedZone, err = regexp.Compile(fmt.Sprintf(`^([^\.]+\.)?%s.%s.%s`, envName, app.Name, app.Domain)); err != nil {
			return err
		}
		if regAppHostedZone, err = regexp.Compile(fmt.Sprintf(`^([^\.]+\.)?%s.%s`, app.Name, app.Domain)); err != nil {
			return err
		}
		if regRootHostedZone, err = regexp.Compile(fmt.Sprintf(`^([^\.]+\.)?%s`, app.Domain)); err != nil {
			return err
		}
		var validAlias bool
		for _, re := range []*regexp.Regexp{regEnvHostedZone, regAppHostedZone, regRootHostedZone} {
			if re.MatchString(alias) {
				validAlias = true
				break
			}
		}
		if validAlias {
			continue
		}
		log.Errorf(`%s must match one of the following patterns:
- %s.%s.%s,
- <name>.%s.%s.%s,
- %s.%s,
- <name>.%s.%s,
- %s,
- <name>.%s
`, color.HighlightCode("http.alias"), envName, app.Name, app.Domain, envName,
			app.Name, app.Domain, app.Name, app.Domain, app.Name,
			app.Domain, app.Domain, app.Domain)
		return fmt.Errorf(`alias "%s" is not supported in hosted zones managed by Copilot`, alias)
	}
	return nil
}

func checkUnsupportedRDSvcAlias(alias, envName string, app *config.Application) error {
	var regEnvHostedZone, regAppHostedZone *regexp.Regexp
	var err error
	// Example: subdomain.env.app.domain, env.app.domain
	if regEnvHostedZone, err = regexp.Compile(fmt.Sprintf(`^([^\.]+\.)?%s.%s.%s`, envName, app.Name, app.Domain)); err != nil {
		return err
	}

	// Example: subdomain.app.domain, app.domain
	if regAppHostedZone, err = regexp.Compile(fmt.Sprintf(`^([^\.]+\.)?%s.%s`, app.Name, app.Domain)); err != nil {
		return err
	}

	if regEnvHostedZone.MatchString(alias) {
		return fmt.Errorf("%s is an environment-level alias, which is not supported yet", alias)
	}

	if regAppHostedZone.MatchString(alias) {
		return fmt.Errorf("%s is an application-level alias, which is not supported yet", alias)
	}

	if alias == app.Domain {
		return fmt.Errorf("%s is a root domain alias, which is not supported yet", alias)
	}

	return nil
}

func validateRDSvcAliasAndAppVersion(svcName, alias, envName string, app *config.Application, appVersionGetter versionGetter) error {
	if alias == "" {
		return nil
	}
	if err := validateAppVersion(app.Name, appVersionGetter); err != nil {
		logAppVersionOutdatedError(svcName)
		return err
	}
	// Alias should be within root hosted zone.
	aliasInvalidLog := fmt.Sprintf(`%s of %s field should match the pattern <subdomain>.%s 
Where <subdomain> cannot be the application name.
`, color.HighlightUserInput(alias), color.HighlightCode("http.alias"), app.Domain)
	if err := checkUnsupportedRDSvcAlias(alias, envName, app); err != nil {
		log.Errorf(aliasInvalidLog)
		return err
	}

	// Example: subdomain.domain
	regRootHostedZone, err := regexp.Compile(fmt.Sprintf(`^([^\.]+\.)%s`, app.Domain))
	if err != nil {
		return err
	}

	if regRootHostedZone.MatchString(alias) {
		return nil
	}

	log.Errorf(aliasInvalidLog)
	return fmt.Errorf("alias is not supported in hosted zones that are not managed by Copilot")
}

func validateAppVersion(appName string, appVersionGetter versionGetter) error {
	appVersion, err := appVersionGetter.Version()
	if err != nil {
		return fmt.Errorf("get version for app %s: %w", appName, err)
	}
	diff := semver.Compare(appVersion, deploy.AliasLeastAppTemplateVersion)
	if diff < 0 {
		return fmt.Errorf(`alias is not compatible with application versions below %s`, deploy.AliasLeastAppTemplateVersion)
	}
	return nil
}

func validateLBWSRuntime(app *config.Application, envName string, mft *manifest.LoadBalancedWebService, appVersionGetter versionGetter) error {
	if app.Domain == "" && mft.HasAliases() {
		log.Errorf(aliasUsedWithoutDomainFriendlyText)
		return errors.New("alias specified when application is not associated with a domain")
	}

	if app.RequiresDNSDelegation() {
		if err := validateAppVersion(app.Name, appVersionGetter); err != nil {
			logAppVersionOutdatedError(aws.StringValue(mft.Name))
			return err
		}
	}

	if err := validateLBSvcAlias(mft.RoutingRule.Alias, app, envName); err != nil {
		return err
	}
	return validateLBSvcAlias(mft.NLBConfig.Aliases, app, envName)
}

func logAppVersionOutdatedError(name string) {
	log.Errorf(`Cannot deploy service %s because the application version is incompatible.
To upgrade the application, please run %s first (see https://aws.github.io/copilot-cli/docs/credentials/#application-credentials).
`, name, color.HighlightCode("copilot app upgrade"))
}

func (o *deploySvcOpts) uriRecommendedActions() ([]string, error) {
	type reachable interface {
		Port() (uint16, bool)
	}
	mft, ok := o.appliedManifest.(reachable)
	if !ok {
		return nil, nil
	}
	if _, ok := mft.Port(); !ok { // No exposed port.
		return nil, nil
	}

	describer, err := describe.NewReachableService(o.appName, o.name, o.store)
	if err != nil {
		return nil, err
	}
	uri, err := describer.URI(o.envName)
	if err != nil {
		return nil, fmt.Errorf("get uri for environment %s: %w", o.envName, err)
	}

	network := "over the internet."
	if o.svcType == manifest.BackendServiceType {
		network = "with service discovery."
	}
	recs := []string{
		fmt.Sprintf("You can access your service at %s %s", color.HighlightResource(uri), network),
	}
	if o.rdSvcAlias != "" {
		recs = append(recs, fmt.Sprintf(`The validation process for https://%s can take more than 15 minutes.
    Please visit %s to check the validation status.`, o.rdSvcAlias, color.Emphasize("https://console.aws.amazon.com/apprunner/home")))
	}
	return recs, nil
}

func (o *deploySvcOpts) subscribeRecommendedActions() []string {
	type subscriber interface {
		Subscriptions() []manifest.TopicSubscription
	}
	if _, ok := o.appliedManifest.(subscriber); !ok {
		return nil
	}
	retrieveEnvVarCode := "const eventsQueueURI = process.env.COPILOT_QUEUE_URI"
	actionRetrieveEnvVar := fmt.Sprintf(
		`Update %s's code to leverage the injected environment variable "COPILOT_QUEUE_URI".
    In JavaScript you can write %s.`,
		o.name,
		color.HighlightCode(retrieveEnvVarCode),
	)
	recs := []string{actionRetrieveEnvVar}
	topicQueueNames := o.buildWorkerQueueNames()
	if topicQueueNames == "" {
		return recs
	}
	retrieveTopicQueueEnvVarCode := fmt.Sprintf("const {%s} = JSON.parse(process.env.COPILOT_TOPIC_QUEUE_URIS)", topicQueueNames)
	actionRetrieveTopicQueues := fmt.Sprintf(
		`You can retrieve topic-specific queues by writing
    %s.`,
		color.HighlightCode(retrieveTopicQueueEnvVarCode),
	)
	recs = append(recs, actionRetrieveTopicQueues)
	return recs
}

func (o *deploySvcOpts) publishRecommendedActions() []string {
	type publisher interface {
		Publish() []manifest.Topic
	}
	mft, ok := o.appliedManifest.(publisher)
	if !ok {
		return nil
	}
	if topics := mft.Publish(); len(topics) == 0 {
		return nil
	}

	return []string{
		fmt.Sprintf(`Update %s's code to leverage the injected environment variable "COPILOT_SNS_TOPIC_ARNS".
    In JavaScript you can write %s.`,
			o.name,
			color.HighlightCode("const {<topicName>} = JSON.parse(process.env.COPILOT_SNS_TOPIC_ARNS)")),
	}
}

func (o *deploySvcOpts) buildWorkerQueueNames() string {
	sb := new(strings.Builder)
	first := true
	for _, subscription := range o.subscriptions {
		if subscription.Queue.IsEmpty() {
			continue
		}
		topicSvc := template.StripNonAlphaNumFunc(aws.StringValue(subscription.Service))
		topicName := template.StripNonAlphaNumFunc(aws.StringValue(subscription.Name))
		subName := fmt.Sprintf("%s%sEventsQueue", topicSvc, strings.Title(topicName))
		if first {
			sb.WriteString(subName)
			first = false
		} else {
			sb.WriteString(fmt.Sprintf(", %s", subName))
		}
	}
	return sb.String()
}

// buildSvcDeployCmd builds the `svc deploy` subcommand.
func buildSvcDeployCmd() *cobra.Command {
	vars := deployWkldVars{}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys a service to an environment.",
		Long:  `Deploys a service to an environment.`,
		Example: `
  Deploys a service named "frontend" to a "test" environment.
  /code $ copilot svc deploy --name frontend --env test
  Deploys a service with additional resource tags.
  /code $ copilot svc deploy --resource-tags source/revision=bb133e7,deployment/initiator=manual`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newSvcDeployOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.imageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringToStringVar(&vars.resourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)
	cmd.Flags().BoolVar(&vars.forceNewUpdate, forceFlag, false, forceFlagDescription)

	return cmd
}

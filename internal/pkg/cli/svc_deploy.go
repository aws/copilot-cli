// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	awscfn "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/tags"
	deploycfn "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/spf13/afero"
	"golang.org/x/mod/semver"

	"github.com/spf13/cobra"

	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	clideploy "github.com/aws/copilot-cli/internal/pkg/cli/deploy"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
)

type deployWkldVars struct {
	appName            string
	name               string
	envName            string
	imageTag           string
	resourceTags       map[string]string
	forceNewUpdate     bool // NOTE: this variable is not applicable for a job workload currently.
	disableRollback    bool
	showDiff           bool
	skipDiffPrompt     bool
	allowWkldDowngrade bool
	detach             bool

	// To facilitate unit tests.
	clientConfigured bool
}

type deploySvcOpts struct {
	deployWkldVars

	store                store
	ws                   wsWlDirReader
	unmarshal            func([]byte) (manifest.DynamicWorkload, error)
	newInterpolator      func(app, env string) interpolator
	cmd                  execRunner
	sessProvider         *sessions.Provider
	newSvcDeployer       func() (workloadDeployer, error)
	svcVersionGetter     versionGetter
	envFeaturesDescriber versionCompatibilityChecker
	diffWriter           io.Writer

	spinner        progress
	sel            wsSelector
	prompt         prompter
	gitShortCommit string

	// cached variables
	targetApp         *config.Application
	targetEnv         *config.Environment
	envSess           *session.Session
	svcType           string
	appliedDynamicMft manifest.DynamicWorkload
	rawMft            string
	rootUserARN       string
	deployRecs        clideploy.ActionRecommender
	noDeploy          bool

	// Overridden in tests.
	templateVersion string
}

func newSvcDeployOpts(vars deployWkldVars) (*deploySvcOpts, error) {
	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}

	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("svc deploy"))
	defaultSession, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}

	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	prompter := prompt.New()

	opts := &deploySvcOpts{
		deployWkldVars: vars,

		store:           store,
		ws:              ws,
		unmarshal:       manifest.UnmarshalWorkload,
		spinner:         termprogress.NewSpinner(log.DiagnosticWriter),
		sel:             selector.NewLocalWorkloadSelector(prompter, store, ws, selector.OnlyInitializedWorkloads),
		prompt:          prompter,
		newInterpolator: newManifestInterpolator,
		cmd:             exec.NewCmd(),
		sessProvider:    sessProvider,
		diffWriter:      os.Stdout,
		templateVersion: version.LatestTemplateVersion(),
	}
	opts.newSvcDeployer = func() (workloadDeployer, error) {
		// NOTE: Defined as a struct member to facilitate unit testing.
		return newSvcDeployer(opts)
	}
	return opts, err
}

func newSvcDeployer(o *deploySvcOpts) (workloadDeployer, error) {
	targetApp, err := o.getTargetApp()
	if err != nil {
		return nil, err
	}
	ovrdr, err := clideploy.NewOverrider(o.ws.WorkloadOverridesPath(o.name), o.appName, o.envName, afero.NewOsFs(), o.sessProvider)
	if err != nil {
		return nil, err
	}

	content := o.appliedDynamicMft.Manifest()
	var deployer workloadDeployer
	in := clideploy.WorkloadDeployerInput{
		SessionProvider: o.sessProvider,
		Name:            o.name,
		App:             targetApp,
		Env:             o.targetEnv,
		Image: clideploy.ContainerImageIdentifier{
			CustomTag:         o.imageTag,
			GitShortCommitTag: o.gitShortCommit,
		},
		Mft:              content,
		RawMft:           o.rawMft,
		EnvVersionGetter: o.envFeaturesDescriber,
		Overrider:        ovrdr,
	}
	switch t := content.(type) {
	case *manifest.LoadBalancedWebService:
		deployer, err = clideploy.NewLBWSDeployer(&in)
	case *manifest.BackendService:
		deployer, err = clideploy.NewBackendDeployer(&in)
	case *manifest.RequestDrivenWebService:
		deployer, err = clideploy.NewRDWSDeployer(&in)
	case *manifest.WorkerService:
		deployer, err = clideploy.NewWorkerSvcDeployer(&in)
	case *manifest.StaticSite:
		deployer, err = clideploy.NewStaticSiteDeployer(&in)
	default:
		return nil, fmt.Errorf("unknown manifest type %T while creating the CloudFormation stack", t)
	}
	if err != nil {
		return nil, fmt.Errorf("initiate workload deployer: %w", err)
	}
	return deployer, nil
}

func newManifestInterpolator(app, env string) interpolator {
	return manifest.NewInterpolator(app, env)
}

// Validate returns an error for any invalid optional flags.
func (o *deploySvcOpts) Validate() error {
	return nil
}

// Ask prompts for and validates any required flags.
func (o *deploySvcOpts) Ask() error {
	if o.appName != "" {
		if _, err := o.getTargetApp(); err != nil {
			return err
		}
	} else {
		// NOTE: This command is required to be executed under a workspace. We don't prompt for it.
		return errNoAppInWorkspace
	}

	if err := o.validateOrAskSvcName(); err != nil {
		return err
	}

	if err := o.validateOrAskEnvName(); err != nil {
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
	if !o.allowWkldDowngrade {
		if err := validateWkldVersion(o.svcVersionGetter, o.name, o.templateVersion); err != nil {
			return err
		}
	}
	mft, interpolated, err := workloadManifest(&workloadManifestInput{
		name:         o.name,
		appName:      o.appName,
		envName:      o.envName,
		ws:           o.ws,
		interpolator: o.newInterpolator(o.appName, o.envName),
		unmarshal:    o.unmarshal,
		sess:         o.envSess,
	})
	if err != nil {
		return err
	}
	o.rawMft = interpolated
	o.appliedDynamicMft = mft
	if o.forceNewUpdate && o.svcType == manifestinfo.StaticSiteType {
		return fmt.Errorf("--%s is not supported for service type %q", forceFlag, manifestinfo.StaticSiteType)
	}
	if err := validateWorkloadManifestCompatibilityWithEnv(o.ws, o.envFeaturesDescriber, mft, o.envName); err != nil {
		return err
	}
	deployer, err := o.newSvcDeployer()
	if err != nil {
		return err
	}
	serviceInRegion, err := deployer.IsServiceAvailableInRegion(o.targetEnv.Region)
	if err != nil {
		return fmt.Errorf("check if %s is available in region %s: %w", o.svcType, o.targetEnv.Region, err)
	}
	if !serviceInRegion {
		log.Warningf(`%s might not be available in region %s; proceed with caution.
`, o.svcType, o.targetEnv.Region)
	}
	uploadOut, err := deployer.UploadArtifacts()
	if err != nil {
		return fmt.Errorf("upload deploy resources for service %s: %w", o.name, err)
	}
	targetApp, err := o.getTargetApp()
	if err != nil {
		return err
	}
	if o.showDiff {
		output, err := deployer.GenerateCloudFormationTemplate(&clideploy.GenerateCloudFormationTemplateInput{
			StackRuntimeConfiguration: clideploy.StackRuntimeConfiguration{
				RootUserARN:               o.rootUserARN,
				Tags:                      targetApp.Tags,
				EnvFileARNs:               uploadOut.EnvFileARNs,
				ImageDigests:              uploadOut.ImageDigests,
				AddonsURL:                 uploadOut.AddonsURL,
				CustomResourceURLs:        uploadOut.CustomResourceURLs,
				StaticSiteAssetMappingURL: uploadOut.StaticSiteAssetMappingLocation,
				Version:                   o.templateVersion,
			},
		})
		if err != nil {
			return fmt.Errorf("generate the template for workload %q against environment %q: %w", o.name, o.envName, err)
		}
		if err := diff(deployer, output.Template, o.diffWriter); err != nil {
			var errHasDiff *errHasDiff
			if !errors.As(err, &errHasDiff) {
				return err
			}
		}
		contd, err := o.skipDiffPrompt, nil
		if !o.skipDiffPrompt {
			contd, err = o.prompt.Confirm(continueDeploymentPrompt, "")
		}
		if err != nil {
			return fmt.Errorf("ask whether to continue with the deployment: %w", err)
		}
		if !contd {
			o.noDeploy = true
			return nil
		}
	}
	deployRecs, err := deployer.DeployWorkload(&clideploy.DeployWorkloadInput{
		StackRuntimeConfiguration: clideploy.StackRuntimeConfiguration{
			ImageDigests:              uploadOut.ImageDigests,
			EnvFileARNs:               uploadOut.EnvFileARNs,
			AddonsURL:                 uploadOut.AddonsURL,
			RootUserARN:               o.rootUserARN,
			Tags:                      tags.Merge(targetApp.Tags, o.resourceTags),
			CustomResourceURLs:        uploadOut.CustomResourceURLs,
			StaticSiteAssetMappingURL: uploadOut.StaticSiteAssetMappingLocation,
			Version:                   o.templateVersion,
		},
		Options: clideploy.Options{
			ForceNewUpdate:  o.forceNewUpdate,
			DisableRollback: o.disableRollback,
			Detach:          o.detach,
		},
	})
	if err != nil {
		var errStackDeletedOnInterrupt *deploycfn.ErrStackDeletedOnInterrupt
		var errStackUpdateCanceledOnInterrupt *deploycfn.ErrStackUpdateCanceledOnInterrupt
		var errEmptyChangeSet *awscfn.ErrChangeSetEmpty
		if errors.As(err, &errStackDeletedOnInterrupt) {
			o.noDeploy = true
			return nil
		}
		if errors.As(err, &errStackUpdateCanceledOnInterrupt) {
			log.Successf("Successfully rolled back service %s to the previous configuration.\n", color.HighlightUserInput(o.name))
			o.noDeploy = true
			return nil
		}
		if o.disableRollback {
			stackName := stack.NameForWorkload(o.targetApp.Name, o.targetEnv.Name, o.name)
			rollbackCmd := fmt.Sprintf("aws cloudformation rollback-stack --stack-name %s --role-arn %s", stackName, o.targetEnv.ExecutionRoleARN)
			log.Infof(`It seems like you have disabled automatic stack rollback for this deployment. To debug, you can:
* Run %s to inspect the service log.
* Visit the AWS console to inspect the errors.
After fixing the deployment, you can:
1. Run %s to rollback the deployment.
2. Run %s to make a new deployment.
`, color.HighlightCode("copilot svc logs"), color.HighlightCode(rollbackCmd), color.HighlightCode("copilot svc deploy"))
		}
		if errors.As(err, &errEmptyChangeSet) {
			return &errNoInfrastructureChanges{parentErr: err}
		}
		return fmt.Errorf("deploy service %s to environment %s: %w", o.name, o.envName, err)
	}
	if o.detach {
		return nil
	}
	log.Successf("Deployed service %s.\n", color.HighlightUserInput(o.name))
	o.deployRecs = deployRecs
	return nil
}

// RecommendActions returns follow-up actions the user can take after successfully executing the command.
func (o *deploySvcOpts) RecommendActions() error {
	if lbMft, ok := o.appliedDynamicMft.Manifest().(*manifest.LoadBalancedWebService); ok {
		if !lbMft.NLBConfig.IsEmpty() {
			log.Warning("With v1.33.0, Copilot applies a security group to your network load balancer. ",
				"This allows more fine-grained intra-VPC access control: ",
				"your service won't need to allow-list the CIDR blocks of the public subnets where the NLB is deployed; ",
				"it only needs to allow-list the NLB, specifically.\n",
				"\n",
				"NLB security group onboarding implies resource recreation, ",
				"because a security group can't be added to an existing NLB that does not already have one. ",
				"Therefore, you may see some resource recreation related to your NLB. ",
				"This means:\n",
				"1. If you don't use DNS aliases, then the NLB's domain name will change.\n",
				"2. If you use DNS aliases, then the aliases will start pointing to the new NLB that is enhanced with a security group.\n",
				"\n",
				"For more on NLB security groups, please see https://docs.aws.amazon.com/elasticloadbalancing/latest/network/load-balancer-security-groups.html.\n")
		}
	}
	if o.noDeploy || o.detach {
		return nil
	}
	var recommendations []string
	uriRecs, err := o.uriRecommendedActions()
	if err != nil {
		return err
	}
	recommendations = append(recommendations, uriRecs...)
	recommendations = append(recommendations, o.deployRecs.RecommendedActions()...)
	recommendations = append(recommendations, o.publishRecommendedActions()...)
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

func (o *deploySvcOpts) validateOrAskSvcName() error {
	if o.name != "" {
		if err := o.validateSvcName(); err != nil {
			return err
		}
	} else {
		name, err := o.sel.Service("Select a service in your workspace", "")
		if err != nil {
			return fmt.Errorf("select service: %w", err)
		}
		o.name = name
	}
	svc, err := o.store.GetService(o.appName, o.name)
	if err != nil {
		return fmt.Errorf("get service %s configuration: %w", o.name, err)
	}
	o.svcType = svc.Type
	return nil
}

func (o *deploySvcOpts) validateOrAskEnvName() error {
	if o.envName != "" {
		return o.validateEnvName()
	}

	name, err := o.sel.Environment("Select an environment", "", o.appName)
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.envName = name
	return nil
}

func (o *deploySvcOpts) configureClients() error {
	o.gitShortCommit = imageTagFromGit(o.cmd) // Best effort assign git tag.
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return fmt.Errorf("get environment %s configuration: %w", o.envName, err)
	}
	o.targetEnv = env

	// client to retrieve an application's resources created with CloudFormation.
	defaultSess, err := o.sessProvider.Default()
	if err != nil {
		return fmt.Errorf("create default session: %w", err)
	}
	envSess, err := o.sessProvider.FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return err
	}
	o.envSess = envSess

	// client to retrieve caller identity.
	caller, err := identity.New(defaultSess).Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}
	o.rootUserARN = caller.RootUserARN

	envDescriber, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
		App:         o.appName,
		Env:         o.envName,
		ConfigStore: o.store,
	})
	if err != nil {
		return err
	}
	o.envFeaturesDescriber = envDescriber

	wkldDescriber, err := describe.NewWorkloadStackDescriber(describe.NewWorkloadConfig{
		App:         o.appName,
		Env:         o.envName,
		Name:        o.name,
		ConfigStore: o.store,
	})
	if err != nil {
		return err
	}
	o.svcVersionGetter = wkldDescriber
	return nil
}

type workloadManifestInput struct {
	name         string
	appName      string
	envName      string
	ws           wsWlDirReader
	interpolator interpolator
	sess         *session.Session
	unmarshal    func([]byte) (manifest.DynamicWorkload, error)
}

func workloadManifest(in *workloadManifestInput) (manifest.DynamicWorkload, string, error) {
	raw, err := in.ws.ReadWorkloadManifest(in.name)
	if err != nil {
		return nil, "", fmt.Errorf("read manifest file for %s: %w", in.name, err)
	}
	interpolated, err := in.interpolator.Interpolate(string(raw))
	if err != nil {
		return nil, "", fmt.Errorf("interpolate environment variables for %s manifest: %w", in.name, err)
	}
	mft, err := in.unmarshal([]byte(interpolated))
	if err != nil {
		return nil, "", fmt.Errorf("unmarshal service %s manifest: %w", in.name, err)
	}
	envMft, err := mft.ApplyEnv(in.envName)
	if err != nil {
		return nil, "", fmt.Errorf("apply environment %s override: %w", in.envName, err)
	}
	if err := envMft.Validate(); err != nil {
		return nil, "", fmt.Errorf("validate manifest against environment %q: %w", in.envName, err)
	}
	if err := envMft.Load(in.sess); err != nil {
		return nil, "", fmt.Errorf("load dynamic content: %w", err)
	}
	return envMft, interpolated, nil
}

func validateWorkloadManifestCompatibilityWithEnv(ws wsEnvironmentsLister, env versionCompatibilityChecker, mft manifest.DynamicWorkload, envName string) error {
	currVersion, err := env.Version()
	if err != nil {
		return fmt.Errorf("get environment %q version: %w", envName, err)
	}
	if currVersion == version.EnvTemplateBootstrap {
		return fmt.Errorf(`cannot deploy a service to an undeployed environment. Please run "copilot env deploy --name %s" to deploy the environment first`, envName)
	}
	availableFeatures, err := env.AvailableFeatures()
	if err != nil {
		return fmt.Errorf("get available features of the %s environment stack: %w", envName, err)
	}
	exists := struct{}{}
	available := make(map[string]struct{})
	for _, f := range availableFeatures {
		available[f] = exists
	}

	features := mft.RequiredEnvironmentFeatures()
	for _, f := range features {
		if _, ok := available[f]; !ok {
			logMsg := fmt.Sprintf(`Your manifest configuration requires your environment %q to have the feature %q available.`, envName, template.FriendlyEnvFeatureName(f))
			if v := template.LeastVersionForFeature(f); v != "" {
				logMsg += fmt.Sprintf(` The least environment version that supports the feature is %s.`, v)
			}
			logMsg += fmt.Sprintf(" Your environment is on %s.", currVersion)
			log.Errorln(logMsg)
			return &errFeatureIncompatibleWithEnvironment{
				ws:             ws,
				missingFeature: f,
				envName:        envName,
				curVersion:     currVersion,
			}
		}
	}
	return nil
}

func validateWkldVersion(vg versionGetter, name, templateVersion string) error {
	svcVersion, err := vg.Version()
	if err != nil {
		var errStackNotExist *cloudformation.ErrStackNotFound
		if errors.As(err, &errStackNotExist) {
			return nil
		}
		return fmt.Errorf("get template version of workload %s: %w", name, err)
	}
	if diff := semver.Compare(svcVersion, templateVersion); diff > 0 {
		return &errCannotDowngradeWkldVersion{
			name:            name,
			version:         svcVersion,
			templateVersion: templateVersion,
		}
	}
	return nil
}

func (o *deploySvcOpts) uriRecommendedActions() ([]string, error) {
	describer, err := describe.NewReachableService(o.appName, o.name, o.store)
	if err != nil {
		var errNotAccessible *describe.ErrNonAccessibleServiceType
		if errors.As(err, &errNotAccessible) {
			return nil, nil
		}
		return nil, err
	}
	uri, err := describer.URI(o.envName)
	if err != nil {
		return nil, fmt.Errorf("get uri for environment %s: %w", o.envName, err)
	}

	network := "over the internet."
	switch uri.AccessType {
	case describe.URIAccessTypeInternal:
		network = "from your internal network."
	case describe.URIAccessTypeServiceDiscovery:
		network = "with service discovery."
	case describe.URIAccessTypeServiceConnect:
		network = "with service connect."
	case describe.URIAccessTypeNone:
		return []string{}, nil
	}

	return []string{
		fmt.Sprintf("Your service is accessible at %s %s", uri.URI, network),
	}, nil
}

func (o *deploySvcOpts) publishRecommendedActions() []string {
	type publisher interface {
		Publish() []manifest.Topic
	}
	mft, ok := o.appliedDynamicMft.Manifest().(publisher)
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

func (o *deploySvcOpts) getTargetApp() (*config.Application, error) {
	if o.targetApp != nil {
		return o.targetApp, nil
	}
	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		return nil, fmt.Errorf("get application %s configuration: %w", o.appName, err)
	}
	o.targetApp = app
	return o.targetApp, nil
}

type errFeatureIncompatibleWithEnvironment struct {
	ws             wsEnvironmentsLister
	missingFeature string
	envName        string
	curVersion     string
}

func (e *errFeatureIncompatibleWithEnvironment) Error() string {
	if e.curVersion == "" {
		return fmt.Sprintf("environment %q is not on a version that supports the %q feature", e.envName, template.FriendlyEnvFeatureName(e.missingFeature))
	}
	return fmt.Sprintf("environment %q is on version %q which does not support the %q feature", e.envName, e.curVersion, template.FriendlyEnvFeatureName(e.missingFeature))
}

// RecommendActions returns recommended actions to be taken after the error.
// Implements main.actionRecommender interface.
func (e *errFeatureIncompatibleWithEnvironment) RecommendActions() string {
	envs, _ := e.ws.ListEnvironments() // Best effort try to detect if env manifest exists.
	for _, env := range envs {
		if e.envName == env {
			return fmt.Sprintf("You can upgrade the %q environment template by running %s.", e.envName, color.HighlightCode(fmt.Sprintf("copilot env deploy --name %s", e.envName)))
		}
	}
	msgs := []string{
		"You can upgrade your environment template by running:",
		fmt.Sprintf("1. Create the directory to store your environment manifest %s.",
			color.HighlightCode(fmt.Sprintf("mkdir -p %s", filepath.Join("copilot", "environments", e.envName)))),
		fmt.Sprintf("2. Generate the manifest %s.",
			color.HighlightCode(fmt.Sprintf("copilot env show -n %s --manifest > %s", e.envName, filepath.Join("copilot", "environments", e.envName, "manifest.yml")))),
		fmt.Sprintf("3. Deploy the environment stack %s.",
			color.HighlightCode(fmt.Sprintf("copilot env deploy --name %s", e.envName))),
	}
	return strings.Join(msgs, "\n")

}

type errHasDiff struct{}

func (e *errHasDiff) Error() string {
	return ""
}

// ExitCode returns 1 for a non-empty diff.
func (e *errHasDiff) ExitCode() int {
	return 1
}

func diff(differ templateDiffer, tmpl string, writer io.Writer) error {
	if out, err := differ.DeployDiff(tmpl); err != nil {
		return err
	} else if out != "" {
		if _, err := writer.Write([]byte(out)); err != nil {
			return err
		}
		return &errHasDiff{}
	}
	if _, err := writer.Write([]byte("No changes.\n")); err != nil {
		return err
	}
	return nil
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
	cmd.Flags().BoolVar(&vars.disableRollback, noRollbackFlag, false, noRollbackFlagDescription)
	cmd.Flags().BoolVar(&vars.showDiff, diffFlag, false, diffFlagDescription)
	cmd.Flags().BoolVar(&vars.skipDiffPrompt, diffAutoApproveFlag, false, diffAutoApproveFlagDescription)
	cmd.Flags().BoolVar(&vars.allowWkldDowngrade, allowDowngradeFlag, false, allowDowngradeFlagDescription)
	cmd.Flags().BoolVar(&vars.detach, detachFlag, false, detachFlagDescription)
	return cmd
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/profile"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/version"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/dustin/go-humanize/english"
	"github.com/spf13/afero"
	"golang.org/x/mod/semver"

	"github.com/aws/aws-sdk-go/service/ssm"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/iam"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/partitions"
	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	deploycfn "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	envInitNamePrompt              = "What is your environment's name?"
	envInitNameHelpPrompt          = "A unique identifier for an environment (e.g. dev, test, prod)."
	envInitDefaultEnvConfirmPrompt = `Would you like to use the default configuration for a new environment?
    - A new VPC with 2 AZs, 2 public subnets and 2 private subnets
    - A new ECS Cluster
    - New IAM Roles to manage services and jobs in your environment
`
	envInitVPCSelectPrompt            = "Which VPC would you like to use?"
	envInitPublicSubnetsSelectPrompt  = "Which public subnets would you like to use?\nYou may choose to press 'Enter' to skip this step if the services and/or jobs you'll deploy to this environment are not internet-facing."
	envInitPrivateSubnetsSelectPrompt = "Which private subnets would you like to use?"

	envInitVPCCIDRPrompt         = "What VPC CIDR would you like to use?"
	envInitVPCCIDRPromptHelp     = "CIDR used for your VPC. For example: 10.1.0.0/16"
	envInitAdjustAZPrompt        = "Which availability zones would you like to use?"
	envInitAdjustAZPromptHelp    = "Availability zone names that span your resources. For example: us-east-1a,us-east1b,us-east-1c"
	envInitPublicCIDRPrompt      = "What CIDR would you like to use for your public subnets?"
	envInitPublicCIDRPromptHelp  = "CIDRs used for your public subnets. For example: 10.1.0.0/24,10.1.1.0/24"
	envInitPrivateCIDRPrompt     = "What CIDR would you like to use for your private subnets?"
	envInitPrivateCIDRPromptHelp = "CIDRs used for your private subnets. For example: 10.1.2.0/24,10.1.3.0/24"

	fmtEnvInitCredsPrompt  = "Which credentials would you like to use to create %s?"
	envInitCredsHelpPrompt = `The credentials are used to create your environment in an AWS account and region.
To learn more:
https://aws.github.io/copilot-cli/docs/credentials/#environment-credentials`
	envInitRegionPrompt        = "Which region?"
	envInitDefaultRegionOption = "us-west-2"

	fmtDNSDelegationStart    = "Sharing DNS permissions for this application to account %s."
	fmtDNSDelegationFailed   = "Failed to grant DNS permissions to account %s.\n\n"
	fmtDNSDelegationComplete = "Shared DNS permissions for this application to account %s.\n\n"
)

var (
	envInitDefaultConfigSelectOption      = "Yes, use default."
	envInitAdjustEnvResourcesSelectOption = "Yes, but I'd like configure the default resources (CIDR ranges, AZs)."
	envInitImportEnvResourcesSelectOption = "No, I'd like to import existing resources (VPC, subnets)."
	envInitCustomizedEnvTypes             = []string{envInitDefaultConfigSelectOption, envInitAdjustEnvResourcesSelectOption, envInitImportEnvResourcesSelectOption}
)

type importVPCVars struct {
	ID               string
	PublicSubnetIDs  []string
	PrivateSubnetIDs []string
}

func (v importVPCVars) isSet() bool {
	if v.ID != "" {
		return true
	}
	return len(v.PublicSubnetIDs) > 0 || len(v.PrivateSubnetIDs) > 0
}

type adjustVPCVars struct {
	CIDR               net.IPNet
	AZs                []string
	PublicSubnetCIDRs  []string
	PrivateSubnetCIDRs []string
}

func (v adjustVPCVars) isSet() bool {
	if v.CIDR.String() != emptyIPNet.String() {
		return true
	}
	for _, arr := range [][]string{v.AZs, v.PublicSubnetCIDRs, v.PrivateSubnetCIDRs} {
		if len(arr) != 0 {
			return true
		}
	}
	return false
}

type tempCredsVars struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

func (v tempCredsVars) isSet() bool {
	return v.AccessKeyID != "" && v.SecretAccessKey != ""
}

type telemetryVars struct {
	EnableContainerInsights bool
}

func (v telemetryVars) toConfig() *config.Telemetry {
	return &config.Telemetry{
		EnableContainerInsights: v.EnableContainerInsights,
	}
}

type initEnvVars struct {
	appName           string
	name              string // Name for the environment.
	profile           string // The named profile to use for credential retrieval. Mutually exclusive with tempCreds.
	isProduction      bool   // True means retain resources even after deletion.
	defaultConfig     bool   // True means using default environment configuration.
	allowAppDowngrade bool

	importVPC          importVPCVars // Existing VPC resources to use instead of creating new ones.
	adjustVPC          adjustVPCVars // Configure parameters for VPC resources generated while initializing an environment.
	telemetry          telemetryVars // Configure observability and monitoring settings.
	importCerts        []string      // Additional existing ACM certificates to use.
	internalALBSubnets []string      // Subnets to be used for internal ALB placement.
	allowVPCIngress    bool          // True means the env stack will create ingress to the internal ALB from ports 80/443.

	tempCreds tempCredsVars // Temporary credentials to initialize the environment. Mutually exclusive with the profile.
	region    string        // The region to create the environment in.
}

type initEnvOpts struct {
	initEnvVars

	// Interfaces to interact with dependencies.
	sessProvider        sessionProvider
	store               store
	envDeployer         deployer
	appDeployer         deployer
	identity            identityService
	envIdentity         identityService
	ec2Client           ec2Client
	newAppVersionGetter func(appName string) (versionGetter, error)
	iam                 roleManager
	cfn                 stackExistChecker
	prog                progress
	prompt              prompter
	selVPC              ec2Selector
	selCreds            func() (credsSelector, error)
	selApp              appSelector
	appCFN              appResourcesGetter
	manifestWriter      environmentManifestWriter
	envLister           wsEnvironmentsLister

	sess *session.Session // Session pointing to environment's AWS account and region.

	// Cached variables.
	wsAppName        string
	mftDisplayedPath string

	// Overridden in tests.
	templateVersion string
}

func newInitEnvOpts(vars initEnvVars) (*initEnvOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("env init"))
	defaultSession, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}
	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	prompter := prompt.New()
	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}
	return &initEnvOpts{
		initEnvVars:  vars,
		sessProvider: sessProvider,
		store:        store,
		appDeployer:  deploycfn.New(defaultSession, deploycfn.WithProgressTracker(os.Stderr)),
		identity:     identity.New(defaultSession),
		prog:         termprogress.NewSpinner(log.DiagnosticWriter),
		prompt:       prompter,
		selCreds: func() (credsSelector, error) {
			cfg, err := profile.NewConfig()
			if err != nil {
				return nil, fmt.Errorf("read named profiles: %w", err)
			}
			return &selector.CredsSelect{
				Session: sessProvider,
				Profile: cfg,
				Prompt:  prompt.New(),
			}, nil
		},
		newAppVersionGetter: func(appName string) (versionGetter, error) {
			return describe.NewAppDescriber(appName)
		},
		selApp:         selector.NewAppEnvSelector(prompt.New(), store),
		appCFN:         deploycfn.New(defaultSession, deploycfn.WithProgressTracker(os.Stderr)),
		manifestWriter: ws,
		envLister:      ws,

		wsAppName:       tryReadingAppName(),
		templateVersion: version.LatestTemplateVersion(),
	}, nil
}

// Validate returns an error if the values passed by flags are invalid.
func (o *initEnvOpts) Validate() error {
	if err := validateWorkspaceApp(o.wsAppName, o.appName, o.store); err != nil {
		return err
	}
	o.appName = o.wsAppName

	if o.name != "" {
		if err := validateEnvironmentName(o.name); err != nil {
			return err
		}
		if err := o.validateDuplicateEnv(); err != nil {
			return err
		}
	}

	if err := o.validateCustomizedResources(); err != nil {
		return err
	}
	return o.validateCredentials()
}

// Ask asks for fields that are required but not passed in.
func (o *initEnvOpts) Ask() error {
	if err := o.askEnvName(); err != nil {
		return err
	}
	if err := o.askEnvSession(); err != nil {
		return err
	}
	if err := o.askEnvRegion(); err != nil {
		return err
	}
	return o.askCustomizedResources()
}

// Execute deploys a new environment with CloudFormation and adds it to SSM.
func (o *initEnvOpts) Execute() error {
	if err := o.initRuntimeClients(); err != nil {
		return err
	}
	if !o.allowAppDowngrade {
		versionGetter, err := o.newAppVersionGetter(o.appName)
		if err != nil {
			return err
		}
		if err := validateAppVersion(versionGetter, o.appName, o.templateVersion); err != nil {
			return err
		}
	}
	app, err := o.store.GetApplication(o.appName)
	if err != nil {
		// Ensure the app actually exists before we write the manifest.
		return err
	}
	envCaller, err := o.envIdentity.Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}

	// 1. Write environment manifest.
	path, err := o.writeManifest()
	if err != nil {
		return err
	}
	o.mftDisplayedPath = path

	// 2. Perform DNS delegation from app to env.
	if app.Domain != "" {
		if err := o.delegateDNSFromApp(app, envCaller.Account); err != nil {
			return fmt.Errorf("granting DNS permissions: %w", err)
		}
	}

	// 3. Attempt to create the service linked role if it doesn't exist.
	// If the call fails because the role already exists, nothing to do.
	// If the call fails because the user doesn't have permissions, then the role must be created outside of Copilot.
	_ = o.iam.CreateECSServiceLinkedRole()

	// 4. Add the stack set instance to the app stackset.
	if err := o.addToStackset(&deploycfn.AddEnvToAppOpts{
		App:          app,
		EnvName:      o.name,
		EnvRegion:    aws.StringValue(o.sess.Config.Region),
		EnvAccountID: envCaller.Account,
	}); err != nil {
		return err
	}

	// 5. Start creating the CloudFormation stack for the environment.
	if err := o.deployEnv(app); err != nil {
		return err
	}

	// 6. Store the environment in SSM with information about the deployed bootstrap roles.
	env, err := o.envDeployer.GetEnvironment(o.appName, o.name)
	if err != nil {
		return fmt.Errorf("get environment struct for %s: %w", o.name, err)
	}
	if err := o.store.CreateEnvironment(env); err != nil {
		return fmt.Errorf("store environment: %w", err)
	}
	log.Successf("Provisioned bootstrap resources for environment %s in region %s under application %s.\n",
		color.HighlightUserInput(env.Name), color.Emphasize(env.Region), color.HighlightUserInput(env.App))
	return nil
}

// RecommendActions returns follow-up actions the user can take after successfully executing the command.
func (o *initEnvOpts) RecommendActions() error {
	logRecommendedActions([]string{
		fmt.Sprintf("Update your manifest %s to change the defaults.", color.HighlightResource(o.mftDisplayedPath)),
		fmt.Sprintf("Run %s to deploy your environment.",
			color.HighlightCode(fmt.Sprintf("copilot env deploy --name %s", o.name))),
	})
	return nil
}

func (o *initEnvOpts) initRuntimeClients() error {
	// Initialize environment clients if not set.
	if o.envIdentity == nil {
		o.envIdentity = identity.New(o.sess)
	}
	if o.envDeployer == nil {
		o.envDeployer = deploycfn.New(o.sess, deploycfn.WithProgressTracker(os.Stderr))
	}
	if o.cfn == nil {
		o.cfn = cloudformation.New(o.sess)
	}
	if o.iam == nil {
		o.iam = iam.New(o.sess)
	}
	return nil
}

func (o *initEnvOpts) validateCustomizedResources() error {
	if o.importVPC.isSet() && o.adjustVPC.isSet() {
		return errors.New("cannot specify both import vpc flags and configure vpc flags")
	}
	if (o.importVPC.isSet() || o.adjustVPC.isSet()) && o.defaultConfig {
		return fmt.Errorf("cannot import or configure vpc if --%s is set", defaultConfigFlag)
	}
	if o.internalALBSubnets != nil && (o.adjustVPC.isSet() || o.defaultConfig) {
		log.Error(`To specify internal ALB subnet placement, you must import existing resources, including subnets.
For default config without subnet placement specification, Copilot will place the internal ALB in the generated private subnets.`)
		return fmt.Errorf("subnets '%s' specified for internal ALB placement, but those subnets are not imported", strings.Join(o.internalALBSubnets, ", "))
	}
	if o.importVPC.isSet() {
		// Allow passing in VPC without subnets, but error out early for too few subnets-- we won't prompt the user to select more of one type if they pass in any.
		if len(o.importVPC.PublicSubnetIDs) == 1 {
			return errors.New("at least two public subnets must be imported to enable Load Balancing")
		}
		if len(o.importVPC.PrivateSubnetIDs) == 1 {
			return fmt.Errorf("at least two private subnets must be imported")
		}
		if err := o.validateInternalALBSubnets(); err != nil {
			return err
		}
	}
	if o.adjustVPC.isSet() {
		if len(o.adjustVPC.AZs) == 1 {
			return errors.New("at least two availability zones must be provided to enable Load Balancing")
		}
	}
	return nil
}

func (o *initEnvOpts) askEnvName() error {
	if o.name != "" {
		return nil
	}

	envName, err := o.prompt.Get(envInitNamePrompt, envInitNameHelpPrompt, validateEnvironmentName, prompt.WithFinalMessage("Environment name:"))
	if err != nil {
		return fmt.Errorf("get environment name: %w", err)
	}
	o.name = envName
	return o.validateDuplicateEnv()
}

func (o *initEnvOpts) askEnvSession() error {
	if o.profile != "" {
		sess, err := o.sessProvider.FromProfile(o.profile)
		if err != nil {
			return fmt.Errorf("create session from profile %s: %w", o.profile, err)
		}
		o.sess = sess
		return nil
	}
	if o.tempCreds.isSet() {
		sess, err := o.sessProvider.FromStaticCreds(o.tempCreds.AccessKeyID, o.tempCreds.SecretAccessKey, o.tempCreds.SessionToken)
		if err != nil {
			return err
		}
		o.sess = sess
		return nil
	}

	selCreds, err := o.selCreds()
	if err != nil {
		errRetrieveCreds := err
		sess, err := o.sessProvider.Default()
		if err != nil {
			return errors.Join(errRetrieveCreds, fmt.Errorf("falling back on default credentials: %w", err))
		}
		o.sess = sess
		return nil
	}

	sess, err := selCreds.Creds(fmt.Sprintf(fmtEnvInitCredsPrompt, color.HighlightUserInput(o.name)), envInitCredsHelpPrompt)
	if err != nil {
		return fmt.Errorf("select creds: %w", err)
	}
	o.sess = sess
	return nil
}

func (o *initEnvOpts) askEnvRegion() error {
	region := aws.StringValue(o.sess.Config.Region)
	if o.region != "" {
		region = o.region
	}
	if region == "" {
		v, err := o.prompt.Get(envInitRegionPrompt, "", nil, prompt.WithDefaultInput(envInitDefaultRegionOption), prompt.WithFinalMessage("Region:"))
		if err != nil {
			return fmt.Errorf("get environment region: %w", err)
		}
		region = v
	}
	o.sess.Config.Region = aws.String(region)
	return nil
}

func (o *initEnvOpts) askCustomizedResources() error {
	if o.defaultConfig {
		return nil
	}
	if o.importVPC.isSet() {
		return o.askImportResources()
	}
	if o.adjustVPC.isSet() {
		return o.askAdjustResources()
	}
	if o.internalALBSubnets != nil {
		log.Infoln("Because you have designated subnets on which to place an internal ALB, you must import VPC resources.")
		return o.askImportResources()
	}
	adjustOrImport, err := o.prompt.SelectOne(
		envInitDefaultEnvConfirmPrompt, "",
		envInitCustomizedEnvTypes,
		prompt.WithFinalMessage("Default environment configuration?"))
	if err != nil {
		return fmt.Errorf("select adjusting or importing resources: %w", err)
	}
	switch adjustOrImport {
	case envInitImportEnvResourcesSelectOption:
		return o.askImportResources()
	case envInitAdjustEnvResourcesSelectOption:
		return o.askAdjustResources()
	case envInitDefaultConfigSelectOption:
		return nil
	}
	return nil
}

func (o *initEnvOpts) askImportResources() error {
	if o.selVPC == nil {
		o.selVPC = selector.NewEC2Select(o.prompt, ec2.New(o.sess))
	}
	if o.importVPC.ID == "" {
		vpcID, err := o.selVPC.VPC(envInitVPCSelectPrompt, "")
		if err != nil {
			if err == selector.ErrVPCNotFound {
				log.Errorf(`No existing VPCs were found. You can either:
- Create a new VPC first and then import it.
- Use the default Copilot environment configuration.
`)
			}
			return fmt.Errorf("select VPC: %w", err)
		}
		o.importVPC.ID = vpcID
	}
	if o.ec2Client == nil {
		o.ec2Client = ec2.New(o.sess)
	}
	dnsSupport, err := o.ec2Client.HasDNSSupport(o.importVPC.ID)
	if err != nil {
		return fmt.Errorf("check if VPC %s has DNS support enabled: %w", o.importVPC.ID, err)
	}
	if !dnsSupport {
		log.Errorln(`Looks like you're creating an environment using a VPC with DNS support *disabled*.
Copilot cannot create services or jobs in VPCs without DNS support. We recommend enabling this property.
To learn more about the issue:
https://aws.amazon.com/premiumsupport/knowledge-center/ecs-pull-container-api-error-ecr/`)
		return fmt.Errorf("VPC %s has no DNS support enabled", o.importVPC.ID)
	}
	if o.importVPC.PublicSubnetIDs == nil {
		publicSubnets, err := o.selVPC.Subnets(selector.SubnetsInput{
			Msg:      envInitPublicSubnetsSelectPrompt,
			Help:     "",
			VPCID:    o.importVPC.ID,
			IsPublic: true,
		})
		if err != nil {
			if errors.Is(err, selector.ErrSubnetsNotFound) {
				log.Warningf(`No existing public subnets were found in VPC %s.
`, o.importVPC.ID)
			} else {
				return fmt.Errorf("select public subnets: %w", err)
			}
		}
		if len(publicSubnets) == 1 {
			return errors.New("select public subnets: at least two public subnets must be selected to enable Load Balancing")
		}
		if len(publicSubnets) == 0 {
			log.Warningf(`If you proceed without public subnets, you will not be able to deploy 
Load Balanced Web Services in this environment, and will need to specify 'private' 
network placement in your workload manifest(s). See the manifest documentation 
specific to your workload type(s) (https://aws.github.io/copilot-cli/docs/manifest/overview/).
`)
		}
		o.importVPC.PublicSubnetIDs = publicSubnets
	}
	if o.importVPC.PrivateSubnetIDs == nil {
		privateSubnets, err := o.selVPC.Subnets(selector.SubnetsInput{
			Msg:      envInitPrivateSubnetsSelectPrompt,
			Help:     "",
			VPCID:    o.importVPC.ID,
			IsPublic: false,
		})
		if err != nil {
			if errors.Is(err, selector.ErrSubnetsNotFound) {
				log.Warningf(`No existing private subnets were found in VPC %s. 
`, o.importVPC.ID)
			} else {
				return fmt.Errorf("select private subnets: %w", err)
			}
		}
		if len(privateSubnets) == 1 {
			return errors.New("select private subnets: at least two private subnets must be selected")
		}
		if len(privateSubnets) == 0 {
			log.Warningf(`If you proceed without private subnets, you will not 
be able to add them after this environment is created.
`)
		}
		o.importVPC.PrivateSubnetIDs = privateSubnets
	}
	if len(o.importVPC.PublicSubnetIDs)+len(o.importVPC.PrivateSubnetIDs) == 0 {
		return errors.New("VPC must have subnets in order to proceed with environment creation")
	}
	return o.validateInternalALBSubnets()
}

func (o *initEnvOpts) askAdjustResources() error {
	if o.adjustVPC.CIDR.String() == emptyIPNet.String() {
		vpcCIDRString, err := o.prompt.Get(envInitVPCCIDRPrompt, envInitVPCCIDRPromptHelp, validateCIDR,
			prompt.WithDefaultInput(stack.DefaultVPCCIDR), prompt.WithFinalMessage("VPC CIDR:"))
		if err != nil {
			return fmt.Errorf("get VPC CIDR: %w", err)
		}
		_, vpcCIDR, err := net.ParseCIDR(vpcCIDRString)
		if err != nil {
			return fmt.Errorf("parse VPC CIDR: %w", err)
		}
		o.adjustVPC.CIDR = *vpcCIDR
	}
	azs, err := o.askAZs()
	if err != nil {
		return err
	}
	o.adjustVPC.AZs = azs
	if o.adjustVPC.PublicSubnetCIDRs == nil {
		publicCIDR, err := o.prompt.Get(
			envInitPublicCIDRPrompt, envInitPublicCIDRPromptHelp,
			validatePublicSubnetsCIDR(len(o.adjustVPC.AZs)),
			prompt.WithDefaultInput(strings.Join(stack.DefaultPublicSubnetCIDRs, ",")), prompt.WithFinalMessage("Public subnets CIDR:"))
		if err != nil {
			return fmt.Errorf("get public subnet CIDRs: %w", err)
		}
		o.adjustVPC.PublicSubnetCIDRs = strings.Split(publicCIDR, ",")
	}
	if o.adjustVPC.PrivateSubnetCIDRs == nil {
		privateCIDR, err := o.prompt.Get(
			envInitPrivateCIDRPrompt, envInitPrivateCIDRPromptHelp,
			validatePrivateSubnetsCIDR(len(o.adjustVPC.AZs)),
			prompt.WithDefaultInput(strings.Join(stack.DefaultPrivateSubnetCIDRs, ",")), prompt.WithFinalMessage("Private subnets CIDR:"))
		if err != nil {
			return fmt.Errorf("get private subnet CIDRs: %w", err)
		}
		o.adjustVPC.PrivateSubnetCIDRs = strings.Split(privateCIDR, ",")
	}
	return nil
}

func (o *initEnvOpts) askAZs() ([]string, error) {
	if o.adjustVPC.AZs != nil {
		return o.adjustVPC.AZs, nil
	}
	if o.ec2Client == nil {
		o.ec2Client = ec2.New(o.sess)
	}
	azs, err := o.ec2Client.ListAZs()
	if err != nil {
		return nil, fmt.Errorf("list availability zones for region %s: %v", aws.StringValue(o.sess.Config.Region), err)
	}

	var options []string
	for _, az := range azs {
		options = append(options, az.Name)
	}
	const minAZs = 2
	if len(options) < minAZs {
		return nil, fmt.Errorf("requires at least %d availability zones (%s) in region %s", minAZs, strings.Join(options, ", "), aws.StringValue(o.sess.Config.Region))
	}
	defaultOptions := make([]string, minAZs)
	for i := 0; i < minAZs; i += 1 {
		defaultOptions[i] = azs[i].Name
	}
	selected, err := o.prompt.MultiSelect(
		envInitAdjustAZPrompt, envInitAdjustAZPromptHelp, options,
		prompt.RequireMinItems(minAZs),
		prompt.WithDefaultSelections(defaultOptions), prompt.WithFinalMessage("AZs:"))
	if err != nil {
		return nil, fmt.Errorf("select availability zones: %v", err)
	}
	return selected, nil
}

func (o *initEnvOpts) validateDuplicateEnv() error {
	_, err := o.store.GetEnvironment(o.appName, o.name)
	if err == nil {
		// Skip error if environment already exists in workspace
		envs, err := o.envLister.ListEnvironments()
		if err != nil {
			return err
		}
		if slices.Contains(envs, o.name) {
			return nil
		}

		dir := filepath.Join("copilot", "environments", o.name)
		log.Infof(`It seems like you are trying to init an environment that already exists.
To generate a manifest for the environment:
1. %s
2. %s

Alternatively, to recreate the environment:
1. %s
2. And then %s
`,
			color.HighlightCode(fmt.Sprintf("mkdir -p %s", dir)),
			color.HighlightCode(fmt.Sprintf("copilot env show -n %s --manifest > %s", o.name, filepath.Join(dir, "manifest.yml"))),
			color.HighlightCode(fmt.Sprintf("copilot env delete --name %s", o.name)),
			color.HighlightCode(fmt.Sprintf("copilot env init --name %s", o.name)))
		return fmt.Errorf("environment %s already exists", color.HighlightUserInput(o.name))
	}

	var errNoSuchEnvironment *config.ErrNoSuchEnvironment
	if !errors.As(err, &errNoSuchEnvironment) {
		return fmt.Errorf("validate if environment exists: %w", err)
	}
	return nil
}

func (o *initEnvOpts) importVPCConfig() *config.ImportVPC {
	if o.defaultConfig || !o.importVPC.isSet() {
		return nil
	}
	return &config.ImportVPC{
		ID:               o.importVPC.ID,
		PrivateSubnetIDs: o.importVPC.PrivateSubnetIDs,
		PublicSubnetIDs:  o.importVPC.PublicSubnetIDs,
	}
}

func (o *initEnvOpts) adjustVPCConfig() *config.AdjustVPC {
	if o.defaultConfig || !o.adjustVPC.isSet() {
		return nil
	}
	return &config.AdjustVPC{
		CIDR:               o.adjustVPC.CIDR.String(),
		AZs:                o.adjustVPC.AZs,
		PrivateSubnetCIDRs: o.adjustVPC.PrivateSubnetCIDRs,
		PublicSubnetCIDRs:  o.adjustVPC.PublicSubnetCIDRs,
	}
}

func (o *initEnvOpts) deployEnv(app *config.Application) error {
	envRegion := aws.StringValue(o.sess.Config.Region)
	resources, err := o.appCFN.GetAppResourcesByRegion(app, envRegion)
	if err != nil {
		return fmt.Errorf("get app resources: %w", err)
	}
	if resources.S3Bucket == "" {
		log.Errorf("Cannot find the S3 artifact bucket in %s region created by app %s. The S3 bucket is necessary for many future operations. For example, when you need addons to your services.", envRegion, app.Name)
		return fmt.Errorf("cannot find the S3 artifact bucket in %s region", envRegion)
	}
	partition, err := partitions.Region(envRegion).Partition()
	if err != nil {
		return err
	}
	artifactBucketARN := s3.FormatARN(partition.ID(), resources.S3Bucket)

	caller, err := o.identity.Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}
	deployEnvInput := &stack.EnvConfig{
		Name: o.name,
		App: deploy.AppInformation{
			Name:                o.appName,
			Domain:              app.Domain,
			AccountPrincipalARN: caller.RootUserARN,
		},
		AdditionalTags:       app.Tags,
		ArtifactBucketARN:    artifactBucketARN,
		ArtifactBucketKeyARN: resources.KMSKeyARN,
		PermissionsBoundary:  app.PermissionsBoundary,
	}

	if err := o.cleanUpDanglingRoles(o.appName, o.name); err != nil {
		return err
	}
	if err := o.envDeployer.CreateAndRenderEnvironment(stack.NewBootstrapEnvStackConfig(deployEnvInput), artifactBucketARN); err != nil {
		var existsErr *cloudformation.ErrStackAlreadyExists
		if errors.As(err, &existsErr) {
			// Do nothing if the stack already exists.
			return nil
		}
		// The stack failed to create due to an unexpect reason.
		// Delete the retained roles created part of the stack.
		o.tryDeletingEnvRoles(o.appName, o.name)
		return err
	}
	return nil
}

func (o *initEnvOpts) addToStackset(opts *deploycfn.AddEnvToAppOpts) error {
	if err := o.appDeployer.AddEnvToApp(opts); err != nil {
		return fmt.Errorf("add env %s to application %s: %w", opts.EnvName, opts.App.Name, err)
	}
	return nil
}

func (o *initEnvOpts) delegateDNSFromApp(app *config.Application, accountID string) error {
	// By default, our DNS Delegation permits same account delegation.
	if accountID == app.AccountID {
		return nil
	}

	o.prog.Start(fmt.Sprintf(fmtDNSDelegationStart, color.HighlightUserInput(accountID)))
	if err := o.appDeployer.DelegateDNSPermissions(app, accountID); err != nil {
		o.prog.Stop(log.Serrorf(fmtDNSDelegationFailed, color.HighlightUserInput(accountID)))
		return err
	}
	o.prog.Stop(log.Ssuccessf(fmtDNSDelegationComplete, color.HighlightUserInput(accountID)))
	return nil
}

func (o *initEnvOpts) validateCredentials() error {
	if o.profile != "" && o.tempCreds.AccessKeyID != "" {
		return fmt.Errorf("cannot specify both --%s and --%s", profileFlag, accessKeyIDFlag)
	}
	if o.profile != "" && o.tempCreds.SecretAccessKey != "" {
		return fmt.Errorf("cannot specify both --%s and --%s", profileFlag, secretAccessKeyFlag)
	}
	if o.profile != "" && o.tempCreds.SessionToken != "" {
		return fmt.Errorf("cannot specify both --%s and --%s", profileFlag, sessionTokenFlag)
	}
	return nil
}

func (o *initEnvOpts) validateInternalALBSubnets() error {
	if len(o.internalALBSubnets) == 0 {
		return nil
	}
	isImported := make(map[string]bool)
	for _, placementSubnet := range o.internalALBSubnets {
		for _, subnet := range append(o.importVPC.PrivateSubnetIDs, o.importVPC.PublicSubnetIDs...) {
			if placementSubnet == subnet {
				isImported[placementSubnet] = true
			}
		}
	}
	if len(isImported) != len(o.internalALBSubnets) {
		return fmt.Errorf("%s '%s' %s designated for ALB placement, but %s imported",
			english.PluralWord(len(o.internalALBSubnets), "subnet", "subnets"),
			strings.Join(o.internalALBSubnets, ", "),
			english.PluralWord(len(o.internalALBSubnets), "was", "were"),
			english.PluralWord(len(o.internalALBSubnets), "it was not", "they were not all"))
	}
	return nil
}

// cleanUpDanglingRoles deletes any IAM roles created for the same app and env that were left over from a previous
// environment creation.
func (o *initEnvOpts) cleanUpDanglingRoles(app, env string) error {
	exists, err := o.cfn.Exists(stack.NameForEnv(app, env))
	if err != nil {
		return fmt.Errorf("check if stack %s exists: %w", stack.NameForEnv(app, env), err)
	}
	if exists {
		return nil
	}
	// There is no environment stack. Either the customer ran "env delete" before, or it's their
	// first time running this command.
	// We should clean up any IAM roles that were *not* deleted during "env delete"
	// before re-creating the stack otherwise the deployment will fail.
	o.tryDeletingEnvRoles(app, env)
	return nil
}

// tryDeletingEnvRoles attempts a best effort deletion of IAM roles created from an environment.
// To ensure that the roles being deleted were created by Copilot, we check if the copilot-environment tag
// is applied to the role.
func (o *initEnvOpts) tryDeletingEnvRoles(app, env string) {
	roleNames := []string{
		fmt.Sprintf("%s-CFNExecutionRole", stack.NameForEnv(app, env)),
		fmt.Sprintf("%s-EnvManagerRole", stack.NameForEnv(app, env)),
	}
	for _, roleName := range roleNames {
		tags, err := o.iam.ListRoleTags(roleName)
		if err != nil {
			continue
		}
		if _, hasTag := tags[deploy.EnvTagKey]; !hasTag {
			continue
		}
		_ = o.iam.DeleteRole(roleName)
	}
}

func (o *initEnvOpts) writeManifest() (string, error) {
	customizedEnv := &config.CustomizeEnv{
		ImportVPC:                   o.importVPCConfig(),
		VPCConfig:                   o.adjustVPCConfig(),
		ImportCertARNs:              o.importCerts,
		InternalALBSubnets:          o.internalALBSubnets,
		EnableInternalALBVPCIngress: o.allowVPCIngress,
	}
	if customizedEnv.IsEmpty() {
		customizedEnv = nil
	}
	props := manifest.EnvironmentProps{
		Name:         o.name,
		CustomConfig: customizedEnv,
		Telemetry:    o.telemetry.toConfig(),
	}

	var manifestExists bool
	manifestPath, err := o.manifestWriter.WriteEnvironmentManifest(manifest.NewEnvironment(&props), props.Name)
	if err != nil {
		e, ok := err.(*workspace.ErrFileExists)
		if !ok {
			return "", fmt.Errorf("write environment manifest: %w", err)
		}
		manifestExists = true
		manifestPath = e.FileName
	}
	manifestPath = displayPath(manifestPath)
	manifestMsgFmt := "Wrote the manifest for environment %s at %s\n"
	if manifestExists {
		manifestMsgFmt = "Manifest file for environment %s already exists at %s, skipping writing it.\n"
	}
	log.Successf(manifestMsgFmt, color.HighlightUserInput(props.Name), color.HighlightResource(manifestPath))
	return manifestPath, nil
}

func validateAppVersion(vg versionGetter, name, templateVersion string) error {
	appVersion, err := vg.Version()
	if err != nil {
		return fmt.Errorf("get template version of application %s: %w", name, err)
	}
	if diff := semver.Compare(appVersion, templateVersion); diff > 0 {
		return &errCannotDowngradeAppVersion{
			appName:         name,
			appVersion:      appVersion,
			templateVersion: templateVersion,
		}
	}
	return nil
}

// buildEnvInitCmd builds the command for adding an environment.
func buildEnvInitCmd() *cobra.Command {
	vars := initEnvVars{}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a new environment in your application.",
		Example: `
  Creates a test environment using your "default" AWS profile and default configuration.
  /code $ copilot env init --name test --profile default --default-config

  Creates a prod-iad environment using your "prod-admin" AWS profile and enables container insights.
  /code $ copilot env init --name prod-iad --profile prod-admin --container-insights

  Creates an environment with imported resources.
  /code $ copilot env init --import-vpc-id vpc-099c32d2b98cdcf47 \
  /code --import-public-subnets subnet-013e8b691862966cf,subnet-014661ebb7ab8681a \
  /code --import-private-subnets subnet-055fafef48fb3c547,subnet-00c9e76f288363e7f \
  /code --import-cert-arns arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012

  Creates an environment with overridden CIDRs and AZs.
  /code $ copilot env init --override-vpc-cidr 10.1.0.0/16 \
  /code --override-az-names us-west-2b,us-west-2c \
  /code --override-public-cidrs 10.1.0.0/24,10.1.1.0/24 \
  /code --override-private-cidrs 10.1.2.0/24,10.1.3.0/24`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newInitEnvOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.profile, profileFlag, "", profileFlagDescription)
	cmd.Flags().StringVar(&vars.tempCreds.AccessKeyID, accessKeyIDFlag, "", accessKeyIDFlagDescription)
	cmd.Flags().StringVar(&vars.tempCreds.SecretAccessKey, secretAccessKeyFlag, "", secretAccessKeyFlagDescription)
	cmd.Flags().StringVar(&vars.tempCreds.SessionToken, sessionTokenFlag, "", sessionTokenFlagDescription)
	cmd.Flags().StringVar(&vars.region, regionFlag, "", envRegionTokenFlagDescription)
	cmd.Flags().BoolVar(&vars.allowAppDowngrade, allowDowngradeFlag, false, allowDowngradeFlagDescription)

	cmd.Flags().BoolVar(&vars.isProduction, prodEnvFlag, false, prodEnvFlagDescription) // Deprecated. Use telemetry flags instead.
	cmd.Flags().BoolVar(&vars.telemetry.EnableContainerInsights, enableContainerInsightsFlag, false, enableContainerInsightsFlagDescription)

	cmd.Flags().StringVar(&vars.importVPC.ID, vpcIDFlag, "", vpcIDFlagDescription)
	cmd.Flags().StringSliceVar(&vars.importVPC.PublicSubnetIDs, publicSubnetsFlag, nil, publicSubnetsFlagDescription)
	cmd.Flags().StringSliceVar(&vars.importVPC.PrivateSubnetIDs, privateSubnetsFlag, nil, privateSubnetsFlagDescription)
	cmd.Flags().StringSliceVar(&vars.importCerts, certsFlag, nil, certsFlagDescription)
	cmd.Flags().IPNetVar(&vars.adjustVPC.CIDR, overrideVPCCIDRFlag, net.IPNet{}, overrideVPCCIDRFlagDescription)
	cmd.Flags().StringSliceVar(&vars.adjustVPC.AZs, overrideAZsFlag, nil, overrideAZsFlagDescription)
	// TODO: use IPNetSliceVar when it is available (https://github.com/spf13/pflag/issues/273).
	cmd.Flags().StringSliceVar(&vars.adjustVPC.PublicSubnetCIDRs, overridePublicSubnetCIDRsFlag, nil, overridePublicSubnetCIDRsFlagDescription)
	cmd.Flags().StringSliceVar(&vars.adjustVPC.PrivateSubnetCIDRs, overridePrivateSubnetCIDRsFlag, nil, overridePrivateSubnetCIDRsFlagDescription)
	cmd.Flags().StringSliceVar(&vars.internalALBSubnets, internalALBSubnetsFlag, nil, internalALBSubnetsFlagDescription)
	cmd.Flags().BoolVar(&vars.allowVPCIngress, allowVPCIngressFlag, false, allowVPCIngressFlagDescription)
	cmd.Flags().BoolVar(&vars.defaultConfig, defaultConfigFlag, false, defaultConfigFlagDescription)

	flags := pflag.NewFlagSet("Common", pflag.ContinueOnError)
	flags.AddFlag(cmd.Flags().Lookup(appFlag))
	flags.AddFlag(cmd.Flags().Lookup(nameFlag))
	flags.AddFlag(cmd.Flags().Lookup(profileFlag))
	flags.AddFlag(cmd.Flags().Lookup(accessKeyIDFlag))
	flags.AddFlag(cmd.Flags().Lookup(secretAccessKeyFlag))
	flags.AddFlag(cmd.Flags().Lookup(sessionTokenFlag))
	flags.AddFlag(cmd.Flags().Lookup(regionFlag))
	flags.AddFlag(cmd.Flags().Lookup(defaultConfigFlag))
	flags.AddFlag(cmd.Flags().Lookup(allowDowngradeFlag))

	resourcesImportFlags := pflag.NewFlagSet("Import Existing Resources", pflag.ContinueOnError)
	resourcesImportFlags.AddFlag(cmd.Flags().Lookup(vpcIDFlag))
	resourcesImportFlags.AddFlag(cmd.Flags().Lookup(publicSubnetsFlag))
	resourcesImportFlags.AddFlag(cmd.Flags().Lookup(privateSubnetsFlag))
	resourcesImportFlags.AddFlag(cmd.Flags().Lookup(certsFlag))

	resourcesConfigFlags := pflag.NewFlagSet("Configure Default Resources", pflag.ContinueOnError)
	resourcesConfigFlags.AddFlag(cmd.Flags().Lookup(overrideVPCCIDRFlag))
	resourcesConfigFlags.AddFlag(cmd.Flags().Lookup(overrideAZsFlag))
	resourcesConfigFlags.AddFlag(cmd.Flags().Lookup(overridePublicSubnetCIDRsFlag))
	resourcesConfigFlags.AddFlag(cmd.Flags().Lookup(overridePrivateSubnetCIDRsFlag))
	resourcesConfigFlags.AddFlag(cmd.Flags().Lookup(internalALBSubnetsFlag))
	resourcesConfigFlags.AddFlag(cmd.Flags().Lookup(allowVPCIngressFlag))

	telemetryFlags := pflag.NewFlagSet("Telemetry", pflag.ContinueOnError)
	telemetryFlags.AddFlag(cmd.Flags().Lookup(enableContainerInsightsFlag))

	cmd.Annotations = map[string]string{
		// The order of the sections we want to display.
		"sections":                    "Common,Import Existing Resources,Configure Default Resources,Telemetry",
		"Common":                      flags.FlagUsages(),
		"Import Existing Resources":   resourcesImportFlags.FlagUsages(),
		"Configure Default Resources": resourcesConfigFlags.FlagUsages(),
		"Telemetry":                   telemetryFlags.FlagUsages(),
	}

	cmd.SetUsageTemplate(`{{h1 "Usage"}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{$annotations := .Annotations}}{{$sections := split .Annotations.sections ","}}{{if gt (len $sections) 0}}

{{range $i, $sectionName := $sections}}{{h1 (print $sectionName " Flags")}}
{{(index $annotations $sectionName) | trimTrailingWhitespaces}}{{if ne (inc $i) (len $sections)}}

{{end}}{{end}}{{end}}{{if .HasAvailableInheritedFlags}}

{{h1 "Global Flags"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasExample}}

{{h1 "Examples"}}{{code .Example}}{{end}}
`)

	return cmd
}

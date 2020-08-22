// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/ec2"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/profile"
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
	envInitDefaultEnvConfirmPrompt = `Would you like to use the default configuration for a new production environment?
    - A new VPC with 2 AZs, 2 public subnets and 2 private subnets
    - A new ECS Cluster
    - New IAM Roles to manage services in your environment
`
	envInitVPCSelectPrompt            = "Which VPC would you like to use?"
	envInitPublicSubnetsSelectPrompt  = "Which public subnets would you like to use?"
	envInitPrivateSubnetsSelectPrompt = "Which private subnets would you like to use?"

	envInitVPCCIDRPrompt         = "What VPC CIDR would you like to use?"
	envInitVPCCIDRPromptHelp     = "CIDR used for your VPC. For example: 10.1.0.0/16"
	envInitPublicCIDRPrompt      = "What CIDR would you like to use for your public subnets?"
	envInitPublicCIDRPromptHelp  = "CIDRs used for your public subnets. For example: 10.1.0.0/24,10.1.1.0/24"
	envInitPrivateCIDRPrompt     = "What CIDR would you like to use for your private subnets?"
	envInitPrivateCIDRPromptHelp = "CIDRs used for your private subnets. For example: 10.1.2.0/24,10.1.3.0/24"

	fmtEnvInitCredsPrompt  = "Which credentials would you like to use to create %s?"
	envInitCredsHelpPrompt = `The credentials are used to create your environment in an AWS account and region.
To learn more:
https://github.com/aws/copilot-cli/wiki/credentials#environment-credentials`
	envInitRegionPrompt        = "Which region?"
	envInitDefaultRegionOption = "us-west-2"

	fmtDeployEnvStart        = "Proposing infrastructure changes for the %s environment."
	fmtDeployEnvComplete     = "Environment %s already exists in application %s.\n"
	fmtDeployEnvFailed       = "Failed to accept changes for the %s environment.\n"
	fmtDNSDelegationStart    = "Sharing DNS permissions for this application to account %s."
	fmtDNSDelegationFailed   = "Failed to grant DNS permissions to account %s.\n"
	fmtDNSDelegationComplete = "Shared DNS permissions for this application to account %s.\n"
	fmtStreamEnvStart        = "Creating the infrastructure for the %s environment."
	fmtStreamEnvFailed       = "Failed to create the infrastructure for the %s environment.\n"
	fmtStreamEnvComplete     = "Created the infrastructure for the %s environment.\n"
	fmtAddEnvToAppStart      = "Linking account %s and region %s to application %s."
	fmtAddEnvToAppFailed     = "Failed to link account %s and region %s to application %s.\n"
	fmtAddEnvToAppComplete   = "Linked account %s and region %s to application %s.\n"
)

var (
	errNamedProfilesNotFound = fmt.Errorf("no named AWS profiles found, run %s first please", color.HighlightCode("aws configure"))

	envInitDefaultConfigSelectOption      = "Yes, use default."
	envInitAdjustEnvResourcesSelectOption = "Yes, but I'd like configure the default resources (CIDR ranges)."
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
	PublicSubnetCIDRs  []string
	PrivateSubnetCIDRs []string
}

func (v adjustVPCVars) isSet() bool {
	if v.CIDR.String() != emptyIPNet.String() {
		return true
	}
	return len(v.PublicSubnetCIDRs) != 0 || len(v.PrivateSubnetCIDRs) != 0
}

type tempCredsVars struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
}

func (v tempCredsVars) isSet() bool {
	return v.AccessKeyID != "" && v.SecretAccessKey != ""
}

type initEnvVars struct {
	*GlobalOpts
	Name          string // Name for the environment.
	Profile       string // The named profile to use for credential retrieval. Mutually exclusive with TempCreds.
	IsProduction  bool   // True means retain resources even after deletion.
	DefaultConfig bool   // True means using default environment configuration.

	ImportVPC importVPCVars // Existing VPC resources to use instead of creating new ones.
	AdjustVPC adjustVPCVars // Configure parameters for VPC resources generated while initializing an environment.

	TempCreds tempCredsVars // Temporary credentials to initialize the environment. Mutually exclusive with the Profile.
	Region    string        // The region to create the environment in.
}

type initEnvOpts struct {
	initEnvVars

	// Interfaces to interact with dependencies.
	sessProvider sessionProvider
	store        store
	envDeployer  deployer
	appDeployer  deployer
	identity     identityService
	envIdentity  identityService
	prog         progress
	selVPC       ec2Selector
	selCreds     credsSelector
	selApp   	 appSelector

	sess *session.Session // Session pointing to environment's AWS account and region.
}

func newInitEnvOpts(vars initEnvVars) (*initEnvOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, err
	}
	sessProvider := sessions.NewProvider()
	defaultSession, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}
	cfg, err := profile.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("read named profiles: %w", err)
	}

	return &initEnvOpts{
		initEnvVars:  vars,
		sessProvider: sessProvider,
		store:        store,
		appDeployer:  deploycfn.New(defaultSession),
		identity:     identity.New(defaultSession),
		prog:         termprogress.NewSpinner(),
		selCreds: &selector.CredsSelect{
			Session: sessProvider,
			Profile: cfg,
			Prompt:  vars.prompt,
		},
		selApp:		  selector.NewSelect(vars.prompt, store),
	}, nil
}

// Validate returns an error if the values passed by flags are invalid.
func (o *initEnvOpts) Validate() error {
	if o.Name != "" {
		if err := validateEnvironmentName(o.Name); err != nil {
			return err
		}
	}

	if o.AppName() == "" {
		existingApps, _ := o.store.ListApplications()
		if len(existingApps) == 0 {
			return fmt.Errorf("no application found: run %s or %s into your workspace please", color.HighlightCode("app init"), color.HighlightCode("cd"))
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
	if o.appName == "" {
		name, err := o.selApp.Application(appShowNamePrompt, appShowNameHelpPrompt)
		if err != nil {
			return fmt.Errorf("select application: %w", err)
		}
		o.appName = name
	}
	return o.askCustomizedResources()
}

// Execute deploys a new environment with CloudFormation and adds it to SSM.
func (o *initEnvOpts) Execute() error {
	// Initialize environment clients if not set.
	if o.envIdentity == nil {
		o.envIdentity = identity.New(o.sess)
	}
	if o.envDeployer == nil {
		o.envDeployer = deploycfn.New(o.sess)
	}

	app, err := o.store.GetApplication(o.AppName())
	if err != nil {
		// Ensure the app actually exists before we do a deployment.
		return err
	}

	if app.RequiresDNSDelegation() {
		if err := o.delegateDNSFromApp(app); err != nil {
			return fmt.Errorf("granting DNS permissions: %w", err)
		}
	}
	// 1. Start creating the CloudFormation stack for the environment.
	if err := o.deployEnv(app); err != nil {
		return err
	}

	// 2. Get the environment
	env, err := o.envDeployer.GetEnvironment(o.AppName(), o.Name)
	if err != nil {
		return fmt.Errorf("get environment struct for %s: %w", o.Name, err)
	}
	env.Prod = o.IsProduction

	// 3. Add the stack set instance to the app stackset.
	if err := o.addToStackset(app, env); err != nil {
		return err
	}

	// 4. Store the environment in SSM.
	if err := o.store.CreateEnvironment(env); err != nil {
		return fmt.Errorf("store environment: %w", err)
	}
	log.Successf("Created environment %s in region %s under application %s.\n",
		color.HighlightUserInput(env.Name), color.Emphasize(env.Region), color.HighlightUserInput(env.App))
	return nil
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *initEnvOpts) RecommendedActions() []string {
	return nil
}

func (o *initEnvOpts) validateCustomizedResources() error {
	if o.ImportVPC.isSet() && o.AdjustVPC.isSet() {
		return errors.New("cannot specify both import vpc flags and configure vpc flags")
	}
	if (o.ImportVPC.isSet() || o.AdjustVPC.isSet()) && o.DefaultConfig {
		return fmt.Errorf("cannot import or configure vpc if --%s is set", defaultConfigFlag)
	}
	return nil
}

func (o *initEnvOpts) askEnvName() error {
	if o.Name != "" {
		return nil
	}

	envName, err := o.prompt.Get(envInitNamePrompt, envInitNameHelpPrompt, validateEnvironmentName)
	if err != nil {
		return fmt.Errorf("get environment name: %w", err)
	}
	o.Name = envName
	return nil
}

func (o *initEnvOpts) askEnvSession() error {
	if o.Profile != "" {
		sess, err := o.sessProvider.FromProfile(o.Profile)
		if err != nil {
			return fmt.Errorf("create session from profile %s: %w", o.Profile, err)
		}
		o.sess = sess
		return nil
	}
	if o.TempCreds.isSet() {
		sess, err := o.sessProvider.FromStaticCreds(o.TempCreds.AccessKeyID, o.TempCreds.SecretAccessKey, o.TempCreds.SessionToken)
		if err != nil {
			return err
		}
		o.sess = sess
		return nil
	}
	sess, err := o.selCreds.Creds(fmt.Sprintf(fmtEnvInitCredsPrompt, color.HighlightUserInput(o.Name)), envInitCredsHelpPrompt)
	if err != nil {
		return fmt.Errorf("select creds: %w", err)
	}
	o.sess = sess
	return nil
}

func (o *initEnvOpts) askEnvRegion() error {
	region := aws.StringValue(o.sess.Config.Region)
	if o.Region != "" {
		region = o.Region
	}
	if region == "" {
		v, err := o.prompt.Get(envInitRegionPrompt, "", nil, prompt.WithDefaultInput(envInitDefaultRegionOption))
		if err != nil {
			return fmt.Errorf("get environment region: %w", err)
		}
		region = v
	}
	o.sess.Config.Region = aws.String(region)
	return nil
}

func (o *initEnvOpts) askCustomizedResources() error {
	if o.DefaultConfig {
		return nil
	}
	if o.ImportVPC.isSet() {
		return o.askImportResources()
	}
	if o.AdjustVPC.isSet() {
		return o.askAdjustResources()
	}
	adjustOrImport, err := o.prompt.SelectOne(
		envInitDefaultEnvConfirmPrompt, "",
		envInitCustomizedEnvTypes)
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
	if o.ImportVPC.ID == "" {
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
		o.ImportVPC.ID = vpcID
	}
	if o.ImportVPC.PublicSubnetIDs == nil {
		publicSubnets, err := o.selVPC.PublicSubnets(envInitPublicSubnetsSelectPrompt, "", o.ImportVPC.ID)
		if err != nil {
			if err == selector.ErrSubnetsNotFound {
				log.Errorf(`No existing public subnets were found in VPC %s. You can either:
- Create new public subnets and then import them.
- Use the default Copilot environment configuration.`, o.ImportVPC.ID)
			}
			return fmt.Errorf("select public subnets: %w", err)
		}
		o.ImportVPC.PublicSubnetIDs = publicSubnets
	}
	if o.ImportVPC.PrivateSubnetIDs == nil {
		privateSubnets, err := o.selVPC.PrivateSubnets(envInitPrivateSubnetsSelectPrompt, "", o.ImportVPC.ID)
		if err != nil {
			if err == selector.ErrSubnetsNotFound {
				log.Errorf(`No existing private subnets were found in VPC %s. You can either:
- Create new private subnets and then import them.
- Use the default Copilot environment configuration.`, o.ImportVPC.ID)
			}
			return fmt.Errorf("select private subnets: %w", err)
		}
		o.ImportVPC.PrivateSubnetIDs = privateSubnets
	}
	return nil
}

func (o *initEnvOpts) askAdjustResources() error {
	if o.AdjustVPC.CIDR.String() == emptyIPNet.String() {
		vpcCIDRString, err := o.prompt.Get(envInitVPCCIDRPrompt, envInitVPCCIDRPromptHelp, validateCIDR,
			prompt.WithDefaultInput(stack.DefaultVPCCIDR))
		if err != nil {
			return fmt.Errorf("get VPC CIDR: %w", err)
		}
		_, vpcCIDR, err := net.ParseCIDR(vpcCIDRString)
		if err != nil {
			return fmt.Errorf("parse VPC CIDR: %w", err)
		}
		o.AdjustVPC.CIDR = *vpcCIDR
	}
	if o.AdjustVPC.PublicSubnetCIDRs == nil {
		publicCIDR, err := o.prompt.Get(envInitPublicCIDRPrompt, envInitPublicCIDRPromptHelp, validateCIDRSlice,
			prompt.WithDefaultInput(stack.DefaultPublicSubnetCIDRs))
		if err != nil {
			return fmt.Errorf("get public subnet CIDRs: %w", err)
		}
		o.AdjustVPC.PublicSubnetCIDRs = strings.Split(publicCIDR, ",")
	}
	if o.AdjustVPC.PrivateSubnetCIDRs == nil {
		privateCIDR, err := o.prompt.Get(envInitPrivateCIDRPrompt, envInitPrivateCIDRPromptHelp, validateCIDRSlice,
			prompt.WithDefaultInput(stack.DefaultPrivateSubnetCIDRs))
		if err != nil {
			return fmt.Errorf("get private subnet CIDRs: %w", err)
		}
		o.AdjustVPC.PrivateSubnetCIDRs = strings.Split(privateCIDR, ",")
	}
	return nil
}

func (o *initEnvOpts) importVPCConfig() *deploy.ImportVPCConfig {
	if o.DefaultConfig || !o.ImportVPC.isSet() {
		return nil
	}
	return &deploy.ImportVPCConfig{
		ID:               o.ImportVPC.ID,
		PrivateSubnetIDs: o.ImportVPC.PrivateSubnetIDs,
		PublicSubnetIDs:  o.ImportVPC.PublicSubnetIDs,
	}
}

func (o *initEnvOpts) adjustVPCConfig() *deploy.AdjustVPCConfig {
	if o.DefaultConfig || !o.AdjustVPC.isSet() {
		return nil
	}
	return &deploy.AdjustVPCConfig{
		CIDR:               o.AdjustVPC.CIDR.String(),
		PrivateSubnetCIDRs: o.AdjustVPC.PrivateSubnetCIDRs,
		PublicSubnetCIDRs:  o.AdjustVPC.PublicSubnetCIDRs,
	}
}

func (o *initEnvOpts) deployEnv(app *config.Application) error {
	caller, err := o.identity.Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}
	deployEnvInput := &deploy.CreateEnvironmentInput{
		Name:                     o.Name,
		AppName:                  o.AppName(),
		Prod:                     o.IsProduction,
		PublicLoadBalancer:       true, // TODO: configure this based on user input or service Type needs?
		ToolsAccountPrincipalARN: caller.RootUserARN,
		AppDNSName:               app.Domain,
		AdditionalTags:           app.Tags,
		AdjustVPCConfig:          o.adjustVPCConfig(),
		ImportVPCConfig:          o.importVPCConfig(),
	}

	o.prog.Start(fmt.Sprintf(fmtDeployEnvStart, color.HighlightUserInput(o.Name)))
	if err := o.envDeployer.DeployEnvironment(deployEnvInput); err != nil {
		var existsErr *cloudformation.ErrStackAlreadyExists
		if errors.As(err, &existsErr) {
			// Do nothing if the stack already exists.
			o.prog.Stop(log.Ssuccessf(fmtDeployEnvComplete,
				color.HighlightUserInput(o.Name), color.HighlightUserInput(o.AppName())))
			return nil
		}
		o.prog.Stop(log.Serrorf(fmtDeployEnvFailed, color.HighlightUserInput(o.Name)))
		return err
	}

	// Display updates while the deployment is happening.
	o.prog.Start(fmt.Sprintf(fmtStreamEnvStart, color.HighlightUserInput(o.Name)))
	stackEvents, responses := o.envDeployer.StreamEnvironmentCreation(deployEnvInput)
	for stackEvent := range stackEvents {
		o.prog.Events(o.humanizeEnvironmentEvents(stackEvent))
	}
	resp := <-responses
	if resp.Err != nil {
		o.prog.Stop(log.Serrorf(fmtStreamEnvFailed, color.HighlightUserInput(o.Name)))
		return resp.Err
	}
	o.prog.Stop(log.Ssuccessf(fmtStreamEnvComplete, color.HighlightUserInput(o.Name)))

	return nil
}

func (o *initEnvOpts) addToStackset(app *config.Application, env *config.Environment) error {
	o.prog.Start(fmt.Sprintf(fmtAddEnvToAppStart, color.Emphasize(env.AccountID), color.Emphasize(env.Region), color.HighlightUserInput(o.AppName())))
	if err := o.appDeployer.AddEnvToApp(app, env); err != nil {
		o.prog.Stop(log.Serrorf(fmtAddEnvToAppFailed, color.Emphasize(env.AccountID), color.Emphasize(env.Region), color.HighlightUserInput(o.AppName())))
		return fmt.Errorf("deploy env %s to application %s: %w", env.Name, app.Name, err)
	}
	o.prog.Stop(log.Ssuccessf(fmtAddEnvToAppComplete, color.Emphasize(env.AccountID), color.Emphasize(env.Region), color.HighlightUserInput(o.AppName())))

	return nil
}

func (o *initEnvOpts) delegateDNSFromApp(app *config.Application) error {
	envAccount, err := o.envIdentity.Get()
	if err != nil {
		return fmt.Errorf("getting environment account ID for DNS Delegation: %w", err)
	}

	// By default, our DNS Delegation permits same account delegation.
	if envAccount.Account == app.AccountID {
		return nil
	}

	o.prog.Start(fmt.Sprintf(fmtDNSDelegationStart, color.HighlightUserInput(envAccount.Account)))
	if err := o.appDeployer.DelegateDNSPermissions(app, envAccount.Account); err != nil {
		o.prog.Stop(log.Serrorf(fmtDNSDelegationFailed, color.HighlightUserInput(envAccount.Account)))
		return err
	}
	o.prog.Stop(log.Ssuccessf(fmtDNSDelegationComplete, color.HighlightUserInput(envAccount.Account)))
	return nil
}

func (o *initEnvOpts) humanizeEnvironmentEvents(resourceEvents []deploy.ResourceEvent) []termprogress.TabRow {
	matcher := map[termprogress.Text]termprogress.ResourceMatcher{
		textVPC: func(event deploy.Resource) bool {
			return event.Type == "AWS::EC2::VPC"
		},
		textInternetGateway: func(event deploy.Resource) bool {
			return event.Type == "AWS::EC2::InternetGateway" ||
				event.Type == "AWS::EC2::VPCGatewayAttachment"
		},
		textPublicSubnets: func(event deploy.Resource) bool {
			return event.Type == "AWS::EC2::Subnet" &&
				strings.HasPrefix(event.LogicalName, "Public")
		},
		textPrivateSubnets: func(event deploy.Resource) bool {
			return event.Type == "AWS::EC2::Subnet" &&
				strings.HasPrefix(event.LogicalName, "Private")
		},
		textRouteTables: func(event deploy.Resource) bool {
			return strings.Contains(event.LogicalName, "Route")
		},
		textECSCluster: func(event deploy.Resource) bool {
			return event.Type == "AWS::ECS::Cluster"
		},
		textALB: func(event deploy.Resource) bool {
			return strings.Contains(event.LogicalName, "LoadBalancer") ||
				strings.Contains(event.Type, "ElasticLoadBalancingV2")
		},
	}
	return termprogress.HumanizeResourceEvents(o.envProgressOrder(), resourceEvents, matcher, defaultResourceCounts)
}

func (o *initEnvOpts) envProgressOrder() (order []termprogress.Text) {
	if !o.ImportVPC.isSet() {
		order = append(order, []termprogress.Text{textVPC, textInternetGateway, textPublicSubnets, textPrivateSubnets, textRouteTables}...)
	}
	order = append(order, []termprogress.Text{textECSCluster, textALB}...)
	return
}

func (o *initEnvOpts) validateCredentials() error {
	if o.Profile != "" && o.TempCreds.AccessKeyID != "" {
		return fmt.Errorf("cannot specify both --%s and --%s", profileFlag, accessKeyIDFlag)
	}
	if o.Profile != "" && o.TempCreds.SecretAccessKey != "" {
		return fmt.Errorf("cannot specify both --%s and --%s", profileFlag, secretAccessKeyFlag)
	}
	if o.Profile != "" && o.TempCreds.SessionToken != "" {
		return fmt.Errorf("cannot specify both --%s and --%s", profileFlag, sessionTokenFlag)
	}
	return nil
}

// BuildEnvInitCmd builds the command for adding an environment.
func BuildEnvInitCmd() *cobra.Command {
	vars := initEnvVars{
		GlobalOpts: NewGlobalOpts(),
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a new environment in your application.",
		Example: `
  Creates a test environment in your "default" AWS profile using default configuration.
  /code $ copilot env init --name test --profile default --default-config

  Creates a prod-iad environment using your "prod-admin" AWS profile.
  /code $ copilot env init --name prod-iad --profile prod-admin --prod

  Creates an environment with imported VPC resources.
  /code $ copilot env init --import-vpc-id vpc-099c32d2b98cdcf47 \
  /code --import-public-subnets subnet-013e8b691862966cf,subnet -014661ebb7ab8681a \
  /code --import-private-subnets subnet-055fafef48fb3c547,subnet-00c9e76f288363e7f

  Creates an environment with overrided CIDRs.
  /code $ copilot env init --override-vpc-cidr 10.1.0.0/16 \
  /code --override-public-cidrs 10.1.0.0/24,10.1.1.0/24 \
  /code --override-private-cidrs 10.1.2.0/24,10.1.3.0/24`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newInitEnvOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	cmd.Flags().StringVarP(&vars.Name, nameFlag, nameFlagShort, "", envFlagDescription)

	cmd.Flags().StringVar(&vars.Profile, profileFlag, "", profileFlagDescription)
	cmd.Flags().StringVar(&vars.TempCreds.AccessKeyID, accessKeyIDFlag, "", accessKeyIDFlagDescription)
	cmd.Flags().StringVar(&vars.TempCreds.SecretAccessKey, secretAccessKeyFlag, "", secretAccessKeyFlagDescription)
	cmd.Flags().StringVar(&vars.TempCreds.SessionToken, sessionTokenFlag, "", sessionTokenFlagDescription)
	cmd.Flags().StringVar(&vars.Region, regionFlag, "", envRegionTokenFlagDescription)

	cmd.Flags().BoolVar(&vars.IsProduction, prodEnvFlag, false, prodEnvFlagDescription)

	cmd.Flags().StringVar(&vars.ImportVPC.ID, vpcIDFlag, "", vpcIDFlagDescription)
	cmd.Flags().StringSliceVar(&vars.ImportVPC.PublicSubnetIDs, publicSubnetsFlag, nil, publicSubnetsFlagDescription)
	cmd.Flags().StringSliceVar(&vars.ImportVPC.PrivateSubnetIDs, privateSubnetsFlag, nil, privateSubnetsFlagDescription)

	cmd.Flags().IPNetVar(&vars.AdjustVPC.CIDR, vpcCIDRFlag, net.IPNet{}, vpcCIDRFlagDescription)
	// TODO: use IPNetSliceVar when it is available (https://github.com/spf13/pflag/issues/273).
	cmd.Flags().StringSliceVar(&vars.AdjustVPC.PublicSubnetCIDRs, publicSubnetCIDRsFlag, nil, publicSubnetCIDRsFlagDescription)
	cmd.Flags().StringSliceVar(&vars.AdjustVPC.PrivateSubnetCIDRs, privateSubnetCIDRsFlag, nil, privateSubnetCIDRsFlagDescription)
	cmd.Flags().BoolVar(&vars.DefaultConfig, defaultConfigFlag, false, defaultConfigFlagDescription)

	flags := pflag.NewFlagSet("Common", pflag.ContinueOnError)
	flags.AddFlag(cmd.Flags().Lookup(nameFlag))
	flags.AddFlag(cmd.Flags().Lookup(profileFlag))
	flags.AddFlag(cmd.Flags().Lookup(accessKeyIDFlag))
	flags.AddFlag(cmd.Flags().Lookup(secretAccessKeyFlag))
	flags.AddFlag(cmd.Flags().Lookup(sessionTokenFlag))
	flags.AddFlag(cmd.Flags().Lookup(regionFlag))
	flags.AddFlag(cmd.Flags().Lookup(defaultConfigFlag))
	flags.AddFlag(cmd.Flags().Lookup(prodEnvFlag))

	resourcesImportFlag := pflag.NewFlagSet("Import Existing Resources", pflag.ContinueOnError)
	resourcesImportFlag.AddFlag(cmd.Flags().Lookup(vpcIDFlag))
	resourcesImportFlag.AddFlag(cmd.Flags().Lookup(publicSubnetsFlag))
	resourcesImportFlag.AddFlag(cmd.Flags().Lookup(privateSubnetsFlag))

	resourcesConfigFlag := pflag.NewFlagSet("Configure Default Resources", pflag.ContinueOnError)
	resourcesConfigFlag.AddFlag(cmd.Flags().Lookup(vpcCIDRFlag))
	resourcesConfigFlag.AddFlag(cmd.Flags().Lookup(publicSubnetCIDRsFlag))
	resourcesConfigFlag.AddFlag(cmd.Flags().Lookup(privateSubnetCIDRsFlag))

	cmd.Annotations = map[string]string{
		// The order of the sections we want to display.
		"sections":                    "Common,Import Existing Resources,Configure Default Resources",
		"Common":                      flags.FlagUsages(),
		"Import Existing Resources":   resourcesImportFlag.FlagUsages(),
		"Configure Default Resources": resourcesConfigFlag.FlagUsages(),
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

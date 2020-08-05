// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/profile"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	deploycfn "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	envInitNamePrompt     = "What is your environment's name?"
	envInitNameHelpPrompt = "A unique identifier for an environment (e.g. dev, test, prod)."

	fmtEnvInitProfilePrompt  = "Which named profile should we use to create %s?"
	envInitProfileHelpPrompt = "The AWS CLI named profile with the permissions to create an environment."
)

const (
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
)

type initEnvVars struct {
	*GlobalOpts
	Name         string
	Profile      string
	IsProduction bool
	UseDefault   bool

	ImportVPC struct {
		ID               string
		PublicSubnetIDs  []string
		PrivateSubnetIDs []string
	}
	AdjustVPC struct {
		CIDR               net.IPNet
		PublicSubnetCIDRs  []string
		PrivateSubnetCIDRs []string
	}
}

type initEnvOpts struct {
	initEnvVars

	// Interfaces to interact with dependencies.
	store         store
	envDeployer   deployer
	appDeployer   deployer
	identity      identityService
	envIdentity   identityService
	profileConfig profileNames
	prog          progress

	// initialize profile-specific env clients
	initProfileClients func(*initEnvOpts) error
}

var initEnvProfileClients = func(o *initEnvOpts) error {
	profileSess, err := sessions.NewProvider().FromProfile(o.Profile)
	if err != nil {
		return fmt.Errorf("create session from profile %s: %w", o.Profile, err)
	}
	o.envIdentity = identity.New(profileSess)
	o.envDeployer = deploycfn.New(profileSess)
	return nil
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
		initEnvVars:        vars,
		store:              store,
		appDeployer:        deploycfn.New(defaultSession),
		identity:           identity.New(defaultSession),
		profileConfig:      cfg,
		prog:               termprogress.NewSpinner(),
		initProfileClients: initEnvProfileClients,
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
		return fmt.Errorf("no application found: run %s or %s into your workspace please", color.HighlightCode("app init"), color.HighlightCode("cd"))
	}
	return o.validateCustomizedResources()
}

// Ask asks for fields that are required but not passed in.
func (o *initEnvOpts) Ask() error {
	if err := o.askEnvName(); err != nil {
		return err
	}
	if err := o.askEnvProfile(); err != nil {
		return err
	}
	return o.askCustomizedResources()
}

// Execute deploys a new environment with CloudFormation and adds it to SSM.
func (o *initEnvOpts) Execute() error {
	app, err := o.store.GetApplication(o.AppName())
	if err != nil {
		// Ensure the app actually exists before we do a deployment.
		return err
	}

	if err = o.initProfileClients(o); err != nil {
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

func (o *initEnvOpts) validateCustomizedResources() error {
	var vpcImport, vpcConfig bool
	if (o.ImportVPC.ID != "") || (o.ImportVPC.PublicSubnetIDs != nil) || (o.ImportVPC.PrivateSubnetIDs != nil) {
		vpcImport = true
	}
	if (o.AdjustVPC.CIDR.String() != deploy.EmptyIPNetString) || (o.AdjustVPC.PublicSubnetCIDRs != nil) || (o.AdjustVPC.PrivateSubnetCIDRs != nil) {
		vpcConfig = true
	}
	if vpcImport && vpcConfig {
		return errors.New("couldn't specify both vpc importing flags and vpc configuring flags")
	}
	if (vpcImport || vpcConfig) && o.UseDefault {
		return errors.New("couldn't import or configure resources if use default flag is set")
	}
	return nil
}

func (o *initEnvOpts) askEnvName() error {
	if o.Name != "" {
		return nil
	}

	envName, err := o.prompt.Get(envInitNamePrompt, envInitNameHelpPrompt, validateEnvironmentName)
	if err != nil {
		return fmt.Errorf("prompt to get environment name: %w", err)
	}
	o.Name = envName
	return nil
}

func (o *initEnvOpts) askEnvProfile() error {
	// TODO: add this behavior to selector pkg.
	if o.Profile != "" {
		return nil
	}

	names := o.profileConfig.Names()
	if len(names) == 0 {
		return errNamedProfilesNotFound
	}

	profile, err := o.prompt.SelectOne(
		fmt.Sprintf(fmtEnvInitProfilePrompt, color.HighlightUserInput(o.Name)),
		envInitProfileHelpPrompt,
		names)
	if err != nil {
		return fmt.Errorf("get the profile name: %w", err)
	}
	o.Profile = profile
	return nil
}

func (o *initEnvOpts) askCustomizedResources() error {
	if o.UseDefault {
		return nil
	}
	return nil
}

func (o *initEnvOpts) importVPCConfig() *deploy.ImportVPCConfig {
	if o.UseDefault {
		return nil
	}
	config := &deploy.ImportVPCConfig{
		ID:               o.ImportVPC.ID,
		PrivateSubnetIDs: o.ImportVPC.PrivateSubnetIDs,
		PublicSubnetIDs:  o.ImportVPC.PublicSubnetIDs,
	}
	if config.IsEmpty() {
		return nil
	}
	return config
}

func (o *initEnvOpts) adjustVPCConfig() *deploy.AdjustVPCConfig {
	if o.UseDefault {
		return nil
	}
	config := &deploy.AdjustVPCConfig{
		CIDR:               o.AdjustVPC.CIDR.String(),
		PrivateSubnetCIDRs: o.AdjustVPC.PrivateSubnetCIDRs,
		PublicSubnetCIDRs:  o.AdjustVPC.PublicSubnetCIDRs,
	}
	if config.IsEmpty() {
		return nil
	}
	return config
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
	resourceCounts := map[termprogress.Text]int{
		textVPC:             1,
		textInternetGateway: 2,
		textPublicSubnets:   2,
		textPrivateSubnets:  2,
		textRouteTables:     4,
		textECSCluster:      1,
		textALB:             4,
	}
	return termprogress.HumanizeResourceEvents(envProgressOrder, resourceEvents, matcher, resourceCounts)
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *initEnvOpts) RecommendedActions() []string {
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
  Creates a test environment in your "default" AWS profile.
  /code $ copilot env init --name test --profile default

  Creates a prod-iad environment using your "prod-admin" AWS profile.
  /code $ copilot env init --name prod-iad --profile prod-admin --prod

  Creates an environment with imported VPC resources.
  /code $ copilot env init --import-vpc-id vpc-099c32d2b98cdcf47
  /code --import-public-subnets subnet-013e8b691862966cf,subnet -014661ebb7ab8681a
  /code --import-private-subnets subnet-055fafef48fb3c547,subnet-00c9e76f288363e7f

  Creates an environment with overrided CIDRs.
  /code $ copilot env init --override-vpc-cidr 10.1.0.0/16
  /code --override-public-cidrs 10.1.0.0/24,10.1.1.0/24
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
	cmd.Flags().BoolVar(&vars.IsProduction, prodEnvFlag, false, prodEnvFlagDescription)
	cmd.Flags().StringVar(&vars.ImportVPC.ID, vpcIDFlag, "", vpcIDFlagDescription)
	cmd.Flags().StringSliceVar(&vars.ImportVPC.PublicSubnetIDs, publicSubnetsFlag, nil, publicSubnetsFlagDescription)
	cmd.Flags().StringSliceVar(&vars.ImportVPC.PrivateSubnetIDs, privateSubnetsFlag, nil, privateSubnetsFlagDescription)
	cmd.Flags().IPNetVar(&vars.AdjustVPC.CIDR, vpcCIDRFlag, net.IPNet{}, vpcCIDRFlagDescription)
	// TODO: use IPNetSliceVar when it is available (https://github.com/spf13/pflag/issues/273).
	cmd.Flags().StringSliceVar(&vars.AdjustVPC.PublicSubnetCIDRs, publicSubnetCIDRsFlag, nil, publicSubnetCIDRsFlagDescription)
	cmd.Flags().StringSliceVar(&vars.AdjustVPC.PrivateSubnetCIDRs, privateSubnetCIDRsFlag, nil, privateSubnetCIDRsFlagDescription)
	cmd.Flags().BoolVar(&vars.UseDefault, defaultEnvironmentFlag, false, defaultEnvironmentFlagDescription)

	requiredFlags := pflag.NewFlagSet("Required", pflag.ContinueOnError)
	requiredFlags.AddFlag(cmd.Flags().Lookup(nameFlag))
	requiredFlags.AddFlag(cmd.Flags().Lookup(profileFlag))

	resourcesImportFlag := pflag.NewFlagSet("Import Existing Resources", pflag.ContinueOnError)
	resourcesImportFlag.AddFlag(cmd.Flags().Lookup(vpcIDFlag))
	resourcesImportFlag.AddFlag(cmd.Flags().Lookup(publicSubnetsFlag))
	resourcesImportFlag.AddFlag(cmd.Flags().Lookup(privateSubnetsFlag))

	resourcesConfigFlag := pflag.NewFlagSet("Configure Default Resources", pflag.ContinueOnError)
	resourcesConfigFlag.AddFlag(cmd.Flags().Lookup(vpcCIDRFlag))
	resourcesConfigFlag.AddFlag(cmd.Flags().Lookup(publicSubnetCIDRsFlag))
	resourcesConfigFlag.AddFlag(cmd.Flags().Lookup(privateSubnetCIDRsFlag))

	optionalFlags := pflag.NewFlagSet("Optional", pflag.ContinueOnError)
	optionalFlags.AddFlag(cmd.Flags().Lookup(defaultEnvironmentFlag))
	optionalFlags.AddFlag(cmd.Flags().Lookup(prodEnvFlag))

	cmd.Annotations = map[string]string{
		// The order of the sections we want to display.
		"sections":                    "Required,Import Existing Resources,Configure Default Resources,Optional",
		"Required":                    requiredFlags.FlagUsages(),
		"Import Existing Resources":   resourcesImportFlag.FlagUsages(),
		"Configure Default Resources": resourcesConfigFlag.FlagUsages(),
		"Optional":                    optionalFlags.FlagUsages(),
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

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/profile"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	deploycfn "github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/spf13/cobra"
)

const (
	envInitNamePrompt     = "What is your environment's name?"
	envInitNameHelpPrompt = "A unique identifier for an environment (e.g. dev, test, prod)."

	fmtEnvInitProfilePrompt  = "Which named profile should we use to create %s?"
	envInitProfileHelpPrompt = "The AWS CLI named profile with the permissions to create an environment."
)

const (
	fmtDeployEnvStart          = "Proposing infrastructure changes for the %s environment."
	fmtDeployEnvFailed         = "Failed to accept changes for the %s environment."
	fmtDNSDelegationStart      = "Sharing DNS permissions for this project to account %s."
	fmtDNSDelegationFailed     = "Failed to grant DNS permissions to account %s."
	fmtDNSDelegationComplete   = "Shared DNS permissions for this project to account %s."
	fmtStreamEnvStart          = "Creating the infrastructure for the %s environment."
	fmtStreamEnvFailed         = "Failed to create the infrastructure for the %s environment."
	fmtStreamEnvComplete       = "Created the infrastructure for the %s environment."
	fmtAddEnvToProjectStart    = "Linking account %s and region %s to project %s."
	fmtAddEnvToProjectFailed   = "Failed to link account %s and region %s to project %s."
	fmtAddEnvToProjectComplete = "Linked account %s and region %s project %s."
)

var (
	errNamedProfilesNotFound = fmt.Errorf("no named AWS profiles found, run %s first please", color.HighlightCode("aws configure"))
)

type initEnvVars struct {
	*GlobalOpts
	EnvName      string // Name of the environment.
	EnvProfile   string // AWS profile used to create an environment.
	IsProduction bool   // Marks the environment as "production" to create it with additional guardrails.
}

type initEnvOpts struct {
	initEnvVars

	// Interfaces to interact with dependencies.
	projectGetter archer.ProjectGetter
	envCreator    archer.EnvironmentCreator
	envDeployer   deployer
	projDeployer  deployer
	identity      identityService
	envIdentity   identityService
	profileConfig profileNames
	prog          progress

	// initialize profile-specific env clients
	initProfileClients func(*initEnvOpts) error
}

var initEnvProfileClients = func(o *initEnvOpts) error {
	profileSess, err := session.NewProvider().FromProfile(o.EnvProfile)
	if err != nil {
		return fmt.Errorf("create session from profile %s: %w", o.EnvProfile, err)
	}
	o.envIdentity = identity.New(profileSess)
	o.envDeployer = deploycfn.New(profileSess)
	return nil
}

func newInitEnvOpts(vars initEnvVars) (*initEnvOpts, error) {
	store, err := store.New()
	if err != nil {
		return nil, err
	}
	sessProvider := session.NewProvider()
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
		projectGetter:      store,
		envCreator:         store,
		projDeployer:       deploycfn.New(defaultSession),
		identity:           identity.New(defaultSession),
		profileConfig:      cfg,
		prog:               termprogress.NewSpinner(),
		initProfileClients: initEnvProfileClients,
	}, nil
}

// Validate returns an error if the values passed by the user are invalid.
func (o *initEnvOpts) Validate() error {
	if o.EnvName != "" {
		if err := validateEnvironmentName(o.EnvName); err != nil {
			return err
		}
	}
	if o.ProjectName() == "" {
		return fmt.Errorf("no project found: run %s or %s into your workspace please", color.HighlightCode("project init"), color.HighlightCode("cd"))
	}
	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *initEnvOpts) Ask() error {
	if err := o.askEnvName(); err != nil {
		return err
	}
	return o.askEnvProfile()
}

// Execute deploys a new environment with CloudFormation and adds it to SSM.
func (o *initEnvOpts) Execute() error {
	project, err := o.projectGetter.GetProject(o.ProjectName())
	if err != nil {
		// Ensure the project actually exists before we do a deployment.
		return err
	}
	if project.RequiresDNSDelegation() {
		if err := o.delegateDNSFromProject(project); err != nil {
			return fmt.Errorf("granting DNS permissions: %w", err)
		}
	}
	if err = o.initProfileClients(o); err != nil {
		return err
	}

	// 1. Start creating the CloudFormation stack for the environment.
	if err := o.deployEnv(project); err != nil {
		return err
	}

	// 2. Get the environment
	env, err := o.retrieveEnvironment()
	if err != nil {
		return err
	}

	// 3. Add the stack set instance to the project stackset.
	if err := o.addToStackset(project, env); err != nil {
		return err
	}

	// 4. Store the environment in SSM.
	if err := o.envCreator.CreateEnvironment(env); err != nil {
		return fmt.Errorf("store environment: %w", err)
	}
	log.Successf("Created environment %s in region %s under project %s.\n",
		color.HighlightUserInput(env.Name), color.HighlightResource(env.Region), color.HighlightResource(env.Project))
	return nil
}

func (o *initEnvOpts) deployEnv(project *archer.Project) error {
	caller, err := o.identity.Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}
	deployEnvInput := &deploy.CreateEnvironmentInput{
		Name:                     o.EnvName,
		Project:                  o.ProjectName(),
		Prod:                     o.IsProduction,
		PublicLoadBalancer:       true, // TODO: configure this based on user input or application Type needs?
		ToolsAccountPrincipalARN: caller.RootUserARN,
		ProjectDNSName:           project.Domain,
	}

	o.prog.Start(fmt.Sprintf(fmtDeployEnvStart, color.HighlightUserInput(o.EnvName)))
	if err := o.envDeployer.DeployEnvironment(deployEnvInput); err != nil {
		var existsErr *cloudformation.ErrStackAlreadyExists
		if errors.As(err, &existsErr) {
			// Do nothing if the stack already exists.
			o.prog.Stop("")
			log.Successf("CloudFormation stack for env %s already exists under project %s! Do nothing.\n",
				color.HighlightUserInput(o.EnvName), color.HighlightResource(o.ProjectName()))
			return nil
		}
		o.prog.Stop(log.Serrorf(fmtDeployEnvFailed, color.HighlightUserInput(o.EnvName)))
		return err
	}

	// Display updates while the deployment is happening.
	o.prog.Start(fmt.Sprintf(fmtStreamEnvStart, color.HighlightUserInput(o.EnvName)))
	stackEvents, responses := o.envDeployer.StreamEnvironmentCreation(deployEnvInput)
	for stackEvent := range stackEvents {
		o.prog.Events(o.humanizeEnvironmentEvents(stackEvent))
	}
	resp := <-responses
	if resp.Err != nil {
		o.prog.Stop(log.Serrorf(fmtStreamEnvFailed, color.HighlightUserInput(o.EnvName)))
		return resp.Err
	}
	o.prog.Stop(log.Ssuccessf(fmtStreamEnvComplete, color.HighlightUserInput(o.EnvName)))

	return nil
}

func (o *initEnvOpts) retrieveEnvironment() (*archer.Environment, error) {
	envStack, err := o.envDeployer.EnvStack(o.ProjectName(), o.EnvName)
	if err != nil {
		return nil, fmt.Errorf("retrieve CloudFormation stack for env %s: %w", o.EnvName, err)
	}
	conf := stack.NewEnvStackConfig(&deploy.CreateEnvironmentInput{
		Project: o.ProjectName(),
		Name:    o.EnvName,
	})
	env, err := conf.ToEnv(envStack)
	if err != nil {
		return nil, fmt.Errorf("construct env struct out of CloudFormation stack: %w", err)
	}
	return env, nil
}

func (o *initEnvOpts) addToStackset(project *archer.Project, env *archer.Environment) error {
	o.prog.Start(fmt.Sprintf(fmtAddEnvToProjectStart, color.HighlightResource(env.AccountID), color.HighlightResource(env.Region), color.HighlightUserInput(o.ProjectName())))
	if err := o.projDeployer.AddEnvToProject(project, env); err != nil {
		o.prog.Stop(log.Serrorf(fmtAddEnvToProjectFailed, color.HighlightResource(env.AccountID), color.HighlightResource(env.Region), color.HighlightUserInput(o.ProjectName())))
		return fmt.Errorf("deploy env %s to project %s: %w", env.Name, project.Name, err)
	}
	o.prog.Stop(log.Ssuccessf(fmtAddEnvToProjectComplete, color.HighlightResource(env.AccountID), color.HighlightResource(env.Region), color.HighlightUserInput(o.ProjectName())))

	return nil
}

func (o *initEnvOpts) delegateDNSFromProject(project *archer.Project) error {
	envAccount, err := o.envIdentity.Get()
	if err != nil {
		return fmt.Errorf("getting environment account ID for DNS Delegation: %w", err)
	}

	// By default, our DNS Delegation permits same account delegation.
	if envAccount.Account == project.AccountID {
		return nil
	}

	o.prog.Start(fmt.Sprintf(fmtDNSDelegationStart, color.HighlightUserInput(envAccount.Account)))
	if err := o.projDeployer.DelegateDNSPermissions(project, envAccount.Account); err != nil {
		o.prog.Stop(log.Serrorf(fmtDNSDelegationFailed, color.HighlightUserInput(envAccount.Account)))
		return err
	}
	o.prog.Stop(log.Ssuccessf(fmtDNSDelegationComplete, color.HighlightUserInput(envAccount.Account)))
	return nil
}

func (o *initEnvOpts) askEnvName() error {
	if o.EnvName != "" {
		return nil
	}

	envName, err := o.prompt.Get(envInitNamePrompt, envInitNameHelpPrompt, validateEnvironmentName)
	if err != nil {
		return fmt.Errorf("prompt to get environment name: %w", err)
	}
	o.EnvName = envName
	return nil
}

func (o *initEnvOpts) askEnvProfile() error {
	if o.EnvProfile != "" {
		return nil
	}

	names := o.profileConfig.Names()
	if len(names) == 0 {
		return errNamedProfilesNotFound
	}

	profile, err := o.prompt.SelectOne(
		fmt.Sprintf(fmtEnvInitProfilePrompt, color.HighlightUserInput(o.EnvName)),
		envInitProfileHelpPrompt,
		names)
	if err != nil {
		return fmt.Errorf("prompt to get the profile name: %w", err)
	}
	o.EnvProfile = profile
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
		Short: "Creates a new environment in your project.",
		Example: `
  Creates a test environment in your "default" AWS profile.
  /code $ ecs-preview env init --name test --profile default

  Creates a prod-iad environment using your "prod-admin" AWS profile.
  /code $ ecs-preview env init --name prod-iad --profile prod-admin --prod`,
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
	cmd.Flags().StringVarP(&vars.EnvName, nameFlag, nameFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.EnvProfile, profileFlag, "", profileFlagDescription)
	cmd.Flags().BoolVar(&vars.IsProduction, prodEnvFlag, false, prodEnvFlagDescription)
	return cmd
}

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
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
	fmtStreamEnvStart          = "Creating the infrastructure for the %s environment."
	fmtStreamEnvFailed         = "Failed to create the infrastructure for the %s environment."
	fmtStreamEnvComplete       = "Created the infrastructure for the %s environment."
	fmtAddEnvToProjectStart    = "Linking account %s and region %s to project %s."
	fmtAddEnvToProjectFailed   = "Failed to link account %s and region %s to project %s."
	fmtAddEnvToProjectComplete = "Linked account %s and region %s project %s."
)

// InitEnvOpts contains the fields to collect for adding an environment.
type InitEnvOpts struct {
	// Flags set by the user.
	EnvName      string // Name of the environment.
	EnvProfile   string // AWS profile used to create an environment.
	IsProduction bool   // Marks the environment as "production" to create it with additional guardrails.

	// Interfaces to interact with dependencies.
	projectGetter archer.ProjectGetter
	envCreator    archer.EnvironmentCreator
	envDeployer   deployer
	projDeployer  deployer
	identity      identityService
	envIdentity   identityService
	prog          progress

	*GlobalOpts
}

// Ask asks for fields that are required but not passed in.
func (opts *InitEnvOpts) Ask() error {
	if opts.EnvName == "" {
		envName, err := opts.prompt.Get(envInitNamePrompt, envInitNameHelpPrompt, validateEnvironmentName)
		if err != nil {
			return fmt.Errorf("prompt to get environment name: %w", err)
		}
		opts.EnvName = envName
	}
	if opts.EnvProfile == "" {
		profile, err := opts.prompt.Get(
			fmt.Sprintf(fmtEnvInitProfilePrompt, color.HighlightUserInput(opts.EnvName)),
			envInitProfileHelpPrompt,
			validateEnvironmentName,
			prompt.WithDefaultInput("default"))
		if err != nil {
			return fmt.Errorf("prompt to get the profile name: %w", err)
		}
		opts.EnvProfile = profile
	}
	return nil
}

// Validate returns an error if the values passed by the user are invalid.
func (opts *InitEnvOpts) Validate() error {
	if err := validateEnvironmentName(opts.EnvName); err != nil {
		return err
	}
	if opts.EnvProfile == "" {
		return fmt.Errorf("profile name cannot be empty, please provide a value with %s", color.HighlightCode(profileFlag))
	}
	if opts.ProjectName() == "" {
		return errors.New("no project found, run `project init` first please")
	}
	return nil
}

// Execute deploys a new environment with CloudFormation and adds it to SSM.
func (opts *InitEnvOpts) Execute() error {
	project, err := opts.projectGetter.GetProject(opts.ProjectName())
	if err != nil {
		// Ensure the project actually exists before we do a deployment.
		return err
	}
	caller, err := opts.identity.Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}

	// 1. Start creating the CloudFormation stack for the environment.
	deployEnvInput := &deploy.CreateEnvironmentInput{
		Name:                     opts.EnvName,
		Project:                  opts.ProjectName(),
		Prod:                     opts.IsProduction,
		PublicLoadBalancer:       true, // TODO: configure this based on user input or application Type needs?
		ToolsAccountPrincipalARN: caller.RootUserARN,
		ProjectDNSName:           project.Domain,
	}

	if project.RequiresDNSDelegation() {
		if err := opts.delegateDNSFromProject(project); err != nil {
			return fmt.Errorf("granting DNS permissions: %w", err)
		}
	}

	opts.prog.Start(fmt.Sprintf(fmtDeployEnvStart, color.HighlightUserInput(opts.EnvName)))

	if err := opts.envDeployer.DeployEnvironment(deployEnvInput); err != nil {
		var existsErr *cloudformation.ErrStackAlreadyExists
		if errors.As(err, &existsErr) {
			// Do nothing if the stack already exists.
			opts.prog.Stop("")
			log.Successf("Environment %s already exists under project %s! Do nothing.\n",
				color.HighlightUserInput(opts.EnvName), color.HighlightResource(opts.ProjectName()))
			return nil
		}
		opts.prog.Stop(log.Serrorf(fmtDeployEnvFailed, color.HighlightUserInput(opts.EnvName)))
		return err
	}

	// 2. Display updates while the deployment is happening.
	opts.prog.Start(fmt.Sprintf(fmtStreamEnvStart, color.HighlightUserInput(opts.EnvName)))
	stackEvents, responses := opts.envDeployer.StreamEnvironmentCreation(deployEnvInput)
	for stackEvent := range stackEvents {
		opts.prog.Events(opts.humanizeEnvironmentEvents(stackEvent))
	}
	resp := <-responses
	if resp.Err != nil {
		opts.prog.Stop(log.Serrorf(fmtStreamEnvFailed, color.HighlightUserInput(opts.EnvName)))
		return resp.Err
	}
	opts.prog.Stop(log.Ssuccessf(fmtStreamEnvComplete, color.HighlightUserInput(opts.EnvName)))

	// 3. Add the stack set instance to the project stackset.
	opts.prog.Start(fmt.Sprintf(fmtAddEnvToProjectStart, color.HighlightResource(resp.Env.AccountID), color.HighlightResource(resp.Env.Region), color.HighlightUserInput(opts.ProjectName())))
	if err := opts.projDeployer.AddEnvToProject(project, resp.Env); err != nil {
		opts.prog.Stop(log.Serrorf(fmtAddEnvToProjectFailed, color.HighlightResource(resp.Env.AccountID), color.HighlightResource(resp.Env.Region), color.HighlightUserInput(opts.ProjectName())))
		return fmt.Errorf("deploy env %s to project %s: %w", resp.Env.Name, project.Name, err)
	}
	opts.prog.Stop(log.Ssuccessf(fmtAddEnvToProjectComplete, color.HighlightResource(resp.Env.AccountID), color.HighlightResource(resp.Env.Region), color.HighlightUserInput(opts.ProjectName())))

	// 4. Store the environment in SSM.
	if err := opts.envCreator.CreateEnvironment(resp.Env); err != nil {
		return fmt.Errorf("store environment: %w", err)
	}
	log.Successf("Created environment %s in region %s under project %s.\n",
		color.HighlightUserInput(resp.Env.Name), color.HighlightResource(resp.Env.Region), color.HighlightResource(resp.Env.Project))
	return nil
}

func (opts *InitEnvOpts) delegateDNSFromProject(project *archer.Project) error {
	envAccount, err := opts.envIdentity.Get()
	if err != nil {
		return fmt.Errorf("getting environment account ID for DNS Delegation: %w", err)
	}

	// By default, our DNS Delegation permits same account delegation.
	if envAccount.Account == project.AccountID {
		return nil
	}

	opts.prog.Start(fmt.Sprintf(fmtDNSDelegationStart, color.HighlightUserInput(envAccount.Account)))

	if err := opts.projDeployer.DelegateDNSPermissions(project, envAccount.Account); err != nil {
		opts.prog.Stop(log.Serrorf(fmtDNSDelegationFailed, color.HighlightUserInput(envAccount.Account)))
		return err
	}
	return nil
}

func (opts *InitEnvOpts) humanizeEnvironmentEvents(resourceEvents []deploy.ResourceEvent) []termprogress.TabRow {
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
func (opts *InitEnvOpts) RecommendedActions() []string {
	return nil
}

// BuildEnvInitCmd builds the command for adding an environment.
func BuildEnvInitCmd() *cobra.Command {
	opts := InitEnvOpts{
		IsProduction: false,
		prog:         termprogress.NewSpinner(),
		GlobalOpts:   NewGlobalOpts(),
	}

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Creates a new environment in your project.",
		Example: `
  Creates a test environment in your "default" AWS profile.
  /code $ archer env init --name test --profile default

  Creates a prod-iad environment using your "prod-admin" AWS profile.
  /code $ archer env init --name prod-iad --profile prod-admin --prod`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}

			store, err := store.New()
			if err != nil {
				return err
			}
			profileSess, err := session.FromProfile(opts.EnvProfile)
			if err != nil {
				return err
			}
			defaultSession, err := session.Default()
			if err != nil {
				return err
			}
			opts.envCreator = store
			opts.projectGetter = store
			opts.envDeployer = cloudformation.New(profileSess)
			opts.projDeployer = cloudformation.New(defaultSession)
			opts.identity = identity.New(defaultSession)
			opts.envIdentity = identity.New(profileSess)
			return nil
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			return opts.Execute()
		}),
	}
	cmd.Flags().StringVarP(&opts.EnvName, nameFlag, nameFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&opts.EnvProfile, profileFlag, "", profileFlagDescription)
	cmd.Flags().BoolVar(&opts.IsProduction, prodEnvFlag, opts.IsProduction, prodEnvFlagDescription)
	return cmd
}

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
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store/ssm"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/spf13/cobra"
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
	envDeployer   environmentDeployer
	identity      identityService

	prog   progress
	prompt prompter

	globalOpts // Embed global options.
}

// Ask asks for fields that are required but not passed in.
func (opts *InitEnvOpts) Ask() error {
	if opts.EnvName == "" {
		envName, err := opts.prompt.Get(
			"What is your environment's name?",
			"A unique identifier for an environment (e.g. dev, test, prod)",
			validateEnvironmentName,
		)
		if err != nil {
			return fmt.Errorf("failed to get environment name: %w", err)
		}
		opts.EnvName = envName
	}
	return nil
}

// Validate returns an error if the values passed by the user are invalid.
func (opts *InitEnvOpts) Validate() error {
	if opts.EnvName != "" {
		if err := validateEnvironmentName(opts.EnvName); err != nil {
			return err
		}
	}
	if opts.projectName == "" {
		return errors.New("no project found, run `project init` first please")
	}
	return nil
}

// Execute deploys a new environment with CloudFormation and adds it to SSM.
func (opts *InitEnvOpts) Execute() error {
	// Ensure the project actually exists before we do a deployment.
	if _, err := opts.projectGetter.GetProject(opts.projectName); err != nil {
		return err
	}

	caller, err := opts.identity.Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}

	deployEnvInput := &deploy.CreateEnvironmentInput{
		Name:                     opts.EnvName,
		Project:                  opts.projectName,
		Prod:                     opts.IsProduction,
		PublicLoadBalancer:       true, // TODO: configure this based on user input or application Type needs?
		ToolsAccountPrincipalARN: caller.ARN,
	}

	opts.prog.Start(fmt.Sprintf("Proposing infrastructure changes for the %s environment", color.HighlightUserInput(opts.EnvName)))
	if err := opts.envDeployer.DeployEnvironment(deployEnvInput); err != nil {
		var existsErr *cloudformation.ErrStackAlreadyExists
		if errors.As(err, &existsErr) {
			// Do nothing if the stack already exists.
			opts.prog.Stop("")
			log.Successf("Environment %s already exists under project %s! Do nothing.\n",
				color.HighlightUserInput(opts.EnvName), color.HighlightResource(opts.projectName))
			return nil
		}
		opts.prog.Stop(fmt.Sprintf("%s Failed to accept changes for the %s environment", color.ErrorMarker, color.HighlightUserInput(opts.EnvName)))
		return err
	}
	opts.prog.Start(fmt.Sprintf("Creating the infrastructure for the %s environment", color.HighlightUserInput(opts.EnvName)))
	stackEvents, responses := opts.envDeployer.StreamEnvironmentCreation(deployEnvInput)
	for stackEvent := range stackEvents {
		opts.prog.Events(opts.humanizeEnvironmentEvents(stackEvent))
	}
	resp := <-responses
	if resp.Err != nil {
		opts.prog.Stop(fmt.Sprintf("%s Failed to create the infrastructure for the %s environment", color.ErrorMarker, color.HighlightUserInput(opts.EnvName)))
		return resp.Err
	}
	opts.prog.Stop(fmt.Sprintf("%s Created the infrastructure for the %s environment", color.SuccessMarker, color.HighlightUserInput(opts.EnvName)))
	if err := opts.envCreator.CreateEnvironment(resp.Env); err != nil {
		return fmt.Errorf("store environment: %w", err)
	}
	log.Successf("Created environment %s in region %s under project %s.\n",
		color.HighlightUserInput(resp.Env.Name), color.HighlightResource(resp.Env.Region), color.HighlightResource(resp.Env.Project))
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
		textNATGateway: func(event deploy.Resource) bool {
			return event.Type == "AWS::EC2::EIP" ||
				event.Type == "AWS::EC2::NatGateway"
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
		textNATGateway:      4,
		textRouteTables:     10,
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
		EnvProfile: "default",
		prog:       termprogress.NewSpinner(),
		prompt:     prompt.New(),
		globalOpts: newGlobalOpts(),
	}

	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Create a new environment in your project.",
		Example: `
  Create a test environment in your "default" AWS profile.
  /code $ archer env init test

  Create a prod-iad environment using your "prod-admin" AWS profile.
  /code $ archer env init prod-iad --profile prod-admin --prod`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.EnvName = args[0]
			}
			store, err := ssm.NewStore()
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
			opts.identity = identity.New(defaultSession)
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			return opts.Execute()
		},
	}
	cmd.Flags().StringVar(&opts.EnvProfile, "profile", "default", "Name of the profile. Defaults to \"default\".")
	cmd.Flags().BoolVar(&opts.IsProduction, "prod", false, "If the environment contains production services.")

	return cmd
}

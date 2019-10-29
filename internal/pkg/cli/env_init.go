// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store/ssm"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/spinner"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	envDeployer   archer.EnvironmentDeployer
	identity      identityService
	prog          progress
	prompt        prompter
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
	if viper.GetString(projectFlag) == "" {
		return errors.New("no project found, run `project init` first")
	}
	return nil
}

// Execute deploys a new environment with CloudFormation and adds it to SSM.
func (opts *InitEnvOpts) Execute() error {
	// Ensure the project actually exists before we do a deployment.
	if _, err := opts.projectGetter.GetProject(viper.GetString(projectFlag)); err != nil {
		return err
	}

	identity, err := opts.identity.Get()
	if err != nil {
		return fmt.Errorf("get identity: %w", err)
	}

	deployEnvInput := &deploy.DeployEnvironmentInput{
		Name:                     opts.EnvName,
		Project:                  viper.GetString(projectFlag),
		Prod:                     opts.IsProduction,
		PublicLoadBalancer:       true, // TODO: configure this based on user input or application Type needs?
		ToolsAccountPrincipalARN: identity.ARN,
	}

	opts.prog.Start("Preparing deployment...")
	if err := opts.envDeployer.DeployEnvironment(deployEnvInput); err != nil {
		var existsErr *cloudformation.ErrStackAlreadyExists
		if errors.As(err, &existsErr) {
			// Do nothing if the stack already exists.
			opts.prog.Stop("Done!")
			log.Successf("Environment %s already exists under project %s! Do nothing.\n",
				color.HighlightUserInput(opts.EnvName), color.HighlightResource(viper.GetString(projectFlag)))
			return nil
		}
		opts.prog.Stop("Error!")
		return err
	}
	opts.prog.Stop("Done!")
	opts.prog.Start("Deploying env...")
	env, err := opts.envDeployer.WaitForEnvironmentCreation(deployEnvInput)
	if err != nil {
		opts.prog.Stop("Error!")
		return err
	}
	if err := opts.envCreator.CreateEnvironment(env); err != nil {
		opts.prog.Stop("Error!")
		return err
	}
	opts.prog.Stop("Done!")
	log.Successf("Created environment %s in region %s under project %s.\n",
		color.HighlightUserInput(env.Name), color.HighlightResource(env.Region), color.HighlightResource(env.Project))
	return nil
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (opts *InitEnvOpts) RecommendedActions() []string {
	return nil
}

// BuildEnvInitCmd builds the command for adding an environment.
func BuildEnvInitCmd() *cobra.Command {
	opts := InitEnvOpts{
		EnvProfile: "default",
		prog:       spinner.New(),
		prompt:     prompt.New(),
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

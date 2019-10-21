// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store/ssm"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/spinner"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// InitEnvOpts contains the fields to collect for adding an environment.
type InitEnvOpts struct {
	ProjectName string
	EnvName     string
	EnvProfile  string
	Production  bool

	manager       archer.EnvironmentCreator
	projectGetter archer.ProjectGetter
	deployer      archer.EnvironmentDeployer
	prog          progress
	prompter      prompter
}

// Ask asks for fields that are required but not passed in.
func (opts *InitEnvOpts) Ask() error {
	if opts.ProjectName == "" {
		projectName, err := opts.prompter.Get(
			"What is your project's name?",
			"A project groups all of your environments together.",
			validateProjectName)

		if err != nil {
			return fmt.Errorf("failed to get project name: %w", err)
		}

		opts.ProjectName = projectName
	}
	if opts.EnvName == "" {
		envName, err := opts.prompter.Get(
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

// Execute deploys a new environment with CloudFormation and adds it to SSM.
func (opts *InitEnvOpts) Execute() error {
	// Ensure the project actually exists before we do a deployment.
	if _, err := opts.projectGetter.GetProject(opts.ProjectName); err != nil {
		return err
	}

	deployEnvInput := &archer.DeployEnvironmentInput{
		Name:               opts.EnvName,
		Project:            opts.ProjectName,
		Prod:               opts.Production,
		PublicLoadBalancer: true, // TODO: configure this based on user input or application Type needs?
	}

	opts.prog.Start("Preparing deployment...")
	if err := opts.deployer.DeployEnvironment(deployEnvInput); err != nil {
		var existsErr *cloudformation.ErrStackAlreadyExists
		if errors.As(err, &existsErr) {
			// Do nothing if the stack already exists.
			opts.prog.Stop("Done!")
			fmt.Printf("The environment %s already exists under project %s.\n", opts.EnvName, opts.ProjectName)
			return nil
		}
		opts.prog.Stop("Error!")
		return err
	}
	opts.prog.Stop("Done!")
	opts.prog.Start("Deploying env...")
	env, err := opts.deployer.WaitForEnvironmentCreation(deployEnvInput)
	if err != nil {
		opts.prog.Stop("Error!")
		return err
	}
	if err := opts.manager.CreateEnvironment(env); err != nil {
		opts.prog.Stop("Error!")
		return err
	}
	opts.prog.Stop("Done!")
	return nil
}

// BuildEnvInitCmd builds the command for adding an environment.
func BuildEnvInitCmd() *cobra.Command {
	opts := InitEnvOpts{
		EnvProfile: "default",
		prog:       spinner.New(),
		prompter:   prompt.New(),
	}

	cmd := &cobra.Command{
		Use:   "init [name]",
		Short: "Create a new environment in your project.",
		Example: `
  Create a test environment in your "default" AWS profile
  $ archer env init test

  Create a prod-iad environment using your "prod-admin" AWS profile
  $ archer env init prod-iad --profile prod-admin --prod`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.EnvName = args[0]
			}

			opts.ProjectName = viper.GetString("project")
			// If the project flag or env name isn't passed in, ask the user for them.
			if err := opts.Ask(); err != nil {
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO: pass in configured session to ssm.NewStore?
			s, err := ssm.NewStore()
			if err != nil {
				return err
			}
			opts.manager = s
			opts.projectGetter = s

			sess, err := session.FromProfile(opts.EnvProfile)
			if err != nil {
				return err
			}

			opts.deployer = cloudformation.New(sess)
			return opts.Execute()
		},
	}
	cmd.Flags().StringVar(&opts.EnvProfile, "profile", "", "Name of the profile. Defaults to \"default\".")
	cmd.Flags().BoolVar(&opts.Production, "prod", false, "If the environment contains production services.")

	return cmd
}

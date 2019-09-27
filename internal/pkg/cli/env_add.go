// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/deploy/cloudformation"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/store/ssm"
	spin "github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/term/spinner"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// AddEnvOpts contains the fields to collect for adding an environment.
type AddEnvOpts struct {
	ProjectName string `survey:"project"`
	EnvName     string `survey:"env"`
	EnvProfile  string
	Production  bool `survey:"prod"`

	prompt        terminal.Stdio
	manager       archer.EnvironmentCreator
	projectGetter archer.ProjectGetter
	deployer      archer.EnvironmentDeployer
	spinner       spinner
}

// Ask asks for fields that are required but not passed in.
func (opts *AddEnvOpts) Ask() error {
	var qs []*survey.Question
	if opts.ProjectName == "" {
		qs = append(qs, &survey.Question{
			Name: "project",
			Prompt: &survey.Input{
				Message: "What is your project's name?",
				Help:    "A project groups all of your environments together.",
			},
			Validate: validateProjectName,
		})
	}
	if opts.EnvName == "" {
		qs = append(qs, &survey.Question{
			Name: "env",
			Prompt: &survey.Input{
				Message: "What is your environment's name?",
			},
			Validate: survey.Required,
		})
	}
	return survey.Ask(qs, opts, survey.WithStdio(opts.prompt.In, opts.prompt.Out, opts.prompt.Err))
}

// Execute deploys a new environment with CloudFormation and adds it to SSM.
func (opts *AddEnvOpts) Execute() error {
	// Ensure the project actually exists before we do a deployment.
	if _, err := opts.projectGetter.GetProject(opts.ProjectName); err != nil {
		return err
	}

	env := archer.Environment{
		Name:               opts.EnvName,
		Project:            opts.ProjectName,
		AccountID:          "1234",
		Region:             "1234",
		Prod:               opts.Production,
		PublicLoadBalancer: true, // TODO: configure this based on user input or application Type needs?
	}

	if err := opts.deployer.DeployEnvironment(env); err != nil {
		return err
	}

	opts.spinner.Start("Deploying env...")

	if err := opts.deployer.Wait(env); err != nil {
		return err
	}

	opts.spinner.Stop("Done!")

	if err := opts.manager.CreateEnvironment(&env); err != nil {
		return err
	}
	return nil
}

// BuildEnvAddCmd builds the command for adding an environment.
func BuildEnvAddCmd() *cobra.Command {
	opts := AddEnvOpts{
		EnvProfile: "default",
		prompt: terminal.Stdio{
			In:  os.Stdin,
			Out: os.Stderr,
			Err: os.Stderr,
		},
		spinner: spin.New(),
	}

	cmd := &cobra.Command{
		Use:   "add [name]",
		Short: "Deploy a new environment to your project",
		Example: `
  Create a test environment in your "default" AWS profile
  $ archer env add test

  Create a prod-iad environment using your "prod-admin" AWS profile
  $ archer env add prod-iad --profile prod-admin --prod`,
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

			// TODO: create this session elsewhere
			sess, err := session.NewSessionWithOptions(session.Options{
				SharedConfigState: session.SharedConfigEnable,
			})
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

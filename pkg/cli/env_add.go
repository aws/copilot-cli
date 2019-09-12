// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/deploy/cloudformation"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/store/ssm"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/spf13/cobra"
)

// AddEnvOpts contains the fields to collect for adding an environment
type AddEnvOpts struct {
	ProjectName string `survey:"project"`
	EnvName     string `survey:"env"`
	EnvProfile  string
	Production  bool `survey:"prod"`
	Prompt      terminal.Stdio
	manager     archer.EnvironmentCreator
	deployer    archer.EnvironmentDeployer
}

// Ask asks for fields that are required but not passed in
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
	return survey.Ask(qs, opts, survey.WithStdio(opts.Prompt.In, opts.Prompt.Out, opts.Prompt.Err))
}

// Validate validates the options
func (opts *AddEnvOpts) Validate() error {
	if opts.ProjectName == "" {
		return fmt.Errorf("to add an environment either run the command in your workspace or provide a --project")
	}
	return nil
}

// AddEnvironment does the heavy lifting of adding an environment
func (opts *AddEnvOpts) AddEnvironment() error {
	env := archer.Environment{
		Name:      opts.EnvName,
		Project:   opts.ProjectName,
		AccountID: "1234",
		Region:    "1234",
		Prod:      opts.Production,
	}

	if err := opts.deployer.DeployEnvironment(env, true); err != nil {
		return err
	}

	// TODO: Add distracting spinny thing
	err := opts.deployer.Wait(env)
	if err != nil {
		return err
	}

	if err := opts.manager.CreateEnvironment(&env); err != nil {
		return err
	}

	return nil
}

// BuildEnvAddCmd builds the command for adding an environment
func BuildEnvAddCmd() *cobra.Command {

	opts := AddEnvOpts{
		EnvProfile: "default",
		Prompt: terminal.Stdio{
			In:  os.Stdin,
			Out: os.Stderr,
			Err: os.Stderr,
		},
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
			if err := opts.Validate(); err != nil {
				return err
			}
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

			// TODO: create this session elsewhere
			sess, err := session.NewSessionWithOptions(session.Options{
				SharedConfigState: session.SharedConfigEnable,
			})

			if err != nil {
				return err
			}

			opts.deployer = cloudformation.New(sess)

			return opts.AddEnvironment()
		},
	}
	cmd.Flags().StringVar(&opts.ProjectName, "project", "", "Name of the project (required).")
	cmd.Flags().StringVar(&opts.EnvProfile, "profile", "", "Name of the profile. Defaults to \"default\".")
	cmd.Flags().BoolVar(&opts.Production, "prod", false, "If the environment contains production services.")

	return cmd
}

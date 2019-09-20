// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/store/ssm"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ListEnvOpts contains the fields to collect for listing an environment.
type ListEnvOpts struct {
	ProjectName   string `survey:"project"`
	prompt        terminal.Stdio
	manager       archer.EnvironmentLister
	projectGetter archer.ProjectGetter
}

// Ask asks for fields that are required but not passed in.
func (opts *ListEnvOpts) Ask() error {
	var qs []*survey.Question
	if opts.ProjectName == "" {
		qs = append(qs, &survey.Question{
			Name: "project",
			Prompt: &survey.Input{
				Message: "Which project's environments would you like to list?",
				Help:    "A project groups all of your environments together.",
			},
			Validate: validateProjectName,
		})
	}
	return survey.Ask(qs, opts, survey.WithStdio(opts.prompt.In, opts.prompt.Out, opts.prompt.Err))
}

// Execute lists the environments through the prompt.
func (opts *ListEnvOpts) Execute() error {
	// Ensure the project actually exists before we try to list its environments.
	if _, err := opts.projectGetter.GetProject(opts.ProjectName); err != nil {
		return err
	}

	envs, err := opts.manager.ListEnvironments(opts.ProjectName)
	if err != nil {
		return err
	}

	prodColor := color.New(color.FgYellow, color.Bold).SprintFunc()
	for _, env := range envs {
		if env.Prod {
			fmt.Fprintf(opts.prompt.Out, "%s (prod)\n", prodColor(env.Name))
		} else {
			fmt.Fprintln(opts.prompt.Out, env.Name)
		}
	}

	return nil
}

// BuildEnvListCmd builds the command for listing environments in a project.
func BuildEnvListCmd() *cobra.Command {
	opts := ListEnvOpts{
		prompt: terminal.Stdio{
			In:  os.Stdin,
			Out: os.Stderr,
			Err: os.Stderr,
		},
	}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lists all the environments in a project",
		Example: `
  Lists all the environments for the test project
  $ archer env ls --project test`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			opts.ProjectName = viper.GetString("project")
			return opts.Ask()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ssmStore, err := ssm.NewStore()
			if err != nil {
				return err
			}
			opts.manager = ssmStore
			opts.projectGetter = ssmStore
			return opts.Execute()
		},
	}
	return cmd
}

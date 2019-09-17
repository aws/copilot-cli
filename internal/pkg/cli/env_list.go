// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/store/ssm"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// ListEnvOpts contains the fields to collect for listing an environment.
type ListEnvOpts struct {
	ProjectName string `survey:"project"`
	prompt      terminal.Stdio
	manager     archer.EnvironmentLister
}

// Execute lists the environments through the prompt.
func (opts *ListEnvOpts) Execute() error {
	envs, err := opts.manager.ListEnvironments(opts.ProjectName)
	if err != nil {
		fmt.Fprintf(opts.prompt.Err, "%v\n", err)
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
		RunE: func(cmd *cobra.Command, args []string) error {
			ssmStore, err := ssm.NewStore()
			if err != nil {
				return err
			}
			opts.manager = ssmStore
			return opts.Execute()
		},
	}
	cmd.Flags().StringVar(&opts.ProjectName, "project", "", "Name of the project (required).")
	return cmd
}

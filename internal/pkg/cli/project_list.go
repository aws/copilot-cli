// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/store/ssm"
	"github.com/spf13/cobra"
)

// ListProjectOpts contains the fields to collect for listing a project.
type ListProjectOpts struct {
	prompt  terminal.Stdio
	manager archer.ProjectLister
}

// Execute lists the existing projects to the prompt.
func (opts *ListProjectOpts) Execute() error {
	projects, err := opts.manager.ListProjects()
	if err != nil {
		return err
	}

	for _, proj := range projects {
		fmt.Fprintln(opts.prompt.Out, proj.Name)
	}

	return nil
}

// BuildProjectListCommand builds the command to list existing projects.
func BuildProjectListCommand() *cobra.Command {
	opts := ListProjectOpts{
		prompt: terminal.Stdio{
			In:  os.Stdin,
			Out: os.Stderr,
			Err: os.Stderr,
		},
	}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lists all projects in your account",
		Example: `
  List all the projects in your account and region
  $ archer project ls`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ssmStore, err := ssm.NewStore()
			if err != nil {
				return err
			}
			opts.manager = ssmStore
			return opts.Execute()
		},
	}
	return cmd
}

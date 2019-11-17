// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/spf13/cobra"
)

// ListProjectOpts contains the fields to collect for listing a project.
type ListProjectOpts struct {
	store archer.ProjectLister
	w     io.Writer
}

// Execute lists the existing projects to the prompt.
func (opts *ListProjectOpts) Execute() error {
	projects, err := opts.store.ListProjects()
	if err != nil {
		return err
	}

	for _, proj := range projects {
		fmt.Fprintln(opts.w, proj.Name)
	}

	return nil
}

// BuildProjectListCommand builds the command to list existing projects.
func BuildProjectListCommand() *cobra.Command {
	opts := ListProjectOpts{
		w: os.Stdout,
	}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lists all projects in your account.",
		Example: `
  List all the projects in your account and region
  /code $ archer project ls`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			ssmStore, err := store.New()
			if err != nil {
				return err
			}
			opts.store = ssmStore
			return opts.Execute()
		}),
	}
	return cmd
}

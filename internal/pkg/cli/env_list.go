// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// ListEnvOpts contains the fields to collect for listing an environment.
type ListEnvOpts struct {
	manager       archer.EnvironmentLister
	projectGetter archer.ProjectGetter

	w io.Writer

	*GlobalOpts
}

// Ask asks for fields that are required but not passed in.
func (opts *ListEnvOpts) Ask() error {
	if opts.ProjectName() != "" {
		return nil
	}

	// TODO: Make this a SelectOne prompt based on existing projects?
	projectName, err := opts.prompt.Get(
		"Which project's environments would you like to list?",
		"A project groups all of your environments together.",
		validateProjectName)

	if err != nil {
		return fmt.Errorf("failed to get project name: %w", err)
	}

	opts.projectName = projectName

	return nil
}

// Execute lists the environments through the prompt.
func (opts *ListEnvOpts) Execute() error {
	// Ensure the project actually exists before we try to list its environments.
	if _, err := opts.projectGetter.GetProject(opts.ProjectName()); err != nil {
		return err
	}

	envs, err := opts.manager.ListEnvironments(opts.ProjectName())
	if err != nil {
		return err
	}

	prodColor := color.New(color.FgYellow, color.Bold).SprintFunc()
	for _, env := range envs {
		if env.Prod {
			fmt.Fprintf(opts.w, "%s (prod)\n", prodColor(env.Name))
		} else {
			fmt.Fprintln(opts.w, env.Name)
		}
	}

	return nil
}

// BuildEnvListCmd builds the command for listing environments in a project.
func BuildEnvListCmd() *cobra.Command {
	opts := ListEnvOpts{
		w:          os.Stdout,
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lists all the environments in a project",
		Example: `
  Lists all the environments for the test project
  /code $ archer env ls --project test`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			return opts.Ask()
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			ssmStore, err := store.New()
			if err != nil {
				return err
			}
			opts.manager = ssmStore
			opts.projectGetter = ssmStore
			return opts.Execute()
		}),
	}
	return cmd
}

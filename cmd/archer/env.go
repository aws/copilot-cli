// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/aws/PRIVATE-amazon-ecs-archer/cmd/archer/template"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer/env"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer/project"
	"github.com/spf13/cobra"
)

func buildEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Environment commands",
		Long: `Command for working with environments.
An environment represents a deployment stage.`,
	}
	cmd.AddCommand(buildEnvAddCmd())
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": "Develop 🔧",
	}
	return cmd
}

func buildEnvAddCmd() *cobra.Command {
	opts, err := project.NewAddEnvOpts()
	cmd := &cobra.Command{
		Use:   "add [name]",
		Short: "Deploy a new environment to your project",
		Example: `
  Create a test environment in your "default" AWS profile
  $ archer env add test

  Create a prod-iad environment using your "prod-admin" AWS profile
  $ archer env add prod-iad --profile prod-admin`,
		Args: cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			opts.EnvName = args[0]
			if err != nil {
				return err
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
			proj, err := project.New(opts.ProjectName)
			if err != nil {
				return err
			}
			environ, err := env.New(opts.EnvName, env.WithProfile(opts.EnvProfile))
			if err != nil {
				return err
			}
			return proj.AddEnv(environ)
		},
	}
	cmd.Flags().StringVar(&opts.ProjectName, "project", "", "Name of the project (required).")
	cmd.Flags().StringVar(&opts.EnvProfile, "profile", "", "Name of the profile. Defaults to \"default\".")
	return cmd
}

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/aws/PRIVATE-amazon-ecs-archer/cmd/archer/template"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer/env"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer/project"
	"github.com/pkg/errors"
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
	var projectName, profileName = "", "default"
	cmd := &cobra.Command{
		Use:   "add [name]",
		Short: "Deploy a new environment to your project",
		Example: `
  Create a test environment in your "default" AWS profile
  $ archer env add test

  Create a prod-iad environment using your "prod-admin" AWS profile
  $ archer env add prod-iad --profile prod-admin`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := project.New(projectName)
			if err != nil {
				return err
			}
			if err := proj.Ask(); err != nil {
				return err
			}
			environ, err := env.New(args[0], env.WithProfile(profileName))
			if err != nil {
				return err
			}
			if err := proj.AddEnv(environ); err != nil {
				return errors.Wrapf(err, "failed to add environment %s to project %s", environ.Name, proj.Name)
			}
			return environ.Deploy()
		},
	}
	cmd.Flags().StringVar(&projectName, "project", "", "Name of the project (required).")
	cmd.Flags().StringVar(&profileName, "profile", "", "Name of the profile. Defaults to \"default\".")
	return cmd
}

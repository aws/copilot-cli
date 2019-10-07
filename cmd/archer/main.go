// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package main contains the root command.
package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/aws/amazon-ecs-cli-v2/cmd/archer/template"
	"github.com/aws/amazon-ecs-cli-v2/version"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
)

func init() {
	color.DisableColorBasedOnEnvVar()
	cobra.EnableCommandSorting = false // Maintain the order in which we add commands.
}

func main() {
	cmd := buildRootCmd()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func buildRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archer",
		Short: "Launch and manage applications on Amazon ECS and AWS Fargate",
		Example: `
  Display the help menu for the init command
  $ archer init --help`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// If we don't set a Run() function the help menu doesn't show up.
			// See https://github.com/spf13/cobra/issues/790
		},
		SilenceUsage: true,
	}

	// Sets version for --version flag. Version command gives more detailed
	// version information.
	cmd.Version = version.Version
	// TODO add version template
	// cmd.SetVersionTemplate(template.VersionFlag)

	cmd.AddCommand(cli.BuildVersionCmd())
	cmd.AddCommand(cli.BuildInitCmd())
	cmd.AddCommand(cli.BuildProjCmd())
	cmd.AddCommand(cli.BuildEnvCmd())
	cmd.AddCommand(cli.BuildAppCmd())
	cmd.AddCommand(cli.BuildCompletionCmd())

	cmd.SetUsageTemplate(template.RootUsage)

	return cmd
}

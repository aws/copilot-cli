// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package main contains the root command.
package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/aws/amazon-ecs-cli-v2/cmd/ecs-preview/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/version"
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
		Use:   "ecs-preview",
		Short: "Launch and manage applications on Amazon ECS and AWS Fargate.",
		Example: `
  Display the help menu for the init command
  /code $ ecs-preview init --help`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// If we don't set a Run() function the help menu doesn't show up.
			// See https://github.com/spf13/cobra/issues/790
		},
		SilenceUsage: true,
	}

	// Sets version for --version flag. Version command gives more detailed
	// version information.
	cmd.Version = version.Version
	cmd.SetVersionTemplate("ecs-preview version: {{.Version}}\n")

	// NOTE: Order for each grouping below is significant in that it affects help menu output ordering.
	// "Getting Started" command group.
	cmd.AddCommand(cli.BuildInitCmd())

	// "Develop" command group.
	cmd.AddCommand(cli.BuildProjCmd())
	cmd.AddCommand(cli.BuildEnvCmd())
	cmd.AddCommand(cli.BuildAppCmd())

	// "Secrets" command group.
	cmd.AddCommand(cli.BuildSecretCmd())

	// "Settings" command group.
	cmd.AddCommand(cli.BuildVersionCmd())
	cmd.AddCommand(cli.BuildCompletionCmd(cmd))

	// "Storage" command group.
	cmd.AddCommand(cli.BuildDatabaseCmd())

	// "Release" command group.
	cmd.AddCommand(cli.BuildPipelineCmd())
	cmd.AddCommand(cli.BuildEndpointCmd())

	cmd.SetUsageTemplate(template.RootUsage)

	return cmd
}

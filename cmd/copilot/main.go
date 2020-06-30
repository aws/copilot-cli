// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package main contains the root command.
package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/version"
)

func init() {
	color.DisableColorBasedOnEnvVar()
	cobra.EnableCommandSorting = false // Maintain the order in which we add commands.
}

func main() {
	cmd := buildRootCmd()
	if err := cmd.Execute(); err != nil {
		log.Errorln(err.Error())
		os.Exit(1)
	}
}

func buildRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "copilot",
		Short: shortDescription,
		Example: `
  Displays the help menu for the "init" command.
  /code $ copilot init --help`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// If we don't set a Run() function the help menu doesn't show up.
			// See https://github.com/spf13/cobra/issues/790
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Sets version for --version flag. Version command gives more detailed
	// version information.
	cmd.Version = version.Version
	cmd.SetVersionTemplate("copilot version: {{.Version}}\n")

	// NOTE: Order for each grouping below is significant in that it affects help menu output ordering.
	// "Getting Started" command group.
	cmd.AddCommand(cli.BuildInitCmd())
	cmd.AddCommand(cli.BuildDocsCmd())

	// "Develop" command group.
	cmd.AddCommand(cli.BuildAppCmd())
	cmd.AddCommand(cli.BuildEnvCmd())
	cmd.AddCommand(cli.BuildSvcCmd())

	// "Addons" command group
	//cmd.AddCommand(cli.BuildStorageCmd())

	// "Operational" command group.

	// "Settings" command group.
	cmd.AddCommand(cli.BuildVersionCmd())
	cmd.AddCommand(cli.BuildCompletionCmd(cmd))

	// "Release" command group.
	cmd.AddCommand(cli.BuildPipelineCmd())
	cmd.AddCommand(cli.BuildDeployCmd())
	cmd.SetUsageTemplate(template.RootUsage)

	return cmd
}

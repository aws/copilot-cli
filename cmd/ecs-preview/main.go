// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package main contains the root command.
package main

import (
	"fmt"
	"os"

	"github.com/aws/amazon-ecs-cli-v2/cmd/ecs-preview/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/datadotworld/dev-tools/go-common/_version"
	"github.com/datadotworld/dev-tools/go-common/commonutil"
	"github.com/datadotworld/dev-tools/go-common/updater"
	"github.com/datadotworld/dev-tools/go-common/updater/artifactory"
	"github.com/spf13/cobra"
)

const (
	verboseFlag = "verbose"
)

func init() {
	color.DisableColorBasedOnEnvVar()
	cobra.EnableCommandSorting = false // Maintain the order in which we add commands.
}

func main() {
	if err := os.Setenv("AWS_PROFILE", "run-admin"); err != nil {
		os.Exit(2)
	}
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
			disableUpdate, _ := cmd.Flags().GetBool(updater.DisableUpdateFlagName)
			forceUpdate, _ := cmd.Flags().GetBool(updater.ForceUpdateFlagName)
			verbose, _ := cmd.Flags().GetBool(verboseFlag)

			if verbose {
				commonutil.SetLogDebug()
			}

			err := updater.DoUpdate(
				artifactory.New("run"),
				disableUpdate, forceUpdate)
			if err != nil {
				_ = fmt.Errorf("update attempt failed - %w", err)
			}
		},
		SilenceUsage: true,
	}

	cmd.PersistentFlags().Bool(updater.ForceUpdateFlagName, false, "Force an update")
	cmd.PersistentFlags().Bool(updater.DisableUpdateFlagName, false,"Disable automatic updates")
	cmd.PersistentFlags().Bool(verboseFlag, false,"Enable verbose output")

	// Sets version for --version flag. Version command gives more detailed
	// version information.
	cmd.Version = version.FullVersion()
	cmd.SetVersionTemplate("ecs-preview version: {{.Version}}\n")

	// NOTE: Order for each grouping below is significant in that it affects help menu output ordering.
	// "Getting Started" command group.
	cmd.AddCommand(cli.BuildInitCmd())

	// "Develop" command group.
	cmd.AddCommand(cli.BuildProjCmd())
	cmd.AddCommand(cli.BuildEnvCmd())
	cmd.AddCommand(cli.BuildAppCmd())
	cmd.AddCommand(cli.BuildVariableCmd())
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

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/cli/template"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/workspace"
)

const (
	// envProjectFlag is the flag for providing the project name to the env commands.
	envProjectFlag = "project"
)

// buildEnvCmd is the top level command for environments
func buildEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Environment commands",
		Long: `Command for working with environments.
An environment represents a deployment stage.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Load the workspace and set the project flag.
			ws, err := workspace.New()
			if err != nil {
				// If there's an error fetching the workspace, fall back to requiring
				// the project flag be set.
				log.Println(err.Error())
				return
			}

			summary, err := ws.Summary()
			if err != nil {
				// If there's an error reading from the workspace, fall back to requiring
				// the project flag be set.
				log.Println(err.Error())
				return
			}
			viper.SetDefault(envProjectFlag, summary.ProjectName)

		},
	}

	// The project flag is available to all subcommands through viper.GetString("project")
	cmd.PersistentFlags().String(envProjectFlag, "", "Name of the project (required unless you're in a workspace).")
	viper.BindPFlag(envProjectFlag, cmd.PersistentFlags().Lookup(envProjectFlag))

	cmd.AddCommand(BuildEnvAddCmd())
	cmd.AddCommand(BuildEnvListCmd())
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": "Develop 🔧",
	}
	return cmd
}

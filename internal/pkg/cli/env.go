// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/aws/amazon-ecs-cli-v2/cmd/archer/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
)

const (
	// EnvProjectFlag is the flag for providing the project name to the env commands.
	EnvProjectFlag = "project"
)

// BuildEnvCmd is the top level command for environments
func BuildEnvCmd() *cobra.Command {
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
			viper.SetDefault(EnvProjectFlag, summary.ProjectName)

		},
	}

	// The project flag is available to all subcommands through viper.GetString("project")
	cmd.PersistentFlags().String(EnvProjectFlag, "", "Name of the project (required unless you're in a workspace).")
	viper.BindPFlag(EnvProjectFlag, cmd.PersistentFlags().Lookup(EnvProjectFlag))

	cmd.AddCommand(BuildEnvAddCmd())
	cmd.AddCommand(BuildEnvListCmd())
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}
	return cmd
}

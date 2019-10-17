// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/aws/amazon-ecs-cli-v2/cmd/archer/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
)

// BuildEnvCmd is the top level command for environments
func BuildEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Environment commands",
		Long: `Command for working with environments.
An environment represents a deployment stage.`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			bindProjectName()
		},
	}
	// The flags bound by viper are available to all sub-commands through viper.GetString({flagName})
	cmd.PersistentFlags().StringP(projectFlag, projectFlagShort, "" /* default */, projectFlagDescription)
	viper.BindPFlag(projectFlag, cmd.PersistentFlags().Lookup(projectFlag))

	cmd.AddCommand(BuildEnvInitCmd())
	cmd.AddCommand(BuildEnvListCmd())
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}
	return cmd
}

// bindProjectName loads the project's name to viper.
// If there is an error, we swallow the error and leave the default value as empty string.
func bindProjectName() {
	name, err := loadProjectName()
	if err != nil {
		return
	}
	viper.SetDefault(projectFlag, name)
}

// loadProjectName retrieves the project's name from the workspace if it exists and returns it.
// If there is an error, it returns an empty string and the error.
func loadProjectName() (string, error) {
	// Load the workspace and set the project flag.
	ws, err := workspace.New()
	if err != nil {
		// If there's an error fetching the workspace, fall back to requiring
		// the project flag be set.
		return "", fmt.Errorf("fetching workspace: %w", err)
	}

	summary, err := ws.Summary()
	if err != nil {
		// If there's an error reading from the workspace, fall back to requiring
		// the project flag be set.
		return "", fmt.Errorf("reading from workspace: %w", err)
	}
	return summary.ProjectName, nil
}

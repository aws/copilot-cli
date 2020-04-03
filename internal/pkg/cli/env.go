// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/aws/amazon-ecs-cli-v2/cmd/ecs-preview/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
)

// BuildEnvCmd is the top level command for environments
func BuildEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Environment commands.",
		Long: `Command for working with environments.
An environment represents a deployment stage.`,
	}
	// The flags bound by viper are available to all sub-commands through viper.GetString({flagName})
	cmd.PersistentFlags().StringP(projectFlag, projectFlagShort, "" /* default */, projectFlagDescription)
	viper.BindPFlag(projectFlag, cmd.PersistentFlags().Lookup(projectFlag))

	cmd.AddCommand(BuildEnvInitCmd())
	cmd.AddCommand(BuildEnvListCmd())
	cmd.AddCommand(BuildEnvDeleteCmd())
	cmd.AddCommand(BuildEnvShowCmd())
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}
	return cmd
}

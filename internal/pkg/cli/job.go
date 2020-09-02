// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// BuildJobCmd is the top level command for jobs.
func BuildJobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "job",
		Short: `Commands for scheduled jobs.
Jobs are Amazon ECS tasks which run on a fixed schedule.`,
		Long: `Commands for scheduled jobs.
Jobs are Amazon ECS tasks which run on a fixed schedule.`,
	}
	// The flags bound by viper are available to all sub-commands through viper.GetString({flagName})
	cmd.PersistentFlags().StringP(appFlag, appFlagShort, "" /* default */, appFlagDescription)
	_ = viper.BindPFlag(appFlag, cmd.PersistentFlags().Lookup(appFlag)) // Ignore err because the flag name is not empty.

	cmd.AddCommand(BuildJobInitCmd())
	// cmd.AddCommand(BuildJobPackageCmd())
	// cmd.AddCommand(BuildJobDeployCmd())

	cmd.SetUsageTemplate(template.Usage)

	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}
	return cmd
}

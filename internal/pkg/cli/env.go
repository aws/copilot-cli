// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
)

// BuildEnvCmd is the top level command for environments.
func BuildEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "env",
		Short: `Commands for environments.
Environments are deployment stages shared between services.`,
		Long: `Commands for environments.
Environments are deployment stages shared between services.`,
	}
	// The flags bound by viper are available to all sub-commands through viper.GetString({flagName})
	cmd.PersistentFlags().StringP(appFlag, appFlagShort, "" /* default */, appFlagDescription)
	viper.BindPFlag(appFlag, cmd.PersistentFlags().Lookup(appFlag))

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

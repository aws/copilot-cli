// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/spf13/cobra"
)

// Deprecated. Clients should use "env deploy" instead after copilot v1.20.0.
// buildEnvUpgradeCmd builds the command to update environment(s) to the latest version of
// the environment template.
func buildEnvUpgradeCmd() *cobra.Command {
	var envName, appName string
	var all bool
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Deprecated. Upgrades the template of an environment to the latest version.",
		Long: `Deprecated. Use "copilot env deploy" instead.
This command is now a no op. Upgrades the template of an environment stack to the latest version.
`,
		Hidden: true,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			return nil
		}),
	}
	cmd.Flags().StringVarP(&envName, nameFlag, nameFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().BoolVar(&all, allFlag, false, upgradeAllEnvsDescription)
	return cmd
}

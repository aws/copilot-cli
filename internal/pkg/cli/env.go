// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/spf13/cobra"
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

	cmd.AddCommand(buildEnvInitCmd())
	cmd.AddCommand(buildEnvListCmd())
	cmd.AddCommand(buildEnvDeleteCmd())
	cmd.AddCommand(buildEnvShowCmd())
	cmd.AddCommand(buildEnvUpgradeCmd())
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}
	return cmd
}

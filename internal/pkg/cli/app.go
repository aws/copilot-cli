// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/spf13/cobra"

	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
)

// BuildAppCmd builds the top level app command and related subcommands.
func BuildAppCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "app",
		Short: `Commands for applications.
Applications are a collection of services and environments.`,
		Long: `Commands for applications.
Applications are a collection of services and environments.`,
	}

	cmd.AddCommand(buildAppInitCommand())
	cmd.AddCommand(buildAppListCommand())
	cmd.AddCommand(buildAppShowCmd())
	cmd.AddCommand(buildAppDeleteCommand())
	cmd.AddCommand(buildAppUpgradeCmd())

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}

	return cmd
}

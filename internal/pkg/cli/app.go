// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/spf13/cobra"

	"github.com/aws/amazon-ecs-cli-v2/cmd/copilot/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
)

// BuildAppCmd builds the top level app command and related subcommands.
func BuildAppCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Manage your applications.",
		Long: `Manage your applications.
Applications are a collection of services, and deployment environments.`,
	}

	cmd.AddCommand(BuildAppInitCommand())
	cmd.AddCommand(BuildAppListCommand())
	cmd.AddCommand(BuildAppShowCmd())
	cmd.AddCommand(BuildAppDeleteCommand())

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}

	return cmd
}

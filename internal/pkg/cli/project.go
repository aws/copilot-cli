// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/spf13/cobra"

	"github.com/aws/amazon-ecs-cli-v2/cmd/ecs-preview/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
)

// BuildProjCmd builds the top level project command and related subcommands.
func BuildProjCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Project commands.",
		Long: `Command for working with projects.
A Project represents all of your deployment environments.`,
	}

	cmd.AddCommand(BuildAppInitCommand())
	cmd.AddCommand(BuildProjectListCommand())
	cmd.AddCommand(BuildAppShowCmd())
	cmd.AddCommand(BuildAppDeleteCommand())

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}

	return cmd
}

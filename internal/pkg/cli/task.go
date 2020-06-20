// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/amazon-ecs-cli-v2/cmd/copilot/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

// BuildTaskCmd is the top level command for task.
func BuildTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "task",
		Short: `Commands for tasks.
Tasks are one-off Amazon ECS tasks.`,
		Long: `Commands for tasks
Tasks are one-off container images that run once in a given environment, then terminate.`,
	}

	cmd.AddCommand(BuildTaskRunCmd())

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Operational,
	}
	return cmd
}

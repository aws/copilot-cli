// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

// BuildTaskCmd is the top level command for task.
func BuildTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "task",
		Short: `Commands for tasks.
One-off Amazon ECS tasks that terminate once their work is done.`,
	}

	cmd.AddCommand(BuildTaskRunCmd())
	cmd.AddCommand(buildTaskExecCmd())
	cmd.AddCommand(BuildTaskDeleteCmd())

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}
	return cmd
}

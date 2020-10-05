// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/spf13/cobra"
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

	cmd.AddCommand(buildJobInitCmd())
	// cmd.AddCommand(BuildJobPackageCmd())
	cmd.AddCommand(buildJobDeployCmd())
	cmd.AddCommand(buildJobDeleteCmd())

	cmd.SetUsageTemplate(template.Usage)

	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}
	return cmd
}

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
		Short: `Commands for jobs.
Jobs are tasks that are triggered by events.`,
		Long: `Commands for jobs.
Jobs are tasks that are triggered by events.`,
	}

	cmd.AddCommand(buildJobInitCmd())
	cmd.AddCommand(buildJobListCmd())
	cmd.AddCommand(buildJobPackageCmd())
	cmd.AddCommand(buildJobOverrideCmd())
	cmd.AddCommand(buildJobDeployCmd())
	cmd.AddCommand(buildJobDeleteCmd())
	cmd.AddCommand(buildJobLogsCmd())
	cmd.AddCommand(buildJobRunCmd())

	cmd.SetUsageTemplate(template.Usage)

	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}
	return cmd
}

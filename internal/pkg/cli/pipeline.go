// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

// BuildPipelineCmd is the top level command for pipelines
func BuildPipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "pipeline",
		Short: `Commands for pipelines.
Continuous delivery pipelines to release services.`,
		Long: `Commands for pipelines.
Continuous delivery pipelines to release services.`,
	}

	cmd.AddCommand(BuildPipelineInitCmd())
	cmd.AddCommand(BuildPipelineUpdateCmd())
	cmd.AddCommand(BuildPipelineDeleteCmd())
	cmd.AddCommand(BuildPipelineShowCmd())
	cmd.AddCommand(BuildPipelineStatusCmd())

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Release,
	}

	return cmd
}

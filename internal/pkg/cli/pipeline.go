// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/amazon-ecs-cli-v2/cmd/ecs-preview/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

// BuildPipelineCmd is the top level command for pipelines
func BuildPipelineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Manage your pipelines.",
		Long: `Manage your pipelines.
Continuous delivery pipelines to release the services in your applications.`,
	}

	cmd.AddCommand(BuildPipelineInitCmd())
	cmd.AddCommand(BuildPipelineUpdateCmd())
	cmd.AddCommand(BuildPipelineDeleteCmd())
	cmd.AddCommand(BuildPipelineShowCmd())

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Release,
	}

	return cmd
}

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/amazon-ecs-cli-v2/cmd/ecs-preview/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

// BuildVariableCmd is the top level command for secrets.
func BuildVariableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "variable",
		Short: "Environment variable commands.",
		Long:  `Command for working with environment variables.`,
	}

	cmd.AddCommand(BuildVariableAddCmd())
	cmd.AddCommand(BuildVariableDeleteCmd())

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Develop,
	}

	return cmd
}

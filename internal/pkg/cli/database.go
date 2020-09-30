// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/amazon-ecs-cli-v2/cmd/ecs-preview/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

// BuildDatabaseCmd is the top level command for the database options.
func BuildDatabaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "database",
		Short: "Database commands.",
		Long:  `Command for working with RDS databases.`,
	}

	cmd.AddCommand(BuildDatabaseCreateCmd())
	cmd.AddCommand(BuildDatabaseDeleteCmd())

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Storage,
	}

	return cmd
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/amazon-ecs-cli-v2/cmd/copilot/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

// BuildStorageCmd is the top level command for storage
func BuildStorageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "storage",
		Short:  "Commands for working with storage and databases.",
		Long: `Commands for working with storage and databases.
Augment your services with S3 buckets, NoSQL and SQL databases.`,
	}

	cmd.AddCommand(BuildStorageInitCmd())

	cmd.SetUsageTemplate(template.Usage)

	cmd.Annotations = map[string]string{
		"group": group.Addons,
	}
	return cmd
}

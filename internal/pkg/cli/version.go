// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the archer subcommands.
package cli

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/cmd/ecs-preview/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/group"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/version"
	"github.com/spf13/cobra"
)

// BuildVersionCmd builds the command for displaying the version
func BuildVersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number.",
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			fmt.Printf("version: %s, built for %s\n", version.Version, version.Platform)
			return nil
		}),
		Annotations: map[string]string{
			"group": group.Settings,
		},
	}
	cmd.SetUsageTemplate(template.Usage)
	return cmd
}

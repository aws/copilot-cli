// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package init provides the init command.
package init

import (
	"github.com/aws/PRIVATE-amazon-ecs-archer/cmd/archer/template"
	"github.com/spf13/cobra"
)

// Build returns the init command.
func Build() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new ECS application ✨",
		Run: func(cmd *cobra.Command, args []string) {
			// TODO
		},
	}
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": "Getting Started",
	}
	return cmd
}

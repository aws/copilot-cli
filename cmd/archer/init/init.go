// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package init provides the init command.
package init

import (
	"github.com/aws/PRIVATE-amazon-ecs-archer/cmd/archer/template"
	archerApp "github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer/app"
	"github.com/spf13/cobra"
)

// Build returns the init command.
func Build() *cobra.Command {
	opts := archerApp.InitOpts{}
	app := archerApp.New()
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create a new ECS application ✨",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return app.Ask()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return app.Init(&opts)
		},
	}
	cmd.Flags().StringVarP(&app.Project, "project", "p", "", "Name of the project (required)")
	cmd.Flags().StringVarP(&app.Name, "app", "a", "", "Name of the application (required)")
	cmd.Flags().StringVarP(&opts.ManifestTemplate, "template", "t", "", "Template of the application to bootstrap the infrastructure")
	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": "Getting Started",
	}
	return cmd
}

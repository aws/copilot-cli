// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package main contains the root command.
package main

import (
	"os"

	initCmd "github.com/aws/PRIVATE-amazon-ecs-archer/cmd/archer/init"
	"github.com/aws/PRIVATE-amazon-ecs-archer/cmd/archer/template"
	"github.com/spf13/cobra"
)

func main() {
	cmd := buildRootCmd()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func buildRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archer",
		Short: "Launch and manage applications on Amazon ECS and AWS Fargate 🚀",
		Example: `
  Display the help menu for the init command
  $ archer init --help`,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// If we don't set a Run() function the help menu doesn't show up.
			// See https://github.com/spf13/cobra/issues/790
		},
		SilenceUsage: true,
	}

	cmd.AddCommand(initCmd.Build())
	cmd.SetUsageTemplate(template.RootUsage)
	return cmd
}

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the archer subcommands.
package cli

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/version"
	"github.com/spf13/cobra"
)


// BuildVersionCmd builds the command for displaying the version
func BuildVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Archer",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Archer version: %s (*%s)", version.Version, version.GitHash)
		},
	}
}

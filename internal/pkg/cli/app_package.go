// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os/exec"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/spf13/cobra"
)

// PackageAppOpts holds the configuration needed to transform an application's manifest to CloudFormation.
type PackageAppOpts struct {
	AppName   string
	EnvName   string
	Tag       string
	OutputDir string

	prompt prompter
}

// NewPackageAppOpts returns a new PackageAppOpts where the image tag is set to "manual-{short git sha}".
// If an error occurred while running git, we set the image name to "latest" instead.
func NewPackageAppOpts() *PackageAppOpts {
	commitID, err := exec.Command("git", "rev-parse", "--short", "HEAD").CombinedOutput()
	if err != nil {
		// If we can't retrieve a commit ID we default the image tag to "latest".
		return &PackageAppOpts{
			Tag:    "latest",
			prompt: prompt.New(),
		}
	}
	return &PackageAppOpts{
		Tag:    fmt.Sprintf("manual-%s", commitID),
		prompt: prompt.New(),
	}
}

// Ask prompts the user for any missing required fields.
func (opts *PackageAppOpts) Ask() error {
	return nil
}

// Validate returns an error if the values provided by the user are invalid.
func (opts *PackageAppOpts) Validate() error {
	return nil
}

// Execute prints the CloudFormation template of the application for the environment.
func (opts *PackageAppOpts) Execute() error {
	return nil
}

// BuildAppPackageCmd builds the command for printing an application's CloudFormation template.
func BuildAppPackageCmd() *cobra.Command {
	opts := NewPackageAppOpts()
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Prints the AWS CloudFormation template of an application.",
		Long:  `Prints the CloudFormation template used to deploy an application to an environment.`,
		Example: `
  Print the CloudFormation template for the "frontend" application parametrized for the "test" environment.
  /code $ archer app package -n frontend -e test

  Write the CloudFormation stack and configuration to a "infrastructure/" sub-directory instead of printing.
  /code $ archer app package -n frontend -e test --output-dir ./infrastructure
  /code $ ls ./infrastructure
  /code frontend.stack.yml      frontend-test.config.yml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			return opts.Execute()
		},
	}
	cmd.Flags().StringVarP(&opts.AppName, "name", "n", "", "Name of the application.")
	cmd.Flags().StringVarP(&opts.EnvName, "env", "e", "", "Name of the environment.")
	cmd.Flags().StringVar(&opts.Tag, "tag", "", `Optional. The application's image tag. Defaults to your latest git commit's hash.`)
	cmd.Flags().StringVar(&opts.OutputDir, "output-dir", "", "Optional. Writes the stack template and template configuration to a directory.")
	return cmd
}

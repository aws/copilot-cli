// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os/exec"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store/ssm"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// PackageAppOpts holds the configuration needed to transform an application's manifest to CloudFormation.
type PackageAppOpts struct {
	// Fields with matching flags.
	AppName   string
	EnvName   string
	Tag       string
	OutputDir string

	// Interfaces to interact with dependencies.
	appStore archer.ApplicationStore
	envStore archer.EnvironmentStore
	prompt   prompter
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
	if opts.AppName == "" {
		names, err := opts.listAppNames()
		if err != nil {
			return err
		}
		app, err := opts.prompt.SelectOne("Which application's CloudFormation template would you like to generate?", "", names)
		if err != nil {
			return fmt.Errorf("prompt application name: %w", err)
		}
		opts.AppName = app
	}
	if opts.EnvName == "" {
		names, err := opts.listEnvNames()
		if err != nil {
			return err
		}
		env, err := opts.prompt.SelectOne("Which environment's configuration would you like to use for your stack's parameters?", "", names)
		if err != nil {
			return fmt.Errorf("prompt environment name: %w", err)
		}
		opts.EnvName = env
	}
	return nil
}

// Validate returns an error if the values provided by the user are invalid.
func (opts *PackageAppOpts) Validate() error {
	project := viper.GetString(projectFlag)
	if project == "" {
		return errNoWorkspace
	}
	if opts.AppName != "" {
		if _, err := opts.appStore.GetApplication(project, opts.AppName); err != nil {
			return err
		}
	}
	if opts.EnvName != "" {
		if _, err := opts.envStore.GetEnvironment(project, opts.EnvName); err != nil {
			return err
		}
	}
	return nil
}

// Execute prints the CloudFormation template of the application for the environment.
func (opts *PackageAppOpts) Execute() error {
	return nil
}

func (opts *PackageAppOpts) listAppNames() ([]string, error) {
	project := viper.GetString(projectFlag)
	apps, err := opts.appStore.ListApplications(project)
	if err != nil {
		return nil, fmt.Errorf("list applications for project %s: %w", project, err)
	}
	var names []string
	for _, app := range apps {
		names = append(names, app.Name)
	}
	return names, nil
}

func (opts *PackageAppOpts) listEnvNames() ([]string, error) {
	project := viper.GetString(projectFlag)
	envs, err := opts.envStore.ListEnvironments(project)
	if err != nil {
		return nil, fmt.Errorf("list environments for project %s: %w", project, err)
	}
	var names []string
	for _, env := range envs {
		names = append(names, env.Name)
	}
	return names, nil
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
		PreRunE: func(cmd *cobra.Command, args []string) error {
			store, err := ssm.NewStore()
			if err != nil {
				return fmt.Errorf("couldn't connect to application datastore: %w", err)
			}
			opts.appStore = store
			opts.envStore = store
			return opts.Validate()
		},
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

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/command"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
)

const (
	jobPackageJobNamePrompt = "Which job would you like to generate a CloudFormation template for?"
	jobPackageEnvNamePrompt = "Which environment would you like to package this stack for?"
)

type packageJobVars struct {
	name      string
	envName   string
	appName   string
	tag       string
	outputDir string
}

type packageJobOpts struct {
	packageJobVars

	// Interfaces to interact with dependencies.
	ws     wsJobDirReader
	store  store
	runner runner
	sel    wsSelector
	prompt prompter
}

func newPackageJobOpts(vars packageJobVars) (*packageJobOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to config store: %w", err)
	}
	prompter := prompt.New()
	sel, err := selector.NewWorkspaceSelect(prompter, store, ws)
	if err != nil {
		return nil, err
	}
	opts := &packageJobOpts{
		packageJobVars: vars,
		ws:             ws,
		store:          store,
		runner:         command.New(),
		sel:            sel,
		prompt:         prompter,
	}
	return opts, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *packageJobOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	if o.name != "" {
		names, err := o.ws.JobNames()
		if err != nil {
			return fmt.Errorf("list jobs in the workspace: %w", err)
		}
		if !contains(o.name, names) {
			return fmt.Errorf("job '%s' does not exist in the workspace", o.name)
		}
	}
	if o.envName != "" {
		if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts the user for any missing required fields.
func (o *packageJobOpts) Ask() error {
	if err := o.askJobName(); err != nil {
		return err
	}
	if err := o.askEnvName(); err != nil {
		return err
	}
	tag, err := askImageTag(o.tag, o.prompt, o.runner)
	if err != nil {
		return err
	}
	o.tag = tag
	return nil
}

// Execute prints the CloudFormation template of the application for the environment.
func (o *packageJobOpts) Execute() error {
	return nil
}

func (o *packageJobOpts) askJobName() error {
	if o.name != "" {
		return nil
	}

	name, err := o.sel.Job(jobPackageJobNamePrompt, "")
	if err != nil {
		return fmt.Errorf("select job: %w", err)
	}
	o.name = name
	return nil
}

func (o *packageJobOpts) askEnvName() error {
	if o.envName != "" {
		return nil
	}

	name, err := o.sel.Environment(jobPackageEnvNamePrompt, "", o.appName)
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.envName = name
	return nil
}

// buildJobPackageCmd builds the command for printing a job's CloudFormation template.
func buildJobPackageCmd() *cobra.Command {
	vars := packageJobVars{}
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Prints the AWS CloudFormation template of a job.",
		Long:  `Prints the CloudFormation template used to deploy a job to an environment.`,
		Example: `
  Print the CloudFormation template for the "report-generator" job parametrized for the "test" environment.
  /code $ copilot job package -n report-generator -e test

  Write the CloudFormation stack and configuration to a "infrastructure/" sub-directory instead of printing.
  /code $ copilot job package -n report-generator -e test --output-dir ./infrastructure
  /code $ ls ./infrastructure
  /code report-generator-test.stack.yml      report-generator-test.params.yml`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newPackageJobOpts(vars)
			if err != nil {
				return err
			}

			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVar(&vars.tag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringVar(&vars.outputDir, stackOutputDirFlag, "", stackOutputDirFlagDescription)
	return cmd
}

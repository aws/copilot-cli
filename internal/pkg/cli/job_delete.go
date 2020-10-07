// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/aws/copilot-cli/internal/pkg/term/selector"

	"github.com/spf13/cobra"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
)

const (
	fmtJobDeleteConfirmPrompt        = "Are you sure you want to delete %s from application %s?"
	fmtJobDeleteFromEnvConfirmPrompt = "Are you sure you want to delete %s from environment %s?"
	jobDeleteAppNamePrompt           = "Which application's job would you like to delete?"
	jobDeleteJobNamePrompt           = "Which job would you like to delete?"
	jobDeleteConfirmHelp             = "This will remove the job from all environments and delete it from your app."
	jobDeleteFromEnvConfirmHelp      = "This will remove the job from just the %s environment."
)

var (
	errJobDeleteCancelled = errors.New("job delete cancelled - no changes made")
)

type deleteJobVars struct {
	appName          string
	skipConfirmation bool
	name             string
	envName          string
}

type deleteJobOpts struct {
	deleteJobVars

	// Interfaces to dependencies.
	store  store
	prompt prompter
	sel    wsSelector
}

func newDeleteJobOpts(vars deleteJobVars) (*deleteJobOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}

	prompter := prompt.New()
	return &deleteJobOpts{
		deleteJobVars: vars,

		store:  store,
		prompt: prompter,
		sel:    selector.NewWorkspaceSelect(prompter, store, ws),
	}, nil
}

// Validate returns an error if the user inputs are invalid.
func (o *deleteJobOpts) Validate() error {
	if o.name != "" {
		if _, err := o.store.GetJob(o.appName, o.name); err != nil {
			return err
		}
	}
	if o.envName != "" {
		return o.validateEnvName()
	}
	return nil
}

// Ask prompts the user for any required flags.
func (o *deleteJobOpts) Ask() error {
	if err := o.askAppName(); err != nil {
		return err
	}
	if err := o.askJobName(); err != nil {
		return err
	}

	if o.skipConfirmation {
		return nil
	}

	// When there's no env name passed in, we'll completely
	// remove the job from the application.
	deletePrompt := fmt.Sprintf(fmtJobDeleteConfirmPrompt, o.name, o.appName)
	deleteConfirmHelp := jobDeleteConfirmHelp
	if o.envName != "" {
		// When a customer provides a particular environment,
		// we'll just delete the job from that environment -
		// but keep it in the app.
		deletePrompt = fmt.Sprintf(fmtJobDeleteFromEnvConfirmPrompt, o.name, o.envName)
		deleteConfirmHelp = fmt.Sprintf(jobDeleteFromEnvConfirmHelp, o.envName)
	}

	deleteConfirmed, err := o.prompt.Confirm(
		deletePrompt,
		deleteConfirmHelp)

	if err != nil {
		return fmt.Errorf("job delete confirmation prompt: %w", err)
	}
	if !deleteConfirmed {
		return errJobDeleteCancelled
	}
	return nil
}

func (o *deleteJobOpts) validateEnvName() error {
	if _, err := o.targetEnv(); err != nil {
		return err
	}
	return nil
}

func (o *deleteJobOpts) targetEnv() (*config.Environment, error) {
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return nil, fmt.Errorf("get environment %s from config store: %w", o.envName, err)
	}
	return env, nil
}

func (o *deleteJobOpts) askAppName() error {
	if o.appName != "" {
		return nil
	}

	name, err := o.sel.Application(jobDeleteAppNamePrompt, "")
	if err != nil {
		return fmt.Errorf("select application name: %w", err)
	}
	o.appName = name
	return nil
}

func (o *deleteJobOpts) askJobName() error {
	if o.name != "" {
		return nil
	}

	name, err := o.sel.Job(jobDeleteJobNamePrompt, "", o.appName)
	if err != nil {
		return fmt.Errorf("select job: %w", err)
	}
	o.name = name
	return nil
}

// buildJobDeleteCmd builds the command to delete job(s).
func buildJobDeleteCmd() *cobra.Command {
	vars := deleteJobVars{}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes a job from an application.",
		Example: `
  Delete the "report-generator" job from the my-app application.
  /code $ copilot job delete --name report-generator --app my-app

  Delete the "report-generator" job from just the prod environment.
  /code $ copilot job delete --name report-generator --env prod

  Delete the "report-generator" job from the my-app application from outside of the workspace.
  /code $ copilot job delete --name report-generator --app my-app

  Delete the "report-generator" job without confirmation prompt.
  /code $ copilot job delete --name report-generator --yes`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeleteJobOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			return opts.Ask()
		}),
	}

	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().BoolVar(&vars.skipConfirmation, yesFlag, false, yesFlagDescription)
	return cmd
}

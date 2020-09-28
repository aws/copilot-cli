// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
)

const (
	jobDeleteNamePrompt              = "Which job would you like to delete?"
	fmtJobDeleteConfirmPrompt        = "Are you sure you want to delete %s from application %s?"
	fmtJobDeleteFromEnvConfirmPrompt = "Are you sure you want to delete %s from environment %s?"
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
	store   store
	sess    sessionProvider
	spinner progress
	prompt  prompter
	appCFN  jobRemoverFromApp
	getECR  func(session *awssession.Session) imageRemover

	// Internal state.
	environments []*config.Environment
}

func newDeleteJobOpts(vars deleteJobVars) (*deleteJobOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	provider := sessions.NewProvider()
	defaultSession, err := provider.Default()
	if err != nil {
		return nil, err
	}

	return &deleteJobOpts{
		deleteJobVars: vars,

		store:   store,
		spinner: termprogress.NewSpinner(),
		prompt:  prompt.New(),
		sess:    provider,
		appCFN:  cloudformation.New(defaultSession),
		getECR: func(session *awssession.Session) imageRemover {
			return ecr.New(session)
		},
	}, nil
}

// Validate returns an error if the user inputs are invalid.
func (o *deleteJobOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	if o.name != "" {
		if _, err := o.store.GetJob(o.appName, o.name); err != nil {
			return err
		}
	}
	if o.envName != "" {
		if err := o.validateEnvName(); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts the user for any required flags.
func (o *deleteJobOpts) Ask() error {
	if err := o.askJobName(); err != nil {
		return err
	}

	if o.skipConfirmation {
		return nil
	}

	// When there's no env name passed in, we'll completely
	// remove the service from the application.
	deletePrompt := fmt.Sprintf(fmtJobDeleteConfirmPrompt, o.name, o.appName)
	deleteConfirmHelp := jobDeleteConfirmHelp
	if o.envName != "" {
		// When a customer provides a particular environment,
		// we'll just delete the service from that environment -
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

func (o *deleteJobOpts) askJobName() error {
	if o.name != "" {
		return nil
	}

	names, err := o.jobNames()
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("couldn't find any jobs in the application %s", o.appName)
	}
	if len(names) == 1 {
		o.name = names[0]
		log.Infof("Only found one job, defaulting to: %s\n", color.HighlightUserInput(o.name))
		return nil
	}
	name, err := o.prompt.SelectOne(jobDeleteNamePrompt, "", names)
	if err != nil {
		return fmt.Errorf("select job to delete: %w", err)
	}
	o.name = name
	return nil
}

func (o *deleteJobOpts) jobNames() ([]string, error) {
	jobs, err := o.store.ListJobs(o.appName)
	if err != nil {
		return nil, fmt.Errorf("list jobs for application %s: %w", o.appName, err)
	}
	var names []string
	for _, job := range jobs {
		names = append(names, job.Name)
	}
	return names, nil
}

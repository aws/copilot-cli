// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/spf13/cobra"
)

const (
	taskDeleteNamePrompt              = "Which task would you like to delete?"
	taskDeleteAppPrompt               = "Which application would you like to delete a task from?"
	taskDeleteEnvPrompt               = "Which environment would you like to delete a task from?"
	fmtTaskDeleteDefaultConfirmPrompt = "Are you sure you want to delete %s from the default cluster?"
	fmtTaskDeleteFromEnvConfirmPrompt = "Are you sure you want to delete %s from application %s and environment %s?"
	taskDeleteConfirmHelp             = "This will delete the task's stack and stop all current executions."
)

var errTaskDeleteCancelled = errors.New("task delete cancelled - no changes made")

type deleteTaskVars struct {
	name             string
	app              string
	env              string
	skipConfirmation bool
	defaultCluster   bool
}

type deleteTaskOpts struct {
	deleteTaskVars

	// Dependencies to interact with other modules
	store  store
	prompt prompter
	sess   sessionProvider
	sel    wsSelector

	// Generators for env-specific clients
	newTaskSel func(session *awssession.Session) cfTaskSelector
}

func newDeleteTaskOpts(vars deleteTaskVars) (*deleteTaskOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	provider := sessions.NewProvider()

	prompter := prompt.New()

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}

	return &deleteTaskOpts{
		deleteTaskVars: vars,

		store:  store,
		prompt: prompter,
		sess:   provider,
		sel:    selector.NewWorkspaceSelect(prompter, store, ws),
		newTaskSel: func(session *awssession.Session) cfTaskSelector {
			cfn := cloudformation.New(session)
			return selector.NewCFTaskSelect(prompter, store, cfn)
		},
	}, nil
}

// Validate checks that flag inputs are valid.
func (o *deleteTaskOpts) Validate() error {

	if o.name != "" {
		if err := basicNameValidation(o.name); err != nil {
			return err
		}
	}

	// If default flag specified,
	if err := o.validateFlagsWithDefaultCluster(); err != nil {
		return err
	}

	if err := o.validateFlagsWithEnv(); err != nil {
		return err
	}

	return nil
}

func (o *deleteTaskOpts) validateFlagsWithEnv() error {
	if o.app != "" {
		if _, err := o.store.GetApplication(o.app); err != nil {
			return fmt.Errorf("get application: %w", err)
		}
	}

	if o.env != "" && o.app != "" {
		if _, err := o.store.GetEnvironment(o.app, o.env); err != nil {
			return err
		}
	}

	return nil
}

func (o *deleteTaskOpts) validateFlagsWithDefaultCluster() error {
	if !o.defaultCluster {
		return nil
	}

	// If app is specified and the same as the workspace app, don't throw an error.
	// If app is not the same as the workspace app, either
	//   a) there is no WS app and it's specified erroneously, in which case we should error
	//   b) there is a WS app and the flag has been set to a different app, in which case we should error.
	// The app flag defaults to the WS app so there's an edge case where it's possible to specify
	// `copilot task delete --app ws-app --default` and not error out, but this should be taken as
	// specifying "default".
	if o.app != tryReadingAppName() {
		return fmt.Errorf("cannot specify both `--app` and `--default`")
	}

	if o.env != "" {
		return fmt.Errorf("cannot specify both `--env` and `--default`")
	}

	return nil
}

func (o *deleteTaskOpts) askAppName() error {
	if o.defaultCluster {
		return nil
	}

	if o.app != "" {
		return nil
	}

	app, err := o.sel.Application(taskDeleteAppPrompt, "", appEnvOptionNone)
	if err != nil {
		return fmt.Errorf("select application name: %w", err)
	}
	if app == appEnvOptionNone {
		o.env = ""
		o.defaultCluster = true
		return nil
	}
	o.app = app
	return nil
}

func (o *deleteTaskOpts) askEnvName() error {
	if o.defaultCluster {
		return nil
	}

	if o.env != "" {
		return nil
	}
	env, err := o.sel.Environment(taskDeleteEnvPrompt, "", o.app, appEnvOptionNone)
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	if env == appEnvOptionNone {
		o.env = ""
		o.app = ""
		o.defaultCluster = true
		return nil
	}
	o.env = env
	return nil
}

// Ask prompts for missing information and fills in gaps.
func (o *deleteTaskOpts) Ask() error {
	if err := o.askAppName(); err != nil {
		return err
	}

	if err := o.askEnvName(); err != nil {
		return err
	}

	if err := o.askTaskName(); err != nil {
		return err
	}

	if o.skipConfirmation {
		return nil
	}

	// Confirm deletion
	deletePrompt := fmt.Sprintf(fmtTaskDeleteDefaultConfirmPrompt, color.HighlightUserInput(o.name))
	if o.env != "" && o.app != "" {
		deletePrompt = fmt.Sprintf(
			fmtTaskDeleteFromEnvConfirmPrompt,
			color.HighlightUserInput(o.name),
			color.HighlightUserInput(o.app),
			color.HighlightUserInput(o.env),
		)
	}

	deleteConfirmed, err := o.prompt.Confirm(
		deletePrompt,
		taskDeleteConfirmHelp)

	if err != nil {
		return fmt.Errorf("task delete confirmation prompt: %w", err)
	}
	if !deleteConfirmed {
		return errTaskDeleteCancelled
	}
	return nil
}

func (o *deleteTaskOpts) getSession() (*awssession.Session, error) {
	if o.defaultCluster {
		sess, err := o.sess.Default()
		if err != nil {
			return nil, err
		}
		return sess, nil
	}
	// Get environment manager role for deleting stack.
	env, err := o.store.GetEnvironment(o.app, o.env)
	if err != nil {
		return nil, err
	}
	sess, err := o.sess.FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return nil, err
	}
	return sess, nil
}

func (o *deleteTaskOpts) askTaskName() error {
	if o.name != "" {
		return nil
	}

	sess, err := o.getSession()
	if err != nil {
		return fmt.Errorf("get task select session: %w", err)
	}
	sel := o.newTaskSel(sess)
	if o.defaultCluster {
		task, err := sel.Task(taskDeleteNamePrompt, "", selector.TaskWithDefaultCluster())
		if err != nil {
			return fmt.Errorf("select task from default cluster: %w", err)
		}
		o.name = task.Name
		return nil
	}
	task, err := sel.Task(taskDeleteNamePrompt, "", selector.TaskWithAppEnv(o.app, o.env))
	if err != nil {
		return fmt.Errorf("select task from environment: %w", err)
	}
	o.name = task.Name
	return nil
}

func (o *deleteTaskOpts) Execute() error {
	// Get clients.

	// Stop tasks.

	// Clear repository.

	// Delete stack.

	return nil
}

func (o *deleteTaskOpts) RecommendedActions() []string {
	return nil
}

// BuildTaskDeleteCmd builds the command to delete application(s).
func BuildTaskDeleteCmd() *cobra.Command {
	vars := deleteTaskVars{}
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "delete",
		Short:  "Deletes a one-off task from an application or default cluster.",
		Example: `
  Delete the "test" task from the default cluster.
  /code $ copilot task delete --name test --default

  Delete the "test" task from the prod environment.
  /code $ copilot task delete --name test --env prod

  Delete the "test" task without confirmation prompt.
  /code $ copilot task delete --name test --yes`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeleteTaskOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Execute(); err != nil {
				return err
			}

			log.Infoln("Recommended follow-up actions:")
			for _, followup := range opts.RecommendedActions() {
				log.Infof("- %s\n", followup)
			}
			return nil
		}),
	}

	cmd.Flags().StringVarP(&vars.app, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.env, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().BoolVar(&vars.skipConfirmation, yesFlag, false, yesFlagDescription)
	cmd.Flags().BoolVar(&vars.defaultCluster, taskDefaultFlag, false, taskDeleteDefaultFlagDescription)
	return cmd
}

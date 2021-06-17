// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/cobra"
	"strconv"
)

const (
	svcStopTaskNamePrompt        = "Which service's tasks would you like to stop?"
	fmtSvcStopTasksConfirmPrompt = "Are you sure you want to stop all tasks inside %s from application %s?"
	svcStopTasksConfirmHelp      = "This will stop all the tasks running under this service."
	taskStopUserInitiated        = "Task stopped as user initiated stop action"
	stopTaskCancelled            = "svc stop-task cancelled - no changes made"
)

var (
	errStopTasksCancelled = errors.New(stopTaskCancelled)
)

type wkldStopTaskVars struct {
	all     bool
	name    string
	envName string
	appName string
	taskIDs []string
}

type svcStopTaskOpts struct {
	wkldStopTaskVars

	store   store
	spinner progress
	sess    sessionProvider
	prompt  prompter
	sel     deploySelector

	// Generators for env-specific clients
	newTaskStopper func(session *session.Session) taskStopper

	// Cached variables
	session *session.Session
}

func newStopTaskOpts(vars wkldStopTaskVars) (*svcStopTaskOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}
	deployStore, err := deploy.NewStore(store)
	if err != nil {
		return nil, fmt.Errorf("connect to deploy store: %w", err)
	}

	provider := sessions.NewProvider()
	svcPrompt := prompt.New()

	return &svcStopTaskOpts{
		wkldStopTaskVars: vars,

		store:   store,
		spinner: termprogress.NewSpinner(log.DiagnosticWriter),
		sess:    provider,
		prompt:  svcPrompt,
		sel:     selector.NewDeploySelect(svcPrompt, store, deployStore),
		newTaskStopper: func(session *session.Session) taskStopper {
			return ecs.New(session)
		},
	}, nil
}

// Validate returns an error if the user inputs are invalid.
func (o *svcStopTaskOpts) Validate() error {
	if o.name != "" {
		if _, err := o.store.GetService(o.appName, o.name); err != nil {
			return err
		}
	}
	if o.envName != "" {
		_, err := o.store.GetEnvironment(o.appName, o.envName)
		if err != nil {
			return fmt.Errorf("get environment %s from config store: %w", o.envName, err)
		}
	}
	if !o.all && o.taskIDs == nil && len(o.taskIDs) == 0 {
		return fmt.Errorf(`any one of the following arguments are required "--all" or  "--tasks"`)
	}
	if o.all && o.taskIDs != nil && len(o.taskIDs) > 0 {
		return fmt.Errorf(`only one of "-all" or "--tasks" may be used`)
	}
	return nil
}

// AskInput prompts the user for any required flags.
func (o *svcStopTaskOpts) AskInput() error {
	if err := o.askAppName(); err != nil {
		return err
	}
	if err := o.askSvcEnvName(); err != nil {
		return err
	}

	// When "--all" is set all the tasks inside the service, will be stopped
	if o.all {
		stopPrompt := fmt.Sprintf(fmtSvcStopTasksConfirmPrompt, o.name, o.appName)
		stopConfirmed, err := o.prompt.Confirm(stopPrompt, svcStopTasksConfirmHelp)

		if err != nil {
			return fmt.Errorf("svc stop task confirmation prompt: %w", err)
		}
		if !stopConfirmed {
			return errStopTasksCancelled
		}
	}
	return nil
}

// Execute stop tasks of the service.
func (o *svcStopTaskOpts) Execute() error {
	sess, err := o.getSession()
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	if o.all {
		return o.stopAllTasks(sess)
	} else {
		return o.stopTaskIds(sess)
	}
}

// askAppName ask for application name
func (o *svcStopTaskOpts) askAppName() error {
	if o.appName != "" {
		return nil
	}

	name, err := o.sel.Application(svcAppNamePrompt, svcAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application name: %w", err)
	}
	o.appName = name
	return nil
}

// askSvcEnvName ask for environment name
func (o *svcStopTaskOpts) askSvcEnvName() error {
	if o.name != "" {
		return nil
	}

	deployedService, err := o.sel.DeployedService(svcStopTaskNamePrompt, "", o.appName, selector.WithEnv(o.envName), selector.WithSvc(o.name))
	if err != nil {
		return fmt.Errorf("select service: %w", err)
	}
	o.name = deployedService.Svc
	o.envName = deployedService.Env
	return nil
}

// getSession get AWS session
func (o *svcStopTaskOpts) getSession() (*session.Session, error) {
	if o.session != nil {
		return o.session, nil
	}
	// Get environment manager role for stopping tasks.
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return nil, err
	}
	sess, err := o.sess.FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return nil, err
	}
	o.session = sess
	return sess, nil
}

// stopTaskIds will stop all tasks with the provided taskID
func (o *svcStopTaskOpts) stopTaskIds(sess *session.Session) error {
	tasksLen := strconv.Itoa(len(o.taskIDs))
	o.spinner.Start(fmt.Sprintf("Stopping %s task(s)", color.HighlightUserInput(tasksLen)))

	// Stop task based on taskArn
	if err := o.newTaskStopper(sess).StopTasksWithTaskIds(o.appName, o.envName, o.taskIDs, taskStopUserInitiated); err != nil {
		o.spinner.Stop(log.Serrorf("Error stopping running tasks in %s.\n", o.name))
		return fmt.Errorf("stop running tasks by ids %w", err)
	}

	o.spinner.Stop(log.Ssuccessln("Task(s) are stopped successfully"))

	return nil
}

// stopAllTasks will stop all the tasks running in the service
func (o *svcStopTaskOpts) stopAllTasks(sess *session.Session) error {
	o.spinner.Start(fmt.Sprintf("Stopping all running tasks in %s.", color.HighlightUserInput(o.name)))

	// Stop all tasks.
	if err := o.newTaskStopper(sess).StopWorkloadTasks(o.appName, o.envName, o.name, taskStopUserInitiated); err != nil {
		o.spinner.Stop(log.Serrorf("Error stopping running tasks in %s.\n", o.name))
		return fmt.Errorf("stop running tasks in family %s: %w", o.name, err)
	}

	o.spinner.Stop(log.Ssuccessf("Stopped all running tasks in %s\n", color.HighlightUserInput(o.name)))

	return nil
}

// buildSvcStopTaskCmd builds the command for displaying service logs in an application.
func buildSvcStopTaskCmd() *cobra.Command {
	vars := wkldStopTaskVars{}
	cmd := &cobra.Command{
		Use:   "stop-task",
		Short: "Stops tasks inside a service.",

		Example: `
  Stop task id "1234"' associated with "my-svc" service in "test" environment
  /code $ copilot svc stop-task --tasks <<taskArn>> -n my-svc -e test
  Stop all tasks associated with "my-svc" service
  /code $ copilot svc stop-task -n my-svc -e test --all`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newStopTaskOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.AskInput(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().BoolVar(&vars.all, allFlag, false, allFlagDescription)
	cmd.Flags().StringSliceVar(&vars.taskIDs, tasksFlag, nil, tasksLogsFlagDescription)
	return cmd
}

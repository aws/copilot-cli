// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecr"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
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
	store   store
	prompt  prompter
	spinner progress
	sess    sessionProvider
	sel     wsSelector

	// Generators for env-specific clients
	newTaskSel           func(session *awssession.Session) cfTaskSelector
	newTaskListerStopper func(session *awssession.Session) tasksListerStopper
	newImageRemover      func(session *awssession.Session) imageRemover
	newTaskDeleter       func(session *awssession.Session) taskDeployer
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

		store:   store,
		spinner: termprogress.NewSpinner(log.DiagnosticWriter),
		prompt:  prompter,
		sess:    provider,
		sel:     selector.NewWorkspaceSelect(prompter, store, ws),
		newTaskSel: func(session *awssession.Session) cfTaskSelector {
			cfn := cloudformation.New(session)
			return selector.NewCFTaskSelect(prompter, store, cfn)
		},
		newTaskListerStopper: func(session *awssession.Session) tasksListerStopper {
			return ecs.New(session)
		},
		newTaskDeleter: func(session *awssession.Session) taskDeployer {
			return cloudformation.New(session)
		},
		newImageRemover: func(session *awssession.Session) imageRemover {
			return ecr.New(session)
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

	if o.app != "" && o.env != "" {
		if _, err := o.store.GetEnvironment(o.app, o.env); err != nil {
			return fmt.Errorf("get environment: %w", err)
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
		o.name = task
		return nil
	}
	task, err := sel.Task(taskDeleteNamePrompt, "", selector.TaskWithAppEnv(o.app, o.env))
	if err != nil {
		return fmt.Errorf("select task from environment: %w", err)
	}
	o.name = task
	return nil
}

func (o *deleteTaskOpts) Execute() error {
	// Get clients.
	sess, err := o.getSession()
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	ecsClient := o.newTaskListerStopper(sess)
	taskDeleter := o.newTaskDeleter(sess)

	// ECR Deletion happens from the default profile in app delete. We can do it here too.
	defaultSess, err := o.sess.DefaultWithRegion(aws.StringValue(sess.Config.Region))
	if err != nil {
		return fmt.Errorf("get default session for ECR deletion: %s", err)
	}
	ecrDeleter := o.newImageRemover(defaultSess)

	// Get information about the task stack. This struct will be used to get the names of the ECR
	// repo and task stack.
	taskInfo, err := taskDeleter.GetTaskStack(o.name)
	if err != nil {
		return fmt.Errorf("retrieve stack information for task %s: %w", o.name, err)
	}

	o.spinner.Start(fmt.Sprintf("Cleaning up resources for task %s.", color.HighlightUserInput(o.name)))
	// Get running tasks in family.
	var tasks []*awsecs.Task
	if o.defaultCluster {
		tasks, err = ecsClient.ListActiveDefaultClusterTasks(ecs.ListTasksFilter{
			TaskGroup: o.name,
		})
		if err != nil {
			o.spinner.Stop(log.Serrorln("Error listing running tasks."))
			return fmt.Errorf("list running tasks in default cluster: %w", err)
		}
	} else {
		tasks, err = ecsClient.ListActiveAppEnvTasks(ecs.ListActiveAppEnvTasksOpts{
			App: o.app,
			Env: o.env,
			ListTasksFilter: ecs.ListTasksFilter{
				TaskGroup: o.name,
			},
		})
		if err != nil {
			o.spinner.Stop(log.Serrorln("Error listing running tasks."))
			return fmt.Errorf("list running tasks in environment %s: %w", o.env, err)
		}
	}

	// Stop tasks.
	taskARNs := make([]string, len(tasks))
	for n, t := range tasks {
		taskARNs[n] = aws.StringValue(t.TaskArn)
	}

	if o.defaultCluster {
		if err = ecsClient.StopDefaultClusterTasks(taskARNs); err != nil {
			o.spinner.Stop(log.Serrorln("Error stopping running tasks in default cluster."))
			return fmt.Errorf("stop running tasks in family %s: %w", o.name, err)
		}
	} else {
		if err = ecsClient.StopAppEnvTasks(o.app, o.env, taskARNs); err != nil {
			o.spinner.Stop(log.Serrorln("Error stopping running tasks in environment."))
			return fmt.Errorf("stop running tasks in family %s: %w", o.name, err)
		}
	}

	// Clear repository.
	err = ecrDeleter.ClearRepository(taskInfo.ECRRepoName())
	if err != nil {
		o.spinner.Stop(log.Serrorln("Error emptying ECR repository."))
		return fmt.Errorf("clear ECR repository for task %s: %w", o.name, err)
	}
	// Delete stack.
	err = taskDeleter.DeleteTask(*taskInfo)
	if err != nil {
		o.spinner.Stop(log.Serrorln("Error deleting CloudFormation stack."))
		return fmt.Errorf("delete stack for task %s: %w", o.name, err)
	}

	o.spinner.Stop(log.Ssuccessf("Deleted resources of task %s.\n", color.HighlightUserInput(o.name)))
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

  Delete the "db-migrate" task from the prod environment.
  /code $ copilot task delete --name db-migrate --env prod

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

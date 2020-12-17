// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"


	"github.com/spf13/cobra"
)

const (
	taskDeleteNamePrompt              = "Which task would you like to delete?"
	fmtTaskDeleteDefaultConfirmPrompt = "Are you sure you want to delete %s from the default cluster?"
	fmtTaskDeleteFromEnvConfirmPrompt = "Are you sure you want to delete %s from environment %s?"
	taskDeleteConfirmHelp             = "This will delete the task's stack and stop all current executions."
)

var (
	errDefaultClusterWithApp = errors.New("")
	errTaskDeleteCancelled   = errors.New("task delete cancelled - no changes made")
)

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
	spinner progress
	sess sessionProvider
	sel taskSelector
	deleter
}

func newDeleteTaskOpts(vars deleteTaskVars) (*deleteTaskOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	provider := sessions.NewProvider()
	defaultSession, err := provider.Default()
	if err != nil {
		return nil, err
	}
	prompter := prompt.New()
	
	return &deleteTaskOpts{
		deleteTaskVars: vars,

		store:   store,
		spinner: termprogress.NewSpinner(),
		prompt:  prompter,
		sess:    provider,
		sel:     selector.NewWorkspaceSelect(prompter, store, ws),
		appCFN:  cloudformation.New(defaultSession),
		getSvcCFN: func(session *awssession.Session) wlDeleter {
			return cloudformation.New(session)
		},
		getECR: func(session *awssession.Session) imageRemover {
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
		return o.validateAppName()
	}

	if o.env != "" {
		return o.validateEnvName()
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

func (o *deleteTaskOpts) validateEnvName() error {
	if o.app != "" {
		if _, err := o.targetEnv(); err != nil {
			return err
		} 
	} else {
		return errNoAppInWorkspace
	}
	return nil
}

func (o *deleteTaskOpts) validateAppName() error {
	if _, err := o.store.GetApplication(o.app); err != nil {
		return fmt.Errorf("get application: %w", err)
	}
	return nil
}

func (o *deleteTaskOpts) targetEnv() (*config.Environment, error) {
	env, err := o.store.GetEnvironment(o.app, o.env)
	if err != nil {
		return nil, fmt.Errorf("get environment %s config: %w", o.env, err)
	}
	return env, nil
}

// Ask prompts for missing information and fills in gaps.
func (o *deleteTaskOpts) Ask() error {
	// Ask task name
	if err := o.askTaskName(); err != nil {
		return err
	}

	if o.skipConfirmation {
		return nil
	}

	// Confirm deletion
	deletePrompt := fmt.Sprintf(fmtTaskDeleteDefaultConfirmPrompt, color.HighlightUserInput(o.name))
	if o.env != "" {
		deletePrompt = fmt.Sprintf(
			fmtTaskDeleteFromEnvConfirmPrompt,
			color.HighlightUserInput(o.name),
			color.HighlightUserInput(o.env),
		)
	}

	deleteConfirmed, err := o.prompt.Confirm(
		deletePrompt,
		taskDeleteConfirmHelp)

	if err != nil {
		return fmt.Errorf("svc delete confirmation prompt: %w", err)
	}
	if !deleteConfirmed {
		return errSvcDeleteCancelled
	}
	return nil
}

func (o *deleteTaskOpts) askTaskName() error {
	return nil
}

func (o *deleteAppOpts) Execute() error {
	
	envs, err := o.store.ListEnvironments(o.name)
	if err != nil {
		return fmt.Errorf("list environments for application %s: %w", o.name, err)
	}
	o.envs = envs

	// Delete tasks from each environment (that is, delete the tasks that were created with each environment's manager role)
	var envTasks []deploy.TaskStackInfo
	for _, env := range envs {
		envSess, err := o.sessProvider.FromRole(env.ManagerRoleARN, env.Region)
		if err != nil {
			return err
		}
		envCF := o.cfn(envSess)
		envECR := o.ecr(envSess)
		envTasks, err = envCF.GetTaskStackInfo(o.name)
		if err != nil {
			return fmt.Errorf("get tasks deployed in environment %s: %w", env.Name, err)
		}
		for _, t := range envTasks {
			o.spinner.Start(fmt.Sprintf("Deleting task %s from environment %s.", t.TaskName(), env.Name))
			if err := envECR.ClearRepository(t.ECRRepoName()); err != nil {
				o.spinner.Stop(log.Serrorf("Error emptying ECR repository for task %s\n", t.TaskName()))
				return fmt.Errorf("empty ECR repository for task %s: %w", t.TaskName(), err)
			}
			if err := envCF.DeleteTask(t); err != nil {
				o.spinner.Stop(log.Serrorf("Error deleting task %s from environment %s.\n", t.TaskName(), t.Env))
				return fmt.Errorf("delete task %s from env %s: %w", t.TaskName(), t.Env, err)
			}
			o.spinner.Stop(log.Ssuccessf("Deleted task %s from environment %s.\n", t.TaskName(), t.Env))
		}
	}

	// Delete tasks in default VPC created with an EnvManagerRole
	defaultSess, err := o.sessProvider.Default()
	if err != nil {
		return err
	}
	defaultECR := o.ecr(defaultSess)
	defaultCFN := o.cfn(defaultSess)
	defaultTasks, err := defaultCFN.GetTaskStackInfo(o.name)
	if err != nil {
		return fmt.Errorf("get tasks deployed in default VPC: %w", err)
	}
	for _, t := range defaultTasks {
		o.spinner.Start(fmt.Sprintf("Deleting environment-dependent task %s from default VPC.", t.TaskName()))
		if err := defaultECR.ClearRepository(t.ECRRepoName()); err != nil {
			o.spinner.Stop(log.Serrorf("Error emptying ECR repository for task %s\n", t.TaskName()))
			return fmt.Errorf("empty ECR repository for task %s: %w", t.TaskName(), err)
		}
		if err := defaultCFN.DeleteTask(t); err != nil {
			o.spinner.Stop(log.Serrorf("Error deleting task %s from default VPC.\n", t.TaskName()))
			return fmt.Errorf("delete task %s: %w", t.TaskName(), err)
		}
		o.spinner.Stop(log.Ssuccessf("Deleted task %s from default VPC.\n", t.TaskName()))
	}
	return nil
}

// Execute attempts to delete a task based on best-effort. If it finds an application
// buildSvcDeleteCmd builds the command to delete application(s).
func buildTaskDeleteCmd() *cobra.Command {
	vars := deleteTaskVars{}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes a one-off task from an application or default cluster.",
		Example: `
  Delete the "test" task from the default cluster.
  /code $ copilot task delete --name test --default

  Delete the "test" task from the prod environment.
  /code $ copilot task delete --name test --env prod

  Delete the "test" task without confirmation prompt.
  /code $ copilot task delete --name test --yes`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeleteSvcOpts(vars)
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
	cmd.Flags().BoolVar(&vars.defaultCluster, taskDefaultFlag, false, "")
	return cmd
}

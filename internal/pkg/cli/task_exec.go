// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/cmd/copilot/template"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/cobra"
)

const (
	useDefaultClusterOption = "None (run in default cluster)"
)

var (
	taskExecTaskPrompt        = fmt.Sprintf("Which %s would you like to execute into?", color.Emphasize("task"))
	taskExecTaskHelpPrompt    = fmt.Sprintf("By default we'll execute into the first %s of the task.", color.Emphasize("essential container"))
	taskExecAppNamePrompt     = fmt.Sprintf("In which %s are you running your %s?", color.Emphasize("application"), color.Emphasize("task"))
	taskExecAppNameHelpPrompt = fmt.Sprintf(`Select the application that your task is deployed to. 
Select %s to execute in a task running in your default cluster instead of any existing application.`, color.Emphasize(useDefaultClusterOption))
	taskExecEnvNamePrompt     = fmt.Sprintf("In which %s are you running your %s?", color.Emphasize("environment"), color.Emphasize("task"))
	taskExecEnvNameHelpPrompt = fmt.Sprintf(`Select the environment that your task is deployed to.
Select %s to execute in a task running in your default cluster instead of any existing environment.`, color.Emphasize(useDefaultClusterOption))
)

type taskExecVars struct {
	execVars
	useDefault bool
}

type taskExecOpts struct {
	taskExecVars
	store              store
	ssmPluginManager   ssmPluginManager
	prompter           prompter
	newTaskSel         func(*session.Session) runningTaskSelector
	configSel          appEnvSelector
	newCommandExecutor func(*session.Session) ecsCommandExecutor
	provider           sessionProvider

	task *awsecs.Task
}

func newTaskExecOpts(vars taskExecVars) (*taskExecOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("task exec"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}

	ssmStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	prompter := prompt.New()
	return &taskExecOpts{
		taskExecVars:     vars,
		store:            ssmStore,
		ssmPluginManager: exec.NewSSMPluginCommand(nil),
		prompter:         prompter,
		newTaskSel: func(sess *session.Session) runningTaskSelector {
			return selector.NewTaskSelector(prompter, ecs.New(sess))
		},
		configSel: selector.NewConfigSelector(prompter, ssmStore),
		newCommandExecutor: func(s *session.Session) ecsCommandExecutor {
			return awsecs.New(s)
		},
		provider: sessProvider,
	}, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *taskExecOpts) Validate() error {
	if o.useDefault && (o.appName != tryReadingAppName() || o.envName != "") {
		return fmt.Errorf("cannot specify both default flag and app or env flags")
	}
	if o.appName != "" {
		if _, err := o.store.GetApplication(o.appName); err != nil {
			return err
		}
		if o.envName != "" {
			if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
				return err
			}
		}
	}
	return validateSSMBinary(o.prompter, o.ssmPluginManager, o.skipConfirmation)
}

// Ask asks for fields that are required but not passed in.
func (o *taskExecOpts) Ask() error {
	if o.useDefault {
		return o.selectTaskInDefaultCluster()
	}
	if o.appName == "" {
		appName, err := o.configSel.Application(taskExecAppNamePrompt, taskExecAppNameHelpPrompt, useDefaultClusterOption)
		if err != nil {
			return fmt.Errorf("select application: %w", err)
		}
		if appName == useDefaultClusterOption {
			o.useDefault = true
			return o.selectTaskInDefaultCluster()
		}
		o.appName = appName
	}
	if o.envName == "" {
		envName, err := o.configSel.Environment(taskExecEnvNamePrompt, taskExecEnvNameHelpPrompt, o.appName, prompt.Option{Value: useDefaultClusterOption})
		if err != nil {
			return fmt.Errorf("select environment: %w", err)
		}
		if envName == useDefaultClusterOption {
			o.useDefault = true
			return o.selectTaskInDefaultCluster()
		}
		o.envName = envName
	}
	return o.selectTaskInAppEnvCluster()
}

// Execute executes a command in a running container.
func (o *taskExecOpts) Execute() error {
	sess, err := o.configSession()
	if err != nil {
		return err
	}
	cluster, container := aws.StringValue(o.task.ClusterArn), aws.StringValue(o.task.Containers[0].Name)
	taskID, err := awsecs.TaskID(aws.StringValue(o.task.TaskArn))
	if err != nil {
		return fmt.Errorf("parse task ARN %s: %w", aws.StringValue(o.task.TaskArn), err)
	}
	log.Infof("Execute %s in container %s in task %s.\n", color.HighlightCode(o.command),
		color.HighlightUserInput(container), color.HighlightResource(taskID))
	if err = o.newCommandExecutor(sess).ExecuteCommand(awsecs.ExecuteCommandInput{
		Cluster:   cluster,
		Command:   o.command,
		Container: container,
		Task:      taskID,
	}); err != nil {
		return fmt.Errorf("execute command %s in container %s: %w", o.command, container, err)
	}
	return nil
}

func (o *taskExecOpts) selectTaskInDefaultCluster() error {
	sess, err := o.provider.Default()
	if err != nil {
		return fmt.Errorf("create default session: %w", err)
	}
	task, err := o.newTaskSel(sess).RunningTask(taskExecTaskPrompt, taskExecTaskHelpPrompt,
		selector.WithDefault(), selector.WithTaskGroup(o.name), selector.WithTaskID(o.taskID))
	if err != nil {
		return fmt.Errorf("select running task in default cluster: %w", err)
	}
	o.task = task
	return nil
}

func (o *taskExecOpts) selectTaskInAppEnvCluster() error {
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return fmt.Errorf("get environment %s: %w", o.envName, err)
	}
	sess, err := o.provider.FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return fmt.Errorf("get session from role %s and region %s: %w", env.ManagerRoleARN, env.Region, err)
	}
	task, err := o.newTaskSel(sess).RunningTask(taskExecTaskPrompt, taskExecTaskHelpPrompt,
		selector.WithAppEnv(o.appName, o.envName), selector.WithTaskGroup(o.name), selector.WithTaskID(o.taskID))
	if err != nil {
		return fmt.Errorf("select running task in environment %s: %w", o.envName, err)
	}
	o.task = task
	return nil
}

func (o *taskExecOpts) configSession() (*session.Session, error) {
	if o.useDefault {
		return o.provider.Default()
	}
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return nil, fmt.Errorf("get environment %s: %w", o.envName, err)
	}
	return o.provider.FromRole(env.ManagerRoleARN, env.Region)
}

// buildTaskExecCmd builds the command for execute a running container in a one-off task.
func buildTaskExecCmd() *cobra.Command {
	var skipPrompt bool
	vars := taskExecVars{}
	cmd := &cobra.Command{
		Use:   "exec",
		Short: "Execute a command in a running container part of a task.",
		Example: `
  Start an interactive bash session with a task in task group "db-migrate" in the "test" environment under the current workspace.
  /code $ copilot task exec -e test -n db-migrate
  Runs the 'cat progress.csv' command in the task prefixed with ID "1848c38" part of the "db-migrate" task group.
  /code $ copilot task exec --name db-migrate --task-id 1848c38 --command "cat progress.csv"
  Start an interactive bash session with a task prefixed with ID "38c3818" in the default cluster.
  /code $ copilot task exec --default --task-id 38c3818`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newTaskExecOpts(vars)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed(yesFlag) {
				opts.skipConfirmation = aws.Bool(false)
				if skipPrompt {
					opts.skipConfirmation = aws.Bool(true)
				}
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", nameFlagDescription)
	cmd.Flags().StringVarP(&vars.command, commandFlag, commandFlagShort, defaultCommand, execCommandFlagDescription)
	cmd.Flags().StringVar(&vars.taskID, taskIDFlag, "", taskIDFlagDescription)
	cmd.Flags().BoolVar(&vars.useDefault, taskDefaultFlag, false, taskExecDefaultFlagDescription)
	cmd.Flags().BoolVar(&skipPrompt, yesFlag, false, execYesFlagDescription)

	cmd.SetUsageTemplate(template.Usage)
	return cmd
}

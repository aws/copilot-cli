// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"

	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/cmd/copilot/template"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/cobra"
)

const (
	svcExecNamePrompt     = "Into which service would you like to execute?"
	svcExecNameHelpPrompt = `Copilot runs your command in one of your chosen service's tasks.
The task is chosen at random, and the first essential container is used.`

	ssmPluginInstallPrompt = `Looks like the Session Manager plugin is not installed yet.
Would you like to install the plugin to execute into the container?`
	ssmPluginInstallPromptHelp = `You must install the Session Manager plugin on your local machine to be able to execute into the container
See https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html`
	ssmPluginUpdatePrompt = `Looks like the Session Manager plugin is using version %s.
Would you like to update it to the latest version %s?`
)

var (
	errSSMPluginCommandInstallCancelled = errors.New("ssm plugin install cancelled")
)

type svcExecOpts struct {
	execVars
	store              store
	sel                deploySelector
	newSvcDescriber    func(*session.Session) serviceDescriber
	newCommandExecutor func(*session.Session) ecsCommandExecutor
	ssmPluginManager   ssmPluginManager
	prompter           prompter
	sessProvider       sessionProvider
	// Override in unit test
	randInt func(int) int
}

func newSvcExecOpts(vars execVars) (*svcExecOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("svc exec"))
	defaultSession, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}
	ssmStore := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	deployStore, err := deploy.NewStore(sessProvider, ssmStore)
	if err != nil {
		return nil, fmt.Errorf("connect to deploy store: %w", err)
	}
	return &svcExecOpts{
		execVars: vars,
		store:    ssmStore,
		sel:      selector.NewDeploySelect(prompt.New(), ssmStore, deployStore),
		newSvcDescriber: func(s *session.Session) serviceDescriber {
			return ecs.New(s)
		},
		newCommandExecutor: func(s *session.Session) ecsCommandExecutor {
			return awsecs.New(s)
		},
		randInt: func(x int) int {
			return rand.Intn(x)
		},
		ssmPluginManager: exec.NewSSMPluginCommand(nil),
		prompter:         prompt.New(),
		sessProvider:     sessProvider,
	}, nil
}

// Validate returns an error for any invalid optional flags.
func (o *svcExecOpts) Validate() error {
	return validateSSMBinary(o.prompter, o.ssmPluginManager, o.skipConfirmation)
}

// Ask prompts for and validates any required flags.
func (o *svcExecOpts) Ask() error {
	if err := o.validateOrAskApp(); err != nil {
		return err
	}
	if err := o.validateAndAskSvcEnvName(); err != nil {
		return err
	}
	return nil
}

// Execute executes a command in a running container.
func (o *svcExecOpts) Execute() error {
	wkld, err := o.store.GetWorkload(o.appName, o.name)
	if err != nil {
		return fmt.Errorf("get workload: %w", err)
	}
	if wkld.Type == manifestinfo.RequestDrivenWebServiceType {
		return fmt.Errorf("executing a command in a running container part of a service is not supported for services with type: '%s'", manifestinfo.RequestDrivenWebServiceType)
	}
	sess, err := o.envSession()
	if err != nil {
		return err
	}
	svcDesc, err := o.newSvcDescriber(sess).DescribeService(o.appName, o.envName, o.name)
	if err != nil {
		return fmt.Errorf("describe ECS service for %s in environment %s: %w", o.name, o.envName, err)
	}
	taskID, err := o.selectTask(awsecs.FilterRunningTasks(svcDesc.Tasks))
	if err != nil {
		return err
	}
	container := o.selectContainer()
	log.Infof("Execute %s in container %s in task %s.\n", color.HighlightCode(o.command),
		color.HighlightUserInput(container), color.HighlightResource(taskID))
	if err = o.newCommandExecutor(sess).ExecuteCommand(awsecs.ExecuteCommandInput{
		Cluster:   svcDesc.ClusterName,
		Command:   o.command,
		Container: container,
		Task:      taskID,
	}); err != nil {
		var errExecCmd *awsecs.ErrExecuteCommand
		if errors.As(err, &errExecCmd) {
			log.Errorf("Failed to execute command %s. Is %s set in your manifest?\n", o.command, color.HighlightCode("exec: true"))
		}
		return fmt.Errorf("execute command %s in container %s: %w", o.command, container, err)
	}
	return nil
}

func (o *svcExecOpts) validateOrAskApp() error {
	if o.appName != "" {
		_, err := o.store.GetApplication(o.appName)
		return err
	}
	app, err := o.sel.Application(svcAppNamePrompt, wkldAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

func (o *svcExecOpts) validateAndAskSvcEnvName() error {
	if o.envName != "" {
		if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
			return err
		}
	}

	if o.name != "" {
		if _, err := o.store.GetService(o.appName, o.name); err != nil {
			return err
		}
	}

	// Note: we let prompter handle the case when there is only option for user to choose from.
	// This is naturally the case when `o.envName != "" && o.name != ""`.
	deployedService, err := o.sel.DeployedService(svcExecNamePrompt, svcExecNameHelpPrompt, o.appName, selector.WithEnv(o.envName), selector.WithName(o.name))
	if err != nil {
		return fmt.Errorf("select deployed service for application %s: %w", o.appName, err)
	}
	o.name = deployedService.Name
	o.envName = deployedService.Env
	return nil
}

func (o *svcExecOpts) envSession() (*session.Session, error) {
	env, err := o.store.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return nil, fmt.Errorf("get environment %s: %w", o.envName, err)
	}
	return o.sessProvider.FromRole(env.ManagerRoleARN, env.Region)
}

func (o *svcExecOpts) selectTask(tasks []*awsecs.Task) (string, error) {
	if len(tasks) == 0 {
		return "", fmt.Errorf("found no running task for service %s in environment %s", o.name, o.envName)
	}
	if o.taskID != "" {
		for _, task := range tasks {
			taskID, err := awsecs.TaskID(aws.StringValue(task.TaskArn))
			if err != nil {
				return "", err
			}
			if strings.HasPrefix(taskID, o.taskID) {
				return taskID, nil
			}
		}
		return "", fmt.Errorf("found no running task whose ID is prefixed with %s", o.taskID)
	}
	taskID, err := awsecs.TaskID(aws.StringValue(tasks[o.randInt(len(tasks))].TaskArn))
	if err != nil {
		return "", err
	}
	return taskID, nil
}

func (o *svcExecOpts) selectContainer() string {
	if o.containerName != "" {
		return o.containerName
	}
	// The first essential container is named with the workload name.
	return o.name
}

func validateSSMBinary(prompt prompter, manager ssmPluginManager, skipConfirmation *bool) error {
	if skipConfirmation != nil && !aws.BoolValue(skipConfirmation) {
		return nil
	}
	err := manager.ValidateBinary()
	if err == nil {
		return nil
	}
	switch v := err.(type) {
	case *exec.ErrSSMPluginNotExist:
		// If ssm plugin is not install, prompt users to install the plugin.
		if skipConfirmation == nil {
			confirmInstall, err := prompt.Confirm(ssmPluginInstallPrompt, ssmPluginInstallPromptHelp)
			if err != nil {
				return fmt.Errorf("prompt to confirm installing the plugin: %w", err)
			}
			if !confirmInstall {
				return errSSMPluginCommandInstallCancelled
			}
		}
		if err := manager.InstallLatestBinary(); err != nil {
			return fmt.Errorf("install ssm plugin: %w", err)
		}
		return nil
	case *exec.ErrOutdatedSSMPlugin:
		// If ssm plugin is not up to date, prompt users to update the plugin.
		if skipConfirmation == nil {
			confirmUpdate, err := prompt.Confirm(
				fmt.Sprintf(ssmPluginUpdatePrompt, v.CurrentVersion, v.LatestVersion), "")
			if err != nil {
				return fmt.Errorf("prompt to confirm updating the plugin: %w", err)
			}
			if !confirmUpdate {
				log.Infof(`Alright, we won't update the Session Manager plugin.
	It might fail to execute if it is not using the latest plugin.
	`)
				return nil
			}
		}
		if err := manager.InstallLatestBinary(); err != nil {
			return fmt.Errorf("update ssm plugin: %w", err)
		}
		return nil
	default:
		log.Errorf(`Failed to validate the Session Manager plugin. Please install or make sure your SSM plugin is up-to-date:
https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html
`)
		return fmt.Errorf("validate ssm plugin: %w", err)
	}
}

// buildSvcExecCmd builds the command for execute a running container in a service.
func buildSvcExecCmd() *cobra.Command {
	vars := execVars{}
	var skipPrompt bool
	cmd := &cobra.Command{
		Use:   "exec",
		Short: "Execute a command in a running container part of a service.",
		Example: `
  Start an interactive bash session with a task part of the "frontend" service.
  /code $ copilot svc exec -a my-app -e test -n frontend
  Runs the 'ls' command in the task prefixed with ID "8c38184" within the "backend" service.
  /code $ copilot svc exec -a my-app -e test --name backend --task-id 8c38184 --command "ls"`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newSvcExecOpts(vars)
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
	cmd.Flags().StringVar(&vars.containerName, containerFlag, "", containerFlagDescription)
	cmd.Flags().BoolVar(&skipPrompt, yesFlag, false, execYesFlagDescription)

	cmd.SetUsageTemplate(template.Usage)
	return cmd
}

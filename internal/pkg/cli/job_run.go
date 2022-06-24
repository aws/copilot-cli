// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	jobRunner "github.com/aws/copilot-cli/internal/pkg/runner/job"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

const (
	jobNamePrompt     = "Which Job would you like to run?"
	jobNameHelpPrompt = "The selected Job will be run"
	envPrompt         = "Which Environment do you want to run your job in?"
	envHelpPrompt     = "The Environment where your job will run"
)

type jobRunVars struct {
	appName string
	envName string
	jobName string
}

type jobRunOpts struct {
	jobRunVars

	configStore store
	sel         deploySelector
	ws          wsSelector

	// cached variables.
	targetEnv *config.Environment

	runner     Runner
	initRunner func()
}

func newJobRunOpts(vars jobRunVars) (*jobRunOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("job deploy"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}
	configStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))

	deployStore, err := deploy.NewStore(sessProvider, configStore)
	if err != nil {
		return nil, fmt.Errorf("connect to deploy store: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	prompter := prompt.New()

	opts := &jobRunOpts{
		jobRunVars:  vars,
		configStore: configStore,
		sel:         selector.NewDeploySelect(prompt.New(), configStore, deployStore),
		ws:          selector.NewLocalWorkloadSelector(prompter, configStore, ws),
	}

	opts.initRunner = func() {
		opts.runner = jobRunner.NewJobRunner(&jobRunner.JobRunnerConfig{
			Sess: defaultSess,
			Env:  opts.envName,
			App:  opts.appName,
			Job:  opts.jobName,
		})
	}
	return opts, nil
}

// Validate is a no-op for this command.
// it's a no-op because all 3 flags are required, and `Validate` only validate optional flags.
func (o *jobRunOpts) Validate() error {
	return nil
}

// Ask prompts for and validates any required flags.
func (o *jobRunOpts) Ask() error {
	if err := o.validateOrAskApp(); err != nil {
		return err
	}
	if err := o.askJobName(); err != nil {
		return err
	}
	if err := o.askEnvName(); err != nil {
		return err
	}
	return nil
}

func (o *jobRunOpts) validateOrAskApp() error {

	if o.appName != "" {
		_, err := o.configStore.GetApplication(o.appName)
		return err
	}

	app, err := o.sel.Application(jobAppNamePrompt, svcAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

func (o *jobRunOpts) askJobName() error {
	if o.jobName != "" {
		if _, err := o.configStore.GetJob(o.appName, o.jobName); err != nil {
			return err
		}
		return nil
	}

	name, err := o.ws.Job("Select a job from your workspace", "")
	if err != nil {
		return fmt.Errorf("select job: %w", err)
	}
	o.jobName = name
	return nil
}

func (o *jobRunOpts) askEnvName() error {
	if o.envName != "" {
		if _, err := o.getTargetEnv(); err != nil {
			return err
		}
		return nil
	}

	name, err := o.ws.Environment("Select an environment", "", o.appName)
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.envName = name
	return nil
}

func (o *jobRunOpts) getTargetEnv() (*config.Environment, error) {
	if o.targetEnv != nil {
		return o.targetEnv, nil
	}
	env, err := o.configStore.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return nil, err
	}
	o.targetEnv = env
	return o.targetEnv, nil
}

func (o *jobRunOpts) Execute() error {
	o.initRunner()
	err := o.runner.Run()

	if err != nil {
		return fmt.Errorf("job execution %s: %w", o.jobName, err)
	}
	return nil
}

func buildJobRunCmd() *cobra.Command {
	vars := jobRunVars{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Runs a job in an environment",
		Long:  "Runs a job in an environment",
		Example: `
		Runs a job named "report-gen" in an application named "report" to a "test" environment
		/code $ copilot job run -a report -n report-gen -e test
		`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newJobRunOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.jobName, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)

	return cmd
}

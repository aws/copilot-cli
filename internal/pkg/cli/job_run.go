// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/aws/stepfunctions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/runner/jobrunner"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type jobRunVars struct {
	appName string
	envName string
	jobName string
}

type jobRunOpts struct {
	jobRunVars

	configStore store
	sel         configSelector
	ws          wsEnvironmentsLister

	// cached variables.
	targetEnv    *config.Environment
	sessProvider *sessions.Provider

	newRunner                  func() (runner, error)
	newEnvCompatibilityChecker func() (versionCompatibilityChecker, error)
}

func newJobRunOpts(vars jobRunVars) (*jobRunOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("job deploy"))

	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}
	configStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}
	prompter := prompt.New()

	opts := &jobRunOpts{
		jobRunVars: vars,

		configStore: configStore,
		sel:         selector.NewConfigSelector(prompter, configStore),
		ws:          ws,

		sessProvider: sessProvider,
	}
	opts.newRunner = func() (runner, error) {
		sess, err := opts.envSession()
		if err != nil {
			return nil, err
		}

		return jobrunner.New(&jobrunner.Config{
			App: opts.appName,
			Env: opts.envName,
			Job: opts.jobName,

			CFN:          cloudformation.New(sess),
			StateMachine: stepfunctions.New(sess),
		}), nil
	}
	opts.newEnvCompatibilityChecker = func() (versionCompatibilityChecker, error) {
		envDescriber, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
			App:         opts.appName,
			Env:         opts.envName,
			ConfigStore: opts.configStore,
		})
		if err != nil {
			return nil, fmt.Errorf("new environment compatibility checker: %v", err)
		}
		return envDescriber, nil
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

// Execute runs the "job run" command.
func (o *jobRunOpts) Execute() error {
	if err := o.validateEnvCompatible(); err != nil {
		return err
	}
	runner, err := o.newRunner()
	if err != nil {
		return err
	}
	if err := runner.Run(); err != nil {
		return fmt.Errorf("execute job %q: %w", o.jobName, err)
	}
	log.Successf("Invoked job %q successfully\n", o.jobName)
	return nil
}

func (o *jobRunOpts) validateOrAskApp() error {
	if o.appName != "" {
		_, err := o.configStore.GetApplication(o.appName)
		return err
	}
	app, err := o.sel.Application(jobAppNamePrompt, wkldAppNameHelpPrompt)
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

	name, err := o.sel.Job("Which job would you like to invoke?", "", o.appName)
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

	name, err := o.sel.Environment("Which environment?", "", o.appName)
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

func (o *jobRunOpts) envSession() (*session.Session, error) {
	env, err := o.getTargetEnv()
	if err != nil {
		return nil, err
	}
	return o.sessProvider.FromRole(env.ManagerRoleARN, env.Region)
}

func (o *jobRunOpts) validateEnvCompatible() error {
	envStack, err := o.newEnvCompatibilityChecker()
	if err != nil {
		return err
	}
	return validateMinEnvVersion(o.ws, envStack, o.appName, o.envName, template.JobRunMinEnvVersion, "job run")
}

func buildJobRunCmd() *cobra.Command {
	vars := jobRunVars{}

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Invoke a job in an environment.",
		Long:  "Invoke a job in an environment.",
		Example: `
  Run a job named "report-gen" in an application named "report" within a "test" environment
  /code $ copilot job run -a report -n report-gen -e test`,
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

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/exec"

	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	jobPackageJobNamePrompt = "Which job would you like to generate a CloudFormation template for?"
	jobPackageEnvNamePrompt = "Which environment would you like to package this stack for?"
)

type packageJobVars struct {
	name      string
	envName   string
	appName   string
	tag       string
	outputDir string
}

type packageJobOpts struct {
	packageJobVars

	// Interfaces to interact with dependencies.
	ws              wsJobDirReader
	store           store
	runner          runner
	sel             wsSelector
	prompt          prompter
	stackSerializer func(mft interface{}, env *config.Environment, app *config.Application, rc stack.RuntimeConfig) (stackSerializer, error)

	// Subcommand implementing svc_package's Execute()
	packageCmd    actionCommand
	newPackageCmd func(*packageJobOpts)
}

func newPackageJobOpts(vars packageJobVars) (*packageJobOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	store, err := config.NewStore()
	if err != nil {
		logFriendlyTextIfRegionIsMissing(err)
		return nil, fmt.Errorf("connect to config store: %w", err)
	}
	p := sessions.NewProvider()
	sess, err := p.Default()
	if err != nil {
		return nil, fmt.Errorf("retrieve default session: %w", err)
	}
	prompter := prompt.New()
	opts := &packageJobOpts{
		packageJobVars: vars,
		ws:             ws,
		store:          store,
		runner:         exec.NewCmd(),
		sel:            selector.NewWorkspaceSelect(prompter, store, ws),
		prompt:         prompter,
	}

	opts.stackSerializer = func(mft interface{}, env *config.Environment, app *config.Application, rc stack.RuntimeConfig) (stackSerializer, error) {
		var serializer stackSerializer
		jobMft := mft.(*manifest.ScheduledJob)
		serializer, err := stack.NewScheduledJob(jobMft, env.Name, app.Name, rc)
		if err != nil {
			return nil, fmt.Errorf("init scheduled job stack serializer: %w", err)
		}
		return serializer, nil
	}

	opts.newPackageCmd = func(o *packageJobOpts) {
		opts.packageCmd = &packageSvcOpts{
			packageSvcVars: packageSvcVars{
				name:      o.name,
				envName:   o.envName,
				appName:   o.appName,
				tag:       imageTagFromGit(o.runner, o.tag),
				outputDir: o.outputDir,
			},
			runner:           o.runner,
			initAddonsClient: initPackageAddonsClient,
			ws:               ws,
			store:            o.store,
			appCFN:           cloudformation.New(sess),
			stackWriter:      os.Stdout,
			paramsWriter:     ioutil.Discard,
			addonsWriter:     ioutil.Discard,
			fs:               &afero.Afero{Fs: afero.NewOsFs()},
			stackSerializer:  o.stackSerializer,
			newEndpointGetter: func(app, env string) (endpointGetter, error) {
				d, err := describe.NewEnvDescriber(describe.NewEnvDescriberConfig{
					App:         app,
					Env:         env,
					ConfigStore: store,
				})
				if err != nil {
					return nil, fmt.Errorf("new env describer for environment %s in app %s: %v", env, app, err)
				}
				return d, nil
			},
		}
	}
	return opts, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *packageJobOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	if o.name != "" {
		names, err := o.ws.JobNames()
		if err != nil {
			return fmt.Errorf("list jobs in the workspace: %w", err)
		}
		if !contains(o.name, names) {
			return fmt.Errorf("job '%s' does not exist in the workspace", o.name)
		}
	}
	if o.envName != "" {
		if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts the user for any missing required fields.
func (o *packageJobOpts) Ask() error {
	if err := o.askJobName(); err != nil {
		return err
	}
	if err := o.askEnvName(); err != nil {
		return err
	}
	return nil
}

// Execute prints the CloudFormation template of the application for the environment.
func (o *packageJobOpts) Execute() error {
	o.newPackageCmd(o)
	return o.packageCmd.Execute()
}

func (o *packageJobOpts) askJobName() error {
	if o.name != "" {
		return nil
	}

	name, err := o.sel.Job(jobPackageJobNamePrompt, "")
	if err != nil {
		return fmt.Errorf("select job: %w", err)
	}
	o.name = name
	return nil
}

func (o *packageJobOpts) askEnvName() error {
	if o.envName != "" {
		return nil
	}

	name, err := o.sel.Environment(jobPackageEnvNamePrompt, "", o.appName)
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.envName = name
	return nil
}

// buildJobPackageCmd builds the command for printing a job's CloudFormation template.
func buildJobPackageCmd() *cobra.Command {
	vars := packageJobVars{}
	cmd := &cobra.Command{
		Use:   "package",
		Short: "Prints the AWS CloudFormation template of a job.",
		Long:  `Prints the CloudFormation template used to deploy a job to an environment.`,
		Example: `
  Print the CloudFormation template for the "report-generator" job parametrized for the "test" environment.
  /code $ copilot job package -n report-generator -e test

  Write the CloudFormation stack and configuration to a "infrastructure/" sub-directory instead of printing.
  /code $ copilot job package -n report-generator -e test --output-dir ./infrastructure
  /code $ ls ./infrastructure
  /code report-generator-test.stack.yml      report-generator-test.params.yml`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newPackageJobOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVar(&vars.tag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringVar(&vars.outputDir, stackOutputDirFlag, "", stackOutputDirFlagDescription)
	return cmd
}

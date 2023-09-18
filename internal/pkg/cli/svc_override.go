// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"slices"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type overrideWorkloadVars struct {
	overrideVars
	envName string // Optional.
}

type overrideWorkloadOpts struct {
	envName string
	*overrideOpts

	// Interfaces to interact with dependencies.
	ws                wsWlDirReader
	wsPrompt          wsSelector
	validateOrAskName func() error
}

func newOverrideWorkloadOpts(vars overrideWorkloadVars) (*overrideWorkloadOpts, error) {
	fs := afero.NewOsFs()
	ws, err := workspace.Use(fs)
	if err != nil {
		return nil, err
	}

	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("svc override"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}
	cfgStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	vars.requiresEnv = true
	prompt := prompt.New()
	cmd := &overrideWorkloadOpts{
		envName: vars.envName,
		overrideOpts: &overrideOpts{
			overrideVars: vars.overrideVars,
			fs:           fs,
			cfgStore:     cfgStore,
			prompt:       prompt,
			cfnPrompt:    selector.NewCFNSelector(prompt),
			spinner:      termprogress.NewSpinner(log.DiagnosticWriter),
		},
		ws:       ws,
		wsPrompt: selector.NewLocalWorkloadSelector(prompt, cfgStore, ws, selector.OnlyInitializedWorkloads),
	}
	return cmd, nil
}

func newOverrideSvcOpts(vars overrideWorkloadVars) (*overrideWorkloadOpts, error) {
	cmd, err := newOverrideWorkloadOpts(vars)
	if err != nil {
		return nil, err
	}
	cmd.validateOrAskName = cmd.validateOrAskServiceName
	cmd.overrideOpts.packageCmd = cmd.newSvcPackageCmd
	return cmd, nil
}

// Validate returns an error for any invalid optional flags.
func (o *overrideWorkloadOpts) Validate() error {
	if err := o.overrideOpts.Validate(); err != nil {
		return err
	}
	return o.validateEnvName()
}

// Ask prompts for and validates any required flags.
func (o *overrideWorkloadOpts) Ask() error {
	if err := o.validateOrAskName(); err != nil {
		return err
	}
	return o.overrideOpts.Ask()
}

// Execute writes IaC override files to the local workspace.
// This method assumes that the IaC tool chosen by the user is valid.
func (o *overrideWorkloadOpts) Execute() error {
	o.overrideOpts.dir = func() string {
		return o.ws.WorkloadOverridesPath(o.name)
	}
	return o.overrideOpts.Execute()
}

func (o *overrideWorkloadOpts) validateEnvName() error {
	if o.envName == "" {
		return nil
	}
	_, err := o.cfgStore.GetEnvironment(o.appName, o.envName)
	if err != nil {
		return fmt.Errorf("get environment %q configuration: %v", o.envName, err)
	}
	return nil
}

func (o *overrideWorkloadOpts) validateOrAskServiceName() error {
	if o.name == "" {
		return o.askServiceName()
	}
	return o.validateServiceName()
}

func (o *overrideWorkloadOpts) validateServiceName() error {
	names, err := o.ws.ListServices()
	if err != nil {
		return fmt.Errorf("list services in the workspace: %v", err)
	}
	if !slices.Contains(names, o.name) {
		return fmt.Errorf("service %q does not exist in the workspace", o.name)
	}
	return nil
}

func (o *overrideWorkloadOpts) askServiceName() error {
	name, err := o.wsPrompt.Service("Which service's resources would you like to override?", "")
	if err != nil {
		return fmt.Errorf("select service name from workspace: %v", err)
	}
	o.name = name
	return nil
}

func (o *overrideWorkloadOpts) newSvcPackageCmd(tplBuf stringWriteCloser) (executor, error) {
	envName, err := o.targetEnvName()
	if err != nil {
		return nil, err
	}
	cmd, err := newPackageSvcOpts(packageSvcVars{
		name:    o.name,
		envName: envName,
		appName: o.appName,
	})
	if err != nil {
		return nil, err
	}
	cmd.templateWriter = tplBuf
	return cmd, nil
}

// targetEnvName returns the name of the environment to use when running "svc package".
// If the user does not explicitly provide an environment, default to a random environment.
func (o *overrideWorkloadOpts) targetEnvName() (string, error) {
	if o.envName != "" {
		return o.envName, nil
	}
	envs, err := o.cfgStore.ListEnvironments(o.appName)
	if err != nil {
		return "", fmt.Errorf("list environments in application %q: %v", o.appName, err)
	}
	if len(envs) == 0 {
		return "", fmt.Errorf("no environments found in application %q", o.appName)
	}
	return envs[0].Name, nil
}

func buildSvcOverrideCmd() *cobra.Command {
	vars := overrideWorkloadVars{}
	cmd := &cobra.Command{
		Use:   "override",
		Short: "Override the AWS CloudFormation template of a service.",
		Long: `Scaffold Infrastructure as Code (IaC) extension files for a service. 
The generated files allow you to extend and override the Copilot-generated AWS CloudFormation template.
You can edit the files to change existing resource properties, delete 
or add new resources to the service's template.`,
		Example: `
  Create a new Cloud Development Kit application to override the "frontend" service template.
  /code $ copilot svc override -n frontend --tool cdk`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newOverrideSvcOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", overrideEnvFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVar(&vars.iacTool, iacToolFlag, "", iacToolFlagDescription)
	cmd.Flags().StringVar(&vars.cdkLang, cdkLanguageFlag, typescriptCDKLang, cdkLanguageFlagDescription)
	cmd.Flags().BoolVar(&vars.skipResources, skipResourcesFlag, false, skipResourcesFlagDescription)
	return cmd
}

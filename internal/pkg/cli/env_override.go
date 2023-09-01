// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
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

type overrideEnvOpts struct {
	*overrideOpts

	// Interfaces to interact with dependencies.
	ws wsEnvironmentReader
}

func newOverrideEnvOpts(vars overrideVars) (*overrideEnvOpts, error) {
	fs := afero.NewOsFs()
	ws, err := workspace.Use(fs)
	if err != nil {
		return nil, err
	}

	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("env override"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %v", err)
	}
	cfgStore := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	vars.requiresEnv = true
	prompt := prompt.New()
	cmd := &overrideEnvOpts{
		overrideOpts: &overrideOpts{
			overrideVars: vars,
			fs:           fs,
			cfgStore:     cfgStore,
			prompt:       prompt,
			cfnPrompt:    selector.NewCFNSelector(prompt),
			spinner:      termprogress.NewSpinner(log.DiagnosticWriter),
		},
		ws: ws,
	}
	cmd.overrideOpts.packageCmd = cmd.newEnvPackageCmd
	return cmd, nil
}

// Validate returns an error for any invalid optional flags.
func (o *overrideEnvOpts) Validate() error {
	if err := o.overrideOpts.Validate(); err != nil {
		return err
	}
	return o.validateName()
}

// Ask prompts for and validates any required flags.
func (o *overrideEnvOpts) Ask() error {
	if err := o.assignEnvName(); err != nil {
		return err
	}
	return o.overrideOpts.Ask()
}

// Execute writes IaC override files to the local workspace.
// This method assumes that the IaC tool chosen by the user is valid.
func (o *overrideEnvOpts) Execute() error {
	o.overrideOpts.dir = func() string {
		return o.ws.EnvOverridesPath()
	}
	return o.overrideOpts.Execute()
}

func (o *overrideEnvOpts) validateName() error {
	if o.name == "" {
		return nil
	}
	names, err := o.ws.ListEnvironments()
	if err != nil {
		return fmt.Errorf("list environments in the workspace: %v", err)
	}
	if !slices.Contains(names, o.name) {
		return fmt.Errorf("environment %q does not exist in the workspace", o.name)
	}
	return nil
}

func (o *overrideEnvOpts) newEnvPackageCmd(tplBuf stringWriteCloser) (executor, error) {
	cmd, err := newPackageEnvOpts(packageEnvVars{
		name:    o.name,
		appName: o.appName,
	})
	if err != nil {
		return nil, err
	}
	cmd.tplWriter = tplBuf
	return cmd, nil
}

// If the user does not explicitly provide an environment, default to a random environment.
func (o *overrideEnvOpts) assignEnvName() error {
	if o.name != "" {
		return nil
	}
	envs, err := o.ws.ListEnvironments()
	if err != nil {
		return fmt.Errorf("list environments in the workspace: %v", err)
	}
	if len(envs) == 0 {
		return errors.New("no environments found in the workspace")
	}
	o.name = envs[0]
	return nil
}

func buildEnvOverrideCmd() *cobra.Command {
	vars := overrideVars{}
	cmd := &cobra.Command{
		Use:   "override",
		Short: "Override the AWS CloudFormation template of environments.",
		Long: `Scaffold Infrastructure as Code (IaC) extension files for environments. 
The generated files allow you to extend and override the Copilot-generated AWS CloudFormation template.
You can edit the files to change existing resource properties, delete 
or add new resources to an environment's template.`,
		Example: `
  Create a new Cloud Development Kit application to override environment templates.
  /code $ copilot env override --tool cdk`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newOverrideEnvOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", overrideEnvFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVar(&vars.iacTool, iacToolFlag, "", iacToolFlagDescription)
	cmd.Flags().StringVar(&vars.cdkLang, cdkLanguageFlag, typescriptCDKLang, cdkLanguageFlagDescription)
	cmd.Flags().BoolVar(&vars.skipResources, skipResourcesFlag, false, skipResourcesFlagDescription)
	return cmd
}

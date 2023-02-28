// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	// IaC options for overrides.
	cdkIacToolkit = "cdk"

	// IaC toolkit configuration.
	typescriptCDKLang = "typescript"
)

var iacToolkits = []string{
	cdkIacToolkit,
}

type overrideVars struct {
	name    string
	envName string
	appName string
	iacTool string

	// CDK override engine flags.
	cdkLang string
}

type overrideSvcOpts struct {
	overrideVars

	// Interfaces to interact with dependencies.
	ws wsWlDirReader
	fs afero.Fs
}

func newOverrideSvcOpts(vars overrideVars) (*overrideSvcOpts, error) {
	fs := afero.NewOsFs()
	ws, err := workspace.Use(fs)
	if err != nil {
		return nil, err
	}

	return &overrideSvcOpts{
		overrideVars: vars,
		ws:           ws,
		fs:           fs,
	}, nil
}

// Validate returns an error for any invalid optional flags.
func (o *overrideSvcOpts) Validate() error {
	return nil
}

// Ask prompts for and validates any required flags.
func (o *overrideSvcOpts) Ask() error {
	return nil
}

// Execute writes IaC override files to the local workspace.
func (o *overrideSvcOpts) Execute() error {
	return nil
}

// RecommendActions prints optional follow-up actions.
func (o *overrideSvcOpts) RecommendActions() error {
	return nil
}

func buildSvcOverrideCmd() *cobra.Command {
	vars := overrideVars{}
	cmd := &cobra.Command{
		Hidden: true,
		Use:    "override",
		Short:  "Override the AWS CloudFormation template of a service.",
		Long: `Scaffold Infrastructure as Code patch files. 
Customize the patch files to change resource properties, delete 
or add new resources to the service's AWS CloudFormation template.`,
		Example: `
  Create a new Cloud Development Kit application to override the "frontend" service template.
  /code $ copilot svc override -n frontend -e test --toolkit cdk`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newOverrideSvcOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVar(&vars.iacTool, iacToolFlag, "", iacToolFlagDescription)
	cmd.Flags().StringVar(&vars.cdkLang, cdkLanguageFlag, typescriptCDKLang, cdkLanguageFlagDescription)
	return cmd
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"slices"

	"github.com/spf13/cobra"
)

func newOverrideJobOpts(vars overrideWorkloadVars) (*overrideWorkloadOpts, error) {
	cmd, err := newOverrideWorkloadOpts(vars)
	if err != nil {
		return nil, err
	}
	cmd.validateOrAskName = cmd.validateOrAskJobName
	cmd.overrideOpts.packageCmd = cmd.newSvcPackageCmd // "job package" uses "svc package" under the hood.
	return cmd, nil
}

func (o *overrideWorkloadOpts) validateOrAskJobName() error {
	if o.name == "" {
		return o.askJobName()
	}
	return o.validateJobName()
}

func (o *overrideWorkloadOpts) askJobName() error {
	name, err := o.wsPrompt.Job("Which job's resources would you like to override?", "")
	if err != nil {
		return fmt.Errorf("select job name from workspace: %v", err)
	}
	o.name = name
	return nil
}

func (o *overrideWorkloadOpts) validateJobName() error {
	names, err := o.ws.ListJobs()
	if err != nil {
		return fmt.Errorf("list jobs in the workspace: %v", err)
	}
	if !slices.Contains(names, o.name) {
		return fmt.Errorf("job %q does not exist in the workspace", o.name)
	}
	return nil
}

func buildJobOverrideCmd() *cobra.Command {
	vars := overrideWorkloadVars{}
	cmd := &cobra.Command{
		Use:   "override",
		Short: "Override the AWS CloudFormation template of a job.",
		Long: `Scaffold Infrastructure as Code (IaC) extension files for a job. 
The generated files allow you to extend and override the Copilot-generated AWS CloudFormation template.
You can edit the files to change existing resource properties, delete 
or add new resources to the job's template.`,
		Example: `
  Create a new Cloud Development Kit application to override the "report" job template.
  /code $ copilot job override -n report --tool cdk`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newOverrideJobOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", overrideEnvFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVar(&vars.iacTool, iacToolFlag, "", iacToolFlagDescription)
	cmd.Flags().StringVar(&vars.cdkLang, cdkLanguageFlag, typescriptCDKLang, cdkLanguageFlagDescription)
	cmd.Flags().BoolVar(&vars.skipResources, skipResourcesFlag, false, skipResourcesFlagDescription)
	return cmd
}

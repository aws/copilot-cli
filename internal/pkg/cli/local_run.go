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
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

type localRunVars struct {
	wkldName string
	appName  string
	envName  string
}

type localRunOpts struct {
	localRunVars

	store store
	ws    wsWlDirReader
	sel   wsSelector
}

func newLocalRunOpts(vars localRunVars) (*localRunOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("local run"))
	defaultSess, err := sessProvider.Default()
	if err != nil {
		return nil, err
	}

	store := config.NewSSMStore(identity.New(defaultSess), ssm.New(defaultSess), aws.StringValue(defaultSess.Config.Region))
	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}

	opts := &localRunOpts{
		localRunVars: vars,

		store: store,
		ws:    ws,
		sel:   selector.NewLocalWorkloadSelector(prompt.New(), store, ws),
	}
	return opts, nil
}

func (o *localRunOpts) Validate() error {
	if o.appName == "" {
		return errNoAppInWorkspace
	}
	_, err := o.store.GetApplication(o.appName)
	if err != nil {
		return fmt.Errorf("get application %s: %w", o.appName, err)
	}
	return nil
}

func (o *localRunOpts) Ask() error {
	if err := o.validateOrAskWorkloadName(); err != nil {
		return err
	}
	if err := o.validateOrAskEnvName(); err != nil {
		return err
	}
	return nil
}

func (o *localRunOpts) Execute() error {
	//TODO: Get build information from the manifest and task definition for services
	//By username:varun359

	return nil
}

func (o *localRunOpts) validateOrAskEnvName() error {
	if o.envName != "" {
		return o.validateEnvName()
	}

	name, err := o.sel.Environment("Select an environment", "", o.appName)
	if err != nil {
		return fmt.Errorf("select environment: %w", err)
	}
	o.envName = name
	return nil
}

func (o *localRunOpts) validateEnvName() error {
	if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
		return fmt.Errorf("get environment %s : %w", o.envName, err)
	}
	return nil
}

func (o *localRunOpts) validateOrAskWorkloadName() error {
	if o.wkldName != "" {
		return o.validateWkldName()
	}

	name, err := o.sel.Workload("Select a workload from your workspace that you want to run locally", "")
	if err != nil {
		return fmt.Errorf("select Workload: %w", err)
	}
	o.wkldName = name
	return nil
}

func (o *localRunOpts) validateWkldName() error {
	if _, err := o.store.GetWorkload(o.appName, o.wkldName); err != nil {
		return fmt.Errorf("get workload name %s : %w", o.wkldName, err)
	}
	return nil
}

// BuildLocalRunCmd builds the command for running a workload locally
func BuildLocalRunCmd() *cobra.Command {
	vars := localRunVars{}
	cmd := &cobra.Command{
		Use:   "local run",
		Short: "Run the workload locally",
		Long:  "Run the workload locally for debugging in a simulated AWS environment",
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newLocalRunOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.wkldName, nameFlag, nameFlagShort, "", workloadFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	return cmd
}

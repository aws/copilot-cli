// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/cobra"
)

const (
	envUpgradeAppPrompt = "In which application is your environment?"

	envUpgradeEnvPrompt = "Which environment do you want to upgrade?"
	envUpgradeEnvHelp   = `Upgrades the AWS CloudFormation template for your environment
to support the latest Copilot features.`
)

// envUpgradeVars holds flag values.
type envUpgradeVars struct {
	appName string // Required. Name of the application.
	name    string // Required. Name of the environment.
	all     bool   // True means all environments should be upgraded.
}

// envUpgradeOpts represents the env upgrade command and holds the necessary data
// and clients to execute the command.
type envUpgradeOpts struct {
	envUpgradeVars

	store environmentStore
	sel   appEnvSelector
}

func newEnvUpgradeOpts(vars envUpgradeVars) (*envUpgradeOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to config store: %v", err)
	}
	return &envUpgradeOpts{
		envUpgradeVars: vars,
		store:          store,
		sel:            selector.NewSelect(prompt.New(), store),
	}, nil
}

// Validate returns an error if the values passed by flags are invalid.
func (o *envUpgradeOpts) Validate() error {
	if o.all && o.name != "" {
		return fmt.Errorf("cannot specify both --%s and --%s flags", allFlag, nameFlag)
	}
	if o.name != "" {
		if _, err := o.store.GetEnvironment(o.appName, o.name); err != nil {
			var errEnvDoesNotExist *config.ErrNoSuchEnvironment
			if errors.As(err, &errEnvDoesNotExist) {
				return err
			}
			return fmt.Errorf("get environment %s configuration from application %s: %v", o.name, o.appName, err)
		}
	}
	return nil
}

// Ask prompts for any required flags that are not set by the user.
func (o *envUpgradeOpts) Ask() error {
	if o.appName == "" {
		app, err := o.sel.Application(envUpgradeAppPrompt, "")
		if err != nil {
			return fmt.Errorf("select application: %v", err)
		}
		o.appName = app
	}
	if !o.all && o.name == "" {
		env, err := o.sel.Environment(envUpgradeEnvPrompt, envUpgradeEnvHelp, o.appName)
		if err != nil {
			return fmt.Errorf("select environment: %v", err)
		}
		o.name = env
	}
	return nil
}

// Execute updates the cloudformation stack an environment to the specified version.
// If the environment stack is busy updating, it spins and waits until the stack can be updated.
func (o *envUpgradeOpts) Execute() error {
	return nil
}

// buildEnvUpgradeCmd builds the command to update the
func buildEnvUpgradeCmd() *cobra.Command {
	vars := envUpgradeVars{}
	cmd := &cobra.Command{
		Use:    "upgrade",
		Short:  "Upgrades the template of an environment to the latest version.",
		Hidden: true,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newEnvUpgradeOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().BoolVar(&vars.all, allFlag, false, upgradeAllEnvsDescription)
	return cmd
}

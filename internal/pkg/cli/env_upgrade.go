// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// envUpgradeVars holds flag values.
type envUpgradeVars struct {
	*GlobalOpts

	name string // Required. Name of the environment.
	all  bool   // True means all environments should be upgraded.
}

// envUpgradeOpts represents the env upgrade command and holds the necessary data
// and clients to execute the command.
type envUpgradeOpts struct {
	envUpgradeVars
}

func newEnvUpgradeOpts(vars envUpgradeVars) *envUpgradeOpts {
	return &envUpgradeOpts{
		envUpgradeVars: vars,
	}
}

// Validate returns an error if the values passed by flags are invalid.
func (o *envUpgradeOpts) Validate() error {
	return nil
}

// Ask prompts for any required flags that are not set by the user.
func (o *envUpgradeVars) Ask() error {
	return nil
}

// Execute updates the cloudformation stack an environment to the specified version.
// If the environment stack is busy updating, it spins and waits until the stack can be updated.
func (o *envUpgradeOpts) Execute() error {
	fmt.Printf("app name is %s\n", o.AppName())
	return nil
}

// BuildEnvUpgradeCmd builds the command to update the
func BuildEnvUpgradeCmd() *cobra.Command {
	vars := envUpgradeVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:    "upgrade",
		Short:  "Upgrades the template of an environment to the latest version.",
		Hidden: true,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts := newEnvUpgradeOpts(vars)
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
	cmd.Flags().BoolVar(&vars.all, allFlag, false, upgradeAllEnvsDescription)
	return cmd
}

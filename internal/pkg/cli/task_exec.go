// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/spf13/cobra"
)

type taskExecVars struct {
	execVars
	cluster string
}

type taskExecOpts struct {
	taskExecVars
	store            store
	ssmPluginManager ssmPluginManager
	prompter         prompter
}

func newTaskExecOpts(vars taskExecVars) (*taskExecOpts, error) {
	ssmStore, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to config store: %w", err)
	}
	return &taskExecOpts{
		taskExecVars:     vars,
		store:            ssmStore,
		ssmPluginManager: exec.NewSSMPluginCommand(nil),
		prompter:         prompt.New(),
	}, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *taskExecOpts) Validate() error {
	if o.cluster != "" && (o.appName != "" || o.envName != "") {
		return fmt.Errorf("cannot specify both cluster flag and app or env flags")
	}
	if o.appName != "" {
		if _, err := o.store.GetApplication(o.appName); err != nil {
			return err
		}
	}
	if o.envName != "" {
		if _, err := o.store.GetEnvironment(o.appName, o.envName); err != nil {
			return err
		}
	}
	return validateSSMBinary(o.prompter, o.ssmPluginManager)
}

// Ask asks for fields that are required but not passed in.
func (o *taskExecOpts) Ask() error {
	return nil
}

// Execute executes a command in a running container.
func (o *taskExecOpts) Execute() error {
	return nil
}

// buildTaskExecCmd builds the command for execute a running container in a one-off task.
func buildTaskExecCmd() *cobra.Command {
	vars := taskExecVars{}
	cmd := &cobra.Command{
		Use:   "exec",
		Short: "Execute a command in a running container part of a task.",
		Example: `
  Start an interactive bash session with a task in task group "db-migrate" in the "test" environment under the current workspace.
  /code $ copilot task exec -e test -n db-migrate
  Runs the 'cat progress.csv' command in the task prefixed with ID "1848c38" part of the "db-migrate" task group.
  /code $ copilot task exec --name db-migrate --task-id 1848c38 --command "cat progress.csv"
  Start an interactive bash session with a task prefixed with ID "38c3818" in the default cluster.
  /code $ copilot task exec --cluster default --task-id 38c3818`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newTaskExecOpts(vars)
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
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", nameFlagDescription)
	cmd.Flags().StringVarP(&vars.command, commandFlag, commandFlagShort, defaultCommand, execCommandFlagDescription)
	cmd.Flags().StringVar(&vars.taskID, taskIDFlag, "", taskIDFlagDescription)
	cmd.Flags().StringVar(&vars.containerName, containerFlag, "", containerFlagDescription)
	cmd.Flags().StringVar(&vars.cluster, clusterFlag, "", clusterFlagDescription)

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Debug,
	}
	return cmd
}

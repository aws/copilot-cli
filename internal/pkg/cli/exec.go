// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

const (
	defaultCommand = "/bin/bash"
)

type execVars struct {
	appName       string
	envName       string
	name          string
	command       string
	taskID        string
	containerName string
	interactive   bool
}

type execOpts struct {
	execVars
}

func newExecOpts(vars execVars) (*execOpts, error) {
	return nil, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *execOpts) Validate() error {
	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *execOpts) Ask() error {
	return nil
}

// Execute executes a command in a running container.
func (o *execOpts) Execute() error {
	return nil
}

// BuildExecCmd is the top level command for exec.
func BuildExecCmd() *cobra.Command {
	vars := execVars{}
	cmd := &cobra.Command{
		Use:   "exec",
		Short: "Execute a command in a running container.",
		Example: `
  Start an interactive bash session with a task part of the "frontend" service.
  /code $ copilot exec -a my-app -e test -n frontend
  Runs the 'cat progress.csv' command in the task prefixed with ID "1848c38" part of the "db-migrate" task group.
  /code $ copilot exec --name db-migrate --task-id 1848c38 --command "cat progress.csv" --interactive=false`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newExecOpts(vars)
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
	cmd.Flags().BoolVar(&vars.interactive, interactiveFlag, true, interactiveFlagDescription)

	cmd.SetUsageTemplate(template.Usage)
	cmd.Annotations = map[string]string{
		"group": group.Debug,
	}
	return cmd
}

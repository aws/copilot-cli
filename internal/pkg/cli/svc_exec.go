// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/copilot-cli/cmd/copilot/template"
	"github.com/aws/copilot-cli/internal/pkg/cli/group"
	"github.com/spf13/cobra"
)

type svcExecOpts struct {
	execVars
}

func newSvcExecOpts(vars execVars) (*svcExecOpts, error) {
	return &svcExecOpts{
		execVars: vars,
	}, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *svcExecOpts) Validate() error {
	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *svcExecOpts) Ask() error {
	return nil
}

// Execute executes a command in a running container.
func (o *svcExecOpts) Execute() error {
	return nil
}

// buildSvcExecCmd builds the command for execute a running container in a service.
func buildSvcExecCmd() *cobra.Command {
	vars := execVars{}
	cmd := &cobra.Command{
		Use:   "exec",
		Short: "Execute a command in a running container part of a service.",
		Example: `
  Start an interactive bash session with a task part of the "frontend" service.
  /code $ copilot svc exec -a my-app -e test -n frontend
  Runs the 'ls' command in the task prefixed with ID "8c38184" within the "backend" service.
  /code $ copilot svc exec -a my-app -e test --name backend --task-id 8c38184 --command "ls" --interactive=false`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newSvcExecOpts(vars)
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

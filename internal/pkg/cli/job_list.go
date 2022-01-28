// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"

	"github.com/aws/copilot-cli/internal/pkg/cli/list"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

const (
	jobListAppNamePrompt = "Which application's jobs would you like to list?"
)

type listJobOpts struct {
	listWkldVars

	// Dependencies
	sel  appSelector
	list workloadListWriter
}

func newListJobOpts(vars listWkldVars) (*listJobOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, err
	}
	ws, err := workspace.New()
	if err != nil {
		return nil, err
	}
	jobLister := &list.JobListWriter{
		Ws:    ws,
		Store: store,
		Out:   os.Stdout,

		ShowLocalJobs: vars.shouldOutputJSON,
		OutputJSON:    vars.shouldOutputJSON,
	}

	return &listJobOpts{
		listWkldVars: vars,

		list: jobLister,
		sel:  selector.NewSelect(prompt.New(), store),
	}, nil
}

func (o *listJobOpts) Ask() error {
	if o.appName != "" {
		return nil
	}

	name, err := o.sel.Application(jobListAppNamePrompt, svcAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application name: %w", err)
	}
	o.appName = name
	return nil
}

// Execute lists the jobs in the workspace or application.
func (o *listJobOpts) Execute() error {
	if err := o.list.Write(o.appName); err != nil {
		return err
	}
	return nil
}

func buildJobListCmd() *cobra.Command {
	vars := listWkldVars{}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lists all the jobs in an application.",
		Example: `
  Lists all the jobs for the "myapp" application.
  /code $ copilot job ls --app myapp`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newListJobOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldShowLocalWorkloads, localFlag, false, localJobFlagDescription)
	return cmd
}

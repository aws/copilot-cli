// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

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
	store store
	ws    wsJobDirReader
	w     io.Writer
	sel   appSelector
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
	return &listJobOpts{
		listWkldVars: vars,

		store: store,
		ws:    ws,
		w:     os.Stdout,
		sel:   selector.NewSelect(prompt.New(), store),
	}, nil
}

func (o *listJobOpts) Ask() error {
	if o.appName != "" {
		return nil
	}

	name, err := o.sel.Application(jobListAppNamePrompt, wkldListAppNameHelp)
	if err != nil {
		return fmt.Errorf("select application name: %w", err)
	}
	o.appName = name
	return nil
}

// Execute lists the jobs in the workspace or application.
func (o *listJobOpts) Execute() error {
	// Ensure the application actually exists before we try to list its services.
	if _, err := o.store.GetApplication(o.appName); err != nil {
		return fmt.Errorf("get application: %w", err)
	}

	jobs, err := o.store.ListJobs(o.appName)
	if err != nil {
		return err
	}

	if o.shouldShowLocalWorkloads {
		localNames, err := o.ws.JobNames()
		if err != nil {
			return fmt.Errorf("get local job names: %w", err)
		}
		jobs = filterSvcsByName(jobs, localNames)
	}

	var out string
	if o.shouldOutputJSON {
		data, err := o.jsonOutput(jobs)
		if err != nil {
			return err
		}
		out = data
		fmt.Fprint(o.w, out)
	} else {
		humanOutput(o.w, jobs)
	}

	return nil
}

func (o *listJobOpts) jsonOutput(jobs []*config.Workload) (string, error) {
	type out struct {
		Jobs []*config.Workload `json:"jobs"`
	}
	b, err := json.Marshal(out{Jobs: jobs})
	if err != nil {
		return "", fmt.Errorf("marshal jobs: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
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

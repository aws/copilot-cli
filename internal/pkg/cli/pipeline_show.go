// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

type showPipelineVars struct {
	*GlobalOpts
	shouldOutputJSON      bool
	shouldOutputResources bool
}

type showPipelineOpts struct {
	showPipelineVars

	// Interfaces to dependencies
	ws    wsPipelineReader
	store storeReader
}

func newShowPipelineOpts(vars showPipelineVars) (*showPipelineOpts, error) {
	ssmStore, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to environment datastore: %w", err)
	}
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}

	opts := &showPipelineOpts{
		showPipelineVars: vars,
		ws:               ws,
		store:            ssmStore,
	}

	return opts, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *showPipelineOpts) Validate() error {
	if o.ProjectName() != "" {
		if _, err := o.store.GetProject(o.ProjectName()); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts for fields that are required but not passed in.
func (o *showPipelineOpts) Ask() error {
	// TODO Placeholder
	return nil
}

// Execute writes the pipeline manifest file.
func (o *showPipelineOpts) Execute() error {
	// TODO Placeholder
	return nil
}

// BuildPipelineShowCmd build the command for deploying a new pipeline or updating an existing pipeline.
func BuildPipelineShowCmd() *cobra.Command {
	vars := showPipelineVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Hidden: true, // TODO remove when ready for production!
		Use:    "show",
		Short:  "Shows info about a deployed pipeline for a project.",
		Long:   "Shows info about a deployed pipeline for a project, including information about each stage.",
		Example: `
  Shows info about the pipeline pipeline-myproject-mycompany-myrepo"
  /code $ ecs-preview pipeline show --project myproject --resources`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newShowPipelineOpts(vars)
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
	cmd.Flags().StringVarP(&vars.projectName, projectFlag, projectFlagShort, "", projectFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputResources, resourcesFlag, false, resourcesFlagDescription)

	return cmd
}

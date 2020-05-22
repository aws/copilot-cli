// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/codepipeline"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe"

	"github.com/spf13/cobra"
)

type pipelineStatusVars struct {
	*GlobalOpts
	shouldOutputJSON bool
	appName          string
	pipelineName     string
}

type pipelineStatusOpts struct {
	pipelineStatusVars

	pipelineSvc         pipelineStateGetter
	statusDescriber     pipelineStatusDescriber
	initStatusDescriber func(opts *pipelineStatusOpts) error
}

func newPipelineStatusOpts(vars pipelineStatusVars) (*pipelineStatusOpts, error) {
	session, err := session.NewProvider().Default()
	if err != nil {
		return nil, fmt.Errorf("session: %w", err)
	}

	opts := &pipelineStatusOpts{
		pipelineStatusVars: vars,
		pipelineSvc:        codepipeline.New(session),
		initStatusDescriber: func(o *pipelineStatusOpts) error {
			d, err := describe.NewPipelineStatus(o.pipelineName)
			if err != nil {
				return fmt.Errorf("creating status describer for pipeline %s in application %s: %w", o.pipelineName, o.appName, err)
			}
			o.statusDescriber = d
			return nil
		},
	}
	return opts, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *pipelineStatusOpts) Validate() error {
	return nil
}

// Ask prompts for fields that are required but not passed in.
func (o *pipelineStatusOpts) Ask() error {
	return nil
}

// Execute displays the status of the pipeline.
func (o *pipelineStatusOpts) Execute() error {
	err := o.initStatusDescriber(o)
	if err != nil {
		return fmt.Errorf("describe status of pipeline: %w", err)
	}
	pipelineStatus, err := o.statusDescriber.Describe()
	if err != nil {
		return fmt.Errorf("describe status of pipeline: %w", err)
	}
	fmt.Print(pipelineStatus)
	return nil
}

// BuildPipelineStatusCmd builds the command for showing the status of a deployed pipeline.
func BuildPipelineStatusCmd() *cobra.Command {
	vars := pipelineStatusVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Hidden: true, // TODO: remove when ready for production!
		Use:    "status",
		Short:  "Shows the status of a pipeline.",
		Long:   "Shows the status of each stage of your pipeline.",

		Example: `
Shows status of the pipeline "pipeline-myapp-myrepo".
/code $ copilot pipeline status -n pipeline-myapp-myrepo`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newPipelineStatusOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Ask(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	// The flags bound by viper are available to all sub-commands through viper.GetString({flagName})
	cmd.Flags().StringVarP(&vars.pipelineName, nameFlag, nameFlagShort, "", pipelineFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, "", appFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)

	return cmd
}

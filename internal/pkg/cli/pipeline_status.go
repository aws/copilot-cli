// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/cli/selector"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/spf13/cobra"
)

const (
	pipelineStatusAppNamePrompt          = "Which application's pipeline status would you like to show?"
	pipelineStatusAppNameHelpPrompt      = "An application is a collection of related services."
	fmtPipelineStatusPipelineNamePrompt  = "Which pipeline of %s would you like to show the status of?"
	pipelineStatusPipelineNameHelpPrompt = "The details of a pipeline's status will be shown (e.g., stages, status, transition)."
)

type pipelineStatusVars struct {
	*GlobalOpts
	shouldOutputJSON bool
	pipelineName     string
}

type pipelineStatusOpts struct {
	pipelineStatusVars

	w             io.Writer
	ws            wsPipelineReader
	store         store
	pipelineSvc   pipelineGetter
	describer     describer
	sel           appSelector
	initDescriber func(opts *pipelineStatusOpts) error
}

func newPipelineStatusOpts(vars pipelineStatusVars) (*pipelineStatusOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store client: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace client: %w", err)
	}

	session, err := session.NewProvider().Default()
	if err != nil {
		return nil, fmt.Errorf("session: %w", err)
	}

	return &pipelineStatusOpts{
		w:                  log.OutputWriter,
		pipelineStatusVars: vars,
		ws:                 ws,
		store:              store,
		pipelineSvc:        codepipeline.New(session),
		sel:                selector.NewSelect(vars.prompt, store),
		initDescriber: func(o *pipelineStatusOpts) error {
			d, err := describe.NewPipelineStatusDescriber(o.pipelineName)
			if err != nil {
				return fmt.Errorf("new pipeline status describer: %w", err)
			}
			o.describer = d
			return nil
		},
	}, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *pipelineStatusOpts) Validate() error {
	if o.AppName() != "" {
		if _, err := o.store.GetApplication(o.AppName()); err != nil {
			return err
		}
	}
	if o.pipelineName != "" {
		if _, err := o.pipelineSvc.GetPipeline(o.pipelineName); err != nil {
			return err
		}
	}
	return nil
}

// Ask prompts for fields that are required but not passed in.
func (o *pipelineStatusOpts) Ask() error {
	if err := o.askAppName(); err != nil {
		return err
	}
	return o.askPipelineName()
}

// Execute displays the status of the pipeline.
func (o *pipelineStatusOpts) Execute() error {
	err := o.initDescriber(o)
	if err != nil {
		return fmt.Errorf("describe status of pipeline: %w", err)
	}
	pipelineStatus, err := o.describer.Describe()
	if err != nil {
		return fmt.Errorf("describe status of pipeline: %w", err)
	}

	if o.shouldOutputJSON {
		data, err := pipelineStatus.JSONString()
		if err != nil {
			return err
		}
		fmt.Fprintf(o.w, data)
	} else {
		fmt.Fprintf(o.w, pipelineStatus.HumanString())
	}

	return nil
}

func (o *pipelineStatusOpts) askAppName() error {
	if o.AppName() != "" {
		return nil
	}
	name, err := o.sel.Application(pipelineStatusAppNamePrompt, pipelineStatusAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = name
	return nil
}

func (o *pipelineStatusOpts) askPipelineName() error {
	// return if pipeline name is set by flag
	if o.pipelineName != "" {
		return nil
	}

	// return pipelineName from manifest if found
	pipelineName, err := o.getPipelineNameFromManifest()
	if err == nil {
		o.pipelineName = pipelineName
		return nil
	}

	if errors.Is(err, workspace.ErrNoPipelineInWorkspace) {
		log.Infof("No pipeline manifest in workspace for application %s, looking for deployed pipelines.\n", color.HighlightUserInput(o.AppName()))
	}

	// find deployed pipelines
	pipelineNames, err := o.retrieveAllPipelines()
	if err != nil {
		return err
	}

	if len(pipelineNames) == 0 {
		log.Infof("No pipelines found for application %s.\n", color.HighlightUserInput(o.AppName()))
		return nil
	}

	if len(pipelineNames) == 1 {
		pipelineName = pipelineNames[0]
		log.Infof("Found pipeline: %s\n", color.HighlightUserInput(pipelineName))
		o.pipelineName = pipelineName

		return nil
	}

	// select from list of deployed pipelines
	pipelineName, err = o.prompt.SelectOne(
		fmt.Sprintf(fmtPipelineStatusPipelineNamePrompt, color.HighlightUserInput(o.AppName())), pipelineStatusPipelineNameHelpPrompt, pipelineNames,
	)
	if err != nil {
		return fmt.Errorf("select pipeline for application %s: %w", o.AppName(), err)
	}
	o.pipelineName = pipelineName
	return nil
}

func (o *pipelineStatusOpts) retrieveAllPipelines() ([]string, error) {
	pipelines, err := o.pipelineSvc.ListPipelineNamesByTags(map[string]string{
		stack.AppTagKey: o.AppName(),
	})
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}
	return pipelines, nil
}

func (o *pipelineStatusOpts) getPipelineNameFromManifest() (string, error) {
	data, err := o.ws.ReadPipelineManifest()
	if err != nil {
		return "", err
	}

	pipeline, err := manifest.UnmarshalPipeline(data)
	if err != nil {
		return "", fmt.Errorf("unmarshal pipeline manifest: %w", err)
	}

	return pipeline.Name, nil
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
			if err := opts.Validate(); err != nil {
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

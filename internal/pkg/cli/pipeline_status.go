// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"

	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
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
	appName          string
	shouldOutputJSON bool
	name             string
}

type pipelineStatusOpts struct {
	pipelineStatusVars

	w             io.Writer
	ws            wsPipelineReader
	store         store
	codepipeline  pipelineGetter
	describer     describer
	sel           appSelector
	prompt        prompter
	initDescriber func(opts *pipelineStatusOpts) error
}

func newPipelineStatusOpts(vars pipelineStatusVars) (*pipelineStatusOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace client: %w", err)
	}

	session, err := sessions.ImmutableProvider(sessions.UserAgentExtras("pipeline status")).Default()
	if err != nil {
		return nil, fmt.Errorf("session: %w", err)
	}

	store := config.NewSSMStore(identity.New(session), ssm.New(session), aws.StringValue(session.Config.Region))
	prompter := prompt.New()
	return &pipelineStatusOpts{
		w:                  log.OutputWriter,
		pipelineStatusVars: vars,
		ws:                 ws,
		store:              store,
		codepipeline:       codepipeline.New(session),
		sel:                selector.NewSelect(prompter, store),
		prompt:             prompter,
		initDescriber: func(o *pipelineStatusOpts) error {
			d, err := describe.NewPipelineStatusDescriber(o.name)
			if err != nil {
				return fmt.Errorf("new pipeline status describer: %w", err)
			}
			o.describer = d
			return nil
		},
	}, nil
}

// Validate returns an error if the optional flag values provided by the user are invalid.
func (o *pipelineStatusOpts) Validate() error {
	if o.appName != "" {
		if _, err := o.store.GetApplication(o.appName); err != nil {
			return fmt.Errorf("validate application name: %w", err)
		}
	}
	return nil
}

// Ask prompts for fields that are required but not passed in, and validates those that are.
func (o *pipelineStatusOpts) Ask() error {
	if o.name != "" {
		_, err := o.codepipeline.GetPipeline(o.name)
		if err != nil {
			return err
		}
		return nil
	}
	// The app name is needed to fetch pipelines.
	if o.appName == "" {
		if err := o.askAppName(); err != nil {
			return err
		}
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
		fmt.Fprint(o.w, data)
	} else {
		fmt.Fprint(o.w, pipelineStatus.HumanString())
	}

	return nil
}

func (o *pipelineStatusOpts) askAppName() error {
	name, err := o.sel.Application(pipelineStatusAppNamePrompt, pipelineStatusAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = name
	return nil
}

func (o *pipelineStatusOpts) askPipelineName() error {
	// find deployed pipelines
	pipelineNames, err := o.retrieveAllPipelines()
	if err != nil {
		return err
	}

	if len(pipelineNames) == 0 {
		return fmt.Errorf("no pipelines found for application %s", color.HighlightUserInput(o.appName))
	}

	if len(pipelineNames) == 1 {
		pipelineName := pipelineNames[0]
		log.Infof("Found pipeline: %s\n", color.HighlightUserInput(pipelineName))
		o.name = pipelineName

		return nil
	}

	// select from list of deployed pipelines
	pipelineName, err := o.prompt.SelectOne(
		fmt.Sprintf(fmtPipelineStatusPipelineNamePrompt, color.HighlightUserInput(o.appName)),
		pipelineStatusPipelineNameHelpPrompt,
		pipelineNames,
		prompt.WithFinalMessage("Pipeline:"),
	)
	if err != nil {
		return fmt.Errorf("select pipeline for application %s: %w", o.appName, err)
	}
	o.name = pipelineName
	return nil
}

func (o *pipelineStatusOpts) retrieveAllPipelines() ([]string, error) {
	pipelines, err := o.codepipeline.ListPipelineNamesByTags(map[string]string{
		deploy.AppTagKey: o.appName,
	})
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}
	return pipelines, nil
}

// buildPipelineStatusCmd builds the command for showing the status of a deployed pipeline.
func buildPipelineStatusCmd() *cobra.Command {
	vars := pipelineStatusVars{}
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Shows the status of a pipeline.",
		Long:  "Shows the status of each stage of your pipeline.",

		Example: `
Shows status of the pipeline "pipeline-myapp-myrepo".
/code $ copilot pipeline status -n pipeline-myapp-myrepo`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newPipelineStatusOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", pipelineFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)

	return cmd
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/spf13/afero"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"

	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/spf13/cobra"
)

const (
	pipelineStatusAppNamePrompt     = "Which application's pipeline status would you like to show?"
	pipelineStatusAppNameHelpPrompt = "An application is a collection of related services."

	fmtpipelineStatusPrompt = "Which pipeline of %s would you like to show the status of?"
)

type pipelineStatusVars struct {
	appName          string
	shouldOutputJSON bool
	name             string
}

type pipelineStatusOpts struct {
	pipelineStatusVars

	w                      io.Writer
	ws                     wsPipelineReader
	store                  store
	codepipeline           pipelineGetter
	describer              describer
	sel                    codePipelineSelector
	prompt                 prompter
	initDescriber          func(opts *pipelineStatusOpts) error
	deployedPipelineLister deployedPipelineLister

	// Cached variables.
	targetPipeline *deploy.Pipeline
}

func newPipelineStatusOpts(vars pipelineStatusVars) (*pipelineStatusOpts, error) {
	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}

	session, err := sessions.ImmutableProvider(sessions.UserAgentExtras("pipeline status")).Default()
	if err != nil {
		return nil, fmt.Errorf("session: %w", err)
	}
	codepipeline := codepipeline.New(session)
	pipelineLister := deploy.NewPipelineStore(rg.New(session))
	store := config.NewSSMStore(identity.New(session), ssm.New(session), aws.StringValue(session.Config.Region))
	prompter := prompt.New()
	return &pipelineStatusOpts{
		w:                      log.OutputWriter,
		pipelineStatusVars:     vars,
		ws:                     ws,
		store:                  store,
		codepipeline:           codepipeline,
		deployedPipelineLister: pipelineLister,
		sel:                    selector.NewAppPipelineSelector(prompter, store, pipelineLister),
		prompt:                 prompter,
		initDescriber: func(o *pipelineStatusOpts) error {
			pipeline, err := o.getTargetPipeline()
			if err != nil {
				return err
			}
			d, err := describe.NewPipelineStatusDescriber(pipeline)
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
	return nil
}

// Ask prompts for fields that are required but not passed in, and validates those that are.
func (o *pipelineStatusOpts) Ask() error {
	if o.appName != "" {
		if _, err := o.store.GetApplication(o.appName); err != nil {
			return fmt.Errorf("validate application name: %w", err)
		}
	} else {
		if err := o.askAppName(); err != nil {
			return err
		}
	}
	if o.name != "" {
		if _, err := o.getTargetPipeline(); err != nil {
			return fmt.Errorf("validate pipeline name %s: %w", o.name, err)
		}
		return nil
	}
	pipeline, err := askDeployedPipelineName(o.sel, fmt.Sprintf(fmtpipelineStatusPrompt, color.HighlightUserInput(o.appName)), o.appName)
	if err != nil {
		return err
	}
	o.name = pipeline.Name
	o.targetPipeline = &pipeline
	return nil
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

func (o *pipelineStatusOpts) getTargetPipeline() (deploy.Pipeline, error) {
	if o.targetPipeline != nil {
		return *o.targetPipeline, nil
	}
	pipeline, err := getDeployedPipelineInfo(o.deployedPipelineLister, o.appName, o.name)
	if err != nil {
		return deploy.Pipeline{}, err
	}
	o.targetPipeline = &pipeline
	return pipeline, nil
}

func (o *pipelineStatusOpts) askAppName() error {
	name, err := o.sel.Application(pipelineStatusAppNamePrompt, pipelineStatusAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = name
	return nil
}

// buildPipelineStatusCmd builds the command for showing the status of a deployed pipeline.
func buildPipelineStatusCmd() *cobra.Command {
	vars := pipelineStatusVars{}
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Shows the status of a pipeline.",
		Long:  "Shows the status of each stage of your pipeline.",

		Example: `
Shows status of the pipeline "my-repo-my-branch".
/code $ copilot pipeline status -n my-repo-my-branch`,
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

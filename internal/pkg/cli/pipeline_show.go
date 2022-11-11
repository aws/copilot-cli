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
	pipelineShowAppNamePrompt     = "Which application's pipelines would you like to show?"
	pipelineShowAppNameHelpPrompt = "An application is a collection of related services."

	fmtPipelineShowPrompt = "Which deployed pipeline of application %s would you like to show the details of?"
)

type showPipelineVars struct {
	appName               string
	name                  string
	shouldOutputJSON      bool
	shouldOutputResources bool
}

type showPipelineOpts struct {
	showPipelineVars

	// Interfaces to dependencies
	w                      io.Writer
	ws                     wsPipelineReader
	store                  applicationStore
	codepipeline           pipelineGetter
	describer              describer
	initDescriber          func(bool) error
	sel                    codePipelineSelector
	deployedPipelineLister deployedPipelineLister
	prompt                 prompter

	// Cached variables.
	targetPipeline *deploy.Pipeline
}

func newShowPipelineOpts(vars showPipelineVars) (*showPipelineOpts, error) {
	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}

	defaultSession, err := sessions.ImmutableProvider(sessions.UserAgentExtras("pipeline show")).Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}
	codepipeline := codepipeline.New(defaultSession)
	pipelineLister := deploy.NewPipelineStore(rg.New(defaultSession))
	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	prompter := prompt.New()
	opts := &showPipelineOpts{
		showPipelineVars:       vars,
		ws:                     ws,
		store:                  store,
		codepipeline:           codepipeline,
		deployedPipelineLister: pipelineLister,
		sel:                    selector.NewAppPipelineSelector(prompter, store, pipelineLister),
		prompt:                 prompter,
		w:                      log.OutputWriter,
	}
	opts.initDescriber = func(enableResources bool) error {
		pipeline, err := opts.getTargetPipeline()
		if err != nil {
			return err
		}
		describer, err := describe.NewPipelineDescriber(pipeline, enableResources)
		if err != nil {
			return fmt.Errorf("new pipeline describer: %w", err)
		}

		opts.describer = describer
		return nil
	}
	return opts, nil
}

// Validate returns an error if the optional flag values passed by the user are invalid.
func (o *showPipelineOpts) Validate() error {
	return nil
}

// Ask prompts for fields that are required but not passed in, and validates those that are.
func (o *showPipelineOpts) Ask() error {
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
	pipeline, err := askDeployedPipelineName(o.sel, fmt.Sprintf(fmtPipelineShowPrompt, color.HighlightUserInput(o.appName)), o.appName)
	if err != nil {
		return err
	}
	o.name = pipeline.Name
	o.targetPipeline = &pipeline
	return nil
}

// Execute shows details about the pipeline.
func (o *showPipelineOpts) Execute() error {
	err := o.initDescriber(o.shouldOutputResources)
	if err != nil {
		return err
	}

	pipeline, err := o.describer.Describe()
	if err != nil {
		return fmt.Errorf("describe pipeline %s: %w", o.name, err)
	}

	if o.shouldOutputJSON {
		data, err := pipeline.JSONString()
		if err != nil {
			return err
		}
		fmt.Fprint(o.w, data)
	} else {
		fmt.Fprint(o.w, pipeline.HumanString())
	}

	return nil
}

func (o *showPipelineOpts) getTargetPipeline() (deploy.Pipeline, error) {
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

func (o *showPipelineOpts) askAppName() error {
	name, err := o.sel.Application(pipelineShowAppNamePrompt, pipelineShowAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = name
	return nil
}

// buildPipelineShowCmd build the command for deploying a new pipeline or updating an existing pipeline.
func buildPipelineShowCmd() *cobra.Command {
	vars := showPipelineVars{}
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Shows info about a deployed pipeline for an application.",
		Long:  "Shows info about a deployed pipeline for an application, including information about each stage.",
		Example: `
  Shows info, including resources, about the pipeline "myrepo-mybranch."
  /code $ copilot pipeline show --name myrepo-mybranch --resources`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newShowPipelineOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, "", pipelineFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputResources, resourcesFlag, false, pipelineResourcesFlagDescription)

	return cmd
}

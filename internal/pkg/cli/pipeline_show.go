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
	pipelineShowAppNamePrompt          = "Which application's pipelines would you like to show?"
	pipelineShowAppNameHelpPrompt      = "An application is a collection of related services."
	fmtPipelineShowPipelineNamePrompt  = "Which pipeline of %s would you like to show?"
	pipelineShowPipelineNameHelpPrompt = "The details of a pipeline will be shown (e.g., region, account ID, stages)."
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
	w             io.Writer
	ws            wsPipelineReader
	store         applicationStore
	codepipeline  pipelineGetter
	describer     describer
	initDescriber func(bool) error
	sel           appSelector
	prompt        prompter
}

func newShowPipelineOpts(vars showPipelineVars) (*showPipelineOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace client: %w", err)
	}

	defaultSession, err := sessions.ImmutableProvider(sessions.UserAgentExtras("pipeline show")).Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}

	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	prompter := prompt.New()
	opts := &showPipelineOpts{
		showPipelineVars: vars,
		ws:               ws,
		store:            store,
		codepipeline:     codepipeline.New(defaultSession),
		sel:              selector.NewSelect(prompter, store),
		prompt:           prompter,
		w:                log.OutputWriter,
	}
	opts.initDescriber = func(enableResources bool) error {
		describer, err := describe.NewPipelineDescriber(opts.name, enableResources)
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
	if o.appName != "" {
		if _, err := o.store.GetApplication(o.appName); err != nil {
			return fmt.Errorf("validate application name: %w", err)
		}
	}
	return nil
}

// Ask prompts for fields that are required but not passed in, and validates those that are.
func (o *showPipelineOpts) Ask() error {
	if o.name != "" {
		if _, err := o.codepipeline.GetPipeline(o.name); err != nil {
			return err
		}
	} else {
		// Validate presence of appName in order to fetch pipelines.
		if o.appName == "" {
			if err := o.askAppName(); err != nil {
				return err
			}
		}
		if err := o.askPipelineName(); err != nil {
			return err
		}
	}
	return nil
}

func (o *showPipelineOpts) askAppName() error {
	name, err := o.sel.Application(pipelineShowAppNamePrompt, pipelineShowAppNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = name
	return nil
}

func (o *showPipelineOpts) askPipelineName() error {
	// find deployed pipelines
	pipelineNames, err := o.retrieveAllPipelines()
	if err != nil {
		return err
	}

	if len(pipelineNames) == 0 {
		log.Infof("No deployed pipelines found for application %s.\n", color.HighlightUserInput(o.appName))
		return nil
	}

	if len(pipelineNames) == 1 {
		pipelineName := pipelineNames[0]
		log.Infof("Found pipeline: %s.\n", color.HighlightUserInput(pipelineName))
		o.name = pipelineName

		return nil
	}

	// select from list of deployed pipelines
	pipelineName, err := o.prompt.SelectOne(
		fmt.Sprintf(fmtPipelineShowPipelineNamePrompt, color.HighlightUserInput(o.appName)),
		pipelineShowPipelineNameHelpPrompt,
		pipelineNames,
		prompt.WithFinalMessage("Pipeline:"),
	)
	if err != nil {
		return fmt.Errorf("select pipeline for application %s: %w", o.appName, err)
	}
	o.name = pipelineName
	return nil

}

func (o *showPipelineOpts) retrieveAllPipelines() ([]string, error) {
	pipelines, err := o.codepipeline.ListPipelineNamesByTags(map[string]string{
		deploy.AppTagKey: o.appName,
	})
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}
	return pipelines, nil
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

// buildPipelineShowCmd build the command for deploying a new pipeline or updating an existing pipeline.
func buildPipelineShowCmd() *cobra.Command {
	vars := showPipelineVars{}
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Shows info about a deployed pipeline for an application.",
		Long:  "Shows info about a deployed pipeline for an application, including information about each stage.",
		Example: `
  Shows info about the pipeline "pipeline-myapp-mycompany-myrepo".
  /code $ copilot pipeline show --app myapp --resources`,
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

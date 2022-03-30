// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/cobra"
)

const (
	pipelineListAppNamePrompt = "Which application are the pipelines in?"
	pipelineListAppNameHelper = "An application is a collection of related services."
)

type listPipelineVars struct {
	appName                  string
	shouldOutputJSON         bool
	shouldShowLocalPipelines bool
}

type listPipelineOpts struct {
	listPipelineVars
	pipelineSvc pipelineGetter
	prompt      prompter
	sel         configSelector
	store       store
	w           io.Writer
	workspace   wsPipelineGetter
	wsAppName   string
}

func newListPipelinesOpts(vars listPipelineVars) (*listPipelineOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, err
	}

	defaultSession, err := sessions.ImmutableProvider(sessions.UserAgentExtras("pipeline ls")).Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}

	wsAppName := tryReadingAppName()
	if vars.appName == "" {
		vars.appName = wsAppName
	}

	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	prompter := prompt.New()
	return &listPipelineOpts{
		listPipelineVars: vars,
		pipelineSvc:      codepipeline.New(defaultSession),
		prompt:           prompter,
		sel:              selector.NewConfigSelect(prompter, store),
		store:            store,
		w:                os.Stdout,
		workspace:        ws,
		wsAppName:        wsAppName,
	}, nil
}

// Ask asks for and validates fields that are required but not passed in.
func (o *listPipelineOpts) Ask() error {
	if o.shouldShowLocalPipelines {
		if err := validateInputApp(o.wsAppName, o.appName, o.store); err != nil {
			return err
		}

		return nil
	}

	if o.appName != "" {
		if _, err := o.store.GetApplication(o.appName); err != nil {
			return fmt.Errorf("validate application: %w", err)
		}
	} else {
		app, err := o.sel.Application(pipelineListAppNamePrompt, pipelineListAppNameHelper)
		if err != nil {
			return fmt.Errorf("select application: %w", err)
		}
		o.appName = app
	}

	return nil
}

// Execute writes the pipelines.
func (o *listPipelineOpts) Execute() error {
	switch {
	case o.shouldShowLocalPipelines && o.shouldOutputJSON:
		return o.jsonOutputLocal()
	case o.shouldShowLocalPipelines:
		return o.humanOutputLocal()
	case o.shouldOutputJSON:
		return o.jsonOutput()
	}

	return o.humanOutput()
}

func (o *listPipelineOpts) jsonOutputLocal() error {
	local, err := o.workspace.ListPipelines()
	if err != nil {
		return err
	}

	deployed, err := o.pipelineSvc.GetPipelinesByTags(map[string]string{
		deploy.AppTagKey: o.appName,
	})
	if err != nil {
		return fmt.Errorf("list pipelines: %w", err)
	}

	cp := make(map[string]*codepipeline.Pipeline)
	for _, pipeline := range deployed {
		cp[pipeline.Name] = pipeline
	}

	type combinedInfo struct {
		Name         string `json:"name"`
		ManifestPath string `json:"manfiestPath"`
		*codepipeline.Pipeline
	}

	var out struct {
		Pipelines []combinedInfo `json:"pipelines"`
	}
	for _, pipeline := range local {
		out.Pipelines = append(out.Pipelines, combinedInfo{
			Name:         pipeline.Name,
			ManifestPath: pipeline.Path,
			Pipeline:     cp[pipeline.Name],
		})
	}

	b, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshal pipelines: %w", err)
	}

	fmt.Fprintf(o.w, "%s\n", b)
	return nil
}

func (o *listPipelineOpts) humanOutputLocal() error {
	local, err := o.workspace.ListPipelines()
	if err != nil {
		return err
	}

	for _, pipeline := range local {
		fmt.Fprintln(o.w, pipeline.Name)
	}

	return nil
}

func (o *listPipelineOpts) jsonOutput() error {
	pipelines, err := o.pipelineSvc.GetPipelinesByTags(map[string]string{
		deploy.AppTagKey: o.appName,
	})
	if err != nil {
		return fmt.Errorf("list pipelines: %w", err)
	}

	type serializedPipelines struct {
		Pipelines []*codepipeline.Pipeline `json:"pipelines"`
	}
	b, err := json.Marshal(serializedPipelines{Pipelines: pipelines})
	if err != nil {
		return fmt.Errorf("marshal pipelines: %w", err)
	}

	fmt.Fprintf(o.w, "%s\n", b)
	return nil
}

func (o *listPipelineOpts) humanOutput() error {
	pipelines, err := o.pipelineSvc.GetPipelinesByTags(map[string]string{
		deploy.AppTagKey: o.appName,
	})
	if err != nil {
		return fmt.Errorf("list pipelines: %w", err)
	}

	for _, pipeline := range pipelines {
		fmt.Fprintln(o.w, pipeline.Name)
	}

	return nil
}

// buildPipelineListCmd builds the command for showing a list of all deployed pipelines.
func buildPipelineListCmd() *cobra.Command {
	vars := listPipelineVars{}
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "Lists all the deployed pipelines in an application.",
		Example: `
  Lists all the pipelines for the frontend application.
  /code $ copilot pipeline ls -a frontend`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newListPipelinesOpts(vars)
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
	cmd.Flags().BoolVar(&vars.shouldShowLocalPipelines, localFlag, false, localPipelineFlagDescription)
	return cmd
}

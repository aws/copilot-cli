// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"

	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/spf13/cobra"
)

const (
	pipelineListAppNamePrompt = "Which application are the pipelines in?"
	pipelineListAppNameHelper = "An application is a collection of related services."
)

type listPipelineVars struct {
	appName          string
	shouldOutputJSON bool
}

type listPipelineOpts struct {
	listPipelineVars
	pipelineSvc pipelineGetter
	prompt      prompter
	sel         configSelector
	w           io.Writer
}

func newListPipelinesOpts(vars listPipelineVars) (*listPipelineOpts, error) {
	defaultSession, err := sessions.ImmutableProvider(sessions.UserAgentExtras("pipeline ls")).Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}
	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	prompter := prompt.New()
	return &listPipelineOpts{
		listPipelineVars: vars,
		pipelineSvc:      codepipeline.New(defaultSession),
		prompt:           prompter,
		sel:              selector.NewConfigSelect(prompter, store),
		w:                os.Stdout,
	}, nil
}

// Ask asks for fields that are required but not passed in.
func (o *listPipelineOpts) Ask() error {
	if o.appName != "" {
		return nil
	}
	app, err := o.sel.Application(pipelineListAppNamePrompt, pipelineListAppNameHelper)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.appName = app
	return nil
}

// Execute writes the pipelines.
func (o *listPipelineOpts) Execute() error {
	var out string
	if o.shouldOutputJSON {
		pipelines, err := o.pipelineSvc.GetPipelinesByTags(map[string]string{
			deploy.AppTagKey: o.appName,
		})
		if err != nil {
			return fmt.Errorf("list pipelines: %w", err)
		}

		data, err := o.jsonOutput(pipelines)
		if err != nil {
			return err
		}
		out = data
	} else {
		pipelines, err := o.pipelineSvc.ListPipelineNamesByTags(map[string]string{
			deploy.AppTagKey: o.appName,
		})
		if err != nil {
			return fmt.Errorf("list pipelines: %w", err)
		}
		out = o.humanOutput(pipelines)
	}
	fmt.Fprint(o.w, out)

	return nil
}

func (o *listPipelineOpts) jsonOutput(pipelines []*codepipeline.Pipeline) (string, error) {
	type serializedPipelines struct {
		Pipelines []*codepipeline.Pipeline `json:"pipelines"`
	}
	b, err := json.Marshal(serializedPipelines{Pipelines: pipelines})
	if err != nil {
		return "", fmt.Errorf("marshal pipelines: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

func (o *listPipelineOpts) humanOutput(pipelines []string) string {
	b := &strings.Builder{}
	for _, pipeline := range pipelines {
		fmt.Fprintln(b, pipeline)
	}
	return b.String()
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
	return cmd
}

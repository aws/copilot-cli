// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"golang.org/x/sync/errgroup"

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

	pipelineListTimeout = 10 * time.Second
)

type listPipelineVars struct {
	appName                  string
	shouldOutputJSON         bool
	shouldShowLocalPipelines bool
}

type listPipelineOpts struct {
	listPipelineVars
	codepipeline   pipelineGetter
	prompt         prompter
	sel            configSelector
	store          store
	w              io.Writer
	workspace      wsPipelineGetter
	wsAppName      string
	pipelineLister deployedPipelineLister
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
		codepipeline:     codepipeline.New(defaultSession),
		pipelineLister:   deploy.NewPipelineStore(vars.appName, rg.New(defaultSession)),
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
	ctx, cancel := context.WithTimeout(context.Background(), pipelineListTimeout)
	defer cancel()

	switch {
	case o.shouldShowLocalPipelines && o.shouldOutputJSON:
		return o.jsonOutputLocal(ctx)
	case o.shouldShowLocalPipelines:
		return o.humanOutputLocal()
	case o.shouldOutputJSON:
		return o.jsonOutputDeployed(ctx)
	}

	return o.humanOutputDeployed(ctx)
}

// jsonOutputLocal prints data about all pipelines in the current workspace.
// If a local pipeline has been deployed, data from codepipeline is included.
func (o *listPipelineOpts) jsonOutputLocal(ctx context.Context) error {
	local, err := o.workspace.ListPipelines()
	if err != nil {
		return err
	}

	deployed, err := getDeployedPipelines(ctx, o.wsAppName, o.pipelineLister, o.codepipeline)
	if err != nil {
		return err
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

// humanOutputLocal prints the name of all pipelines in the current workspace.
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

// jsonOutputDeployed prints data about all pipelines in the given app that have been deployed.
func (o *listPipelineOpts) jsonOutputDeployed(ctx context.Context) error {
	pipelines, err := getDeployedPipelines(ctx, o.appName, o.pipelineLister, o.codepipeline)
	if err != nil {
		return err
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

// humanOutputDeployed prints the name of all pipelines in the given app that have been deployed.
func (o *listPipelineOpts) humanOutputDeployed(ctx context.Context) error {
	pipelines, err := getDeployedPipelines(ctx, o.appName, o.pipelineLister, o.codepipeline)
	if err != nil {
		return err
	}

	for _, pipeline := range pipelines {
		fmt.Fprintln(o.w, pipeline.Name)
	}

	return nil
}

func getDeployedPipelines(ctx context.Context, app string, lister deployedPipelineLister, getter pipelineGetter) ([]*codepipeline.Pipeline, error) {
	names, err := lister.ListDeployedPipelines()
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}

	var mux sync.Mutex
	var res []*codepipeline.Pipeline

	g, _ := errgroup.WithContext(ctx)

	for i := range names {
		if names[i].AppName != app {
			continue
		}

		name := names[i].Name()
		g.Go(func() error {
			pipeline, err := getter.GetPipeline(name)
			if err != nil {
				return fmt.Errorf("get pipeline %q: %w", name, err)
			}

			mux.Lock()
			defer mux.Unlock()
			res = append(res, pipeline)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	sort.Slice(res, func(i, j int) bool {
		return res[i].Name < res[j].Name
	})

	return res, nil
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

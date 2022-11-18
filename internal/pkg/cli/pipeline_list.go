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
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"

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
	prompt         prompter
	sel            configSelector
	store          store
	w              io.Writer
	workspace      wsPipelineGetter
	pipelineLister deployedPipelineLister

	newDescriber newPipelineDescriberFunc

	wsAppName string
}

type newPipelineDescriberFunc func(pipeline deploy.Pipeline) (describer, error)

func newListPipelinesOpts(vars listPipelineVars) (*listPipelineOpts, error) {
	ws, err := workspace.Use(afero.NewOsFs())
	if err != nil {
		return nil, err
	}

	defaultSession, err := sessions.ImmutableProvider(sessions.UserAgentExtras("pipeline ls")).Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}

	var wsAppName string
	if vars.shouldShowLocalPipelines {
		wsAppName = tryReadingAppName()
	}

	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	prompter := prompt.New()
	return &listPipelineOpts{
		listPipelineVars: vars,
		pipelineLister:   deploy.NewPipelineStore(rg.New(defaultSession)),
		prompt:           prompter,
		sel:              selector.NewConfigSelector(prompter, store),
		store:            store,
		w:                os.Stdout,
		workspace:        ws,
		newDescriber: func(pipeline deploy.Pipeline) (describer, error) {
			return describe.NewPipelineDescriber(pipeline, false)
		},
		wsAppName: wsAppName,
	}, nil
}

// Ask asks for and validates fields that are required but not passed in.
func (o *listPipelineOpts) Ask() error {
	if o.shouldShowLocalPipelines {
		return validateWorkspaceApp(o.wsAppName, o.appName, o.store)
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

	return o.humanOutputDeployed()
}

// jsonOutputLocal prints data about all pipelines in the current workspace.
// If a local pipeline has been deployed, data from codepipeline is included.
func (o *listPipelineOpts) jsonOutputLocal(ctx context.Context) error {
	local, err := o.workspace.ListPipelines()
	if err != nil {
		return err
	}

	deployed, err := getDeployedPipelines(ctx, o.appName, o.pipelineLister, o.newDescriber)
	if err != nil {
		return err
	}

	cp := make(map[string]*describe.Pipeline)
	for _, pipeline := range deployed {
		cp[pipeline.Name] = pipeline
	}

	type info struct {
		Name         string `json:"name"`
		ManifestPath string `json:"manifestPath"`
		PipelineName string `json:"pipelineName,omitempty"`
	}

	var out struct {
		Pipelines []info `json:"pipelines"`
	}
	for _, pipeline := range local {
		i := info{
			Name:         pipeline.Name,
			ManifestPath: pipeline.Path,
		}

		if v, ok := cp[pipeline.Name]; ok {
			i.PipelineName = v.Pipeline.Name
		}

		out.Pipelines = append(out.Pipelines, i)
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
	pipelines, err := getDeployedPipelines(ctx, o.appName, o.pipelineLister, o.newDescriber)
	if err != nil {
		return err
	}

	type serializedPipelines struct {
		Pipelines []*describe.Pipeline `json:"pipelines"`
	}
	b, err := json.Marshal(serializedPipelines{Pipelines: pipelines})
	if err != nil {
		return fmt.Errorf("marshal pipelines: %w", err)
	}

	fmt.Fprintf(o.w, "%s\n", b)
	return nil
}

// humanOutputDeployed prints the name of all pipelines in the given app that have been deployed.
func (o *listPipelineOpts) humanOutputDeployed() error {
	pipelines, err := o.pipelineLister.ListDeployedPipelines(o.appName)
	if err != nil {
		return fmt.Errorf("list deployed pipelines: %w", err)
	}

	sort.Slice(pipelines, func(i, j int) bool {
		return pipelines[i].Name < pipelines[j].Name
	})

	for _, p := range pipelines {
		fmt.Fprintln(o.w, p.Name)
	}

	return nil
}

func getDeployedPipelines(ctx context.Context, app string, lister deployedPipelineLister, newDescriber newPipelineDescriberFunc) ([]*describe.Pipeline, error) {
	pipelines, err := lister.ListDeployedPipelines(app)
	if err != nil {
		return nil, fmt.Errorf("list deployed pipelines: %w", err)
	}

	var mux sync.Mutex
	var res []*describe.Pipeline

	g, _ := errgroup.WithContext(ctx)

	for i := range pipelines {
		pipeline := pipelines[i]
		g.Go(func() error {
			d, err := newDescriber(pipeline)
			if err != nil {
				return fmt.Errorf("create pipeline describer for %q: %w", pipeline.ResourceName, err)
			}

			info, err := d.Describe()
			if err != nil {
				return fmt.Errorf("describe pipeline %q: %w", pipeline.ResourceName, err)
			}

			p, ok := info.(*describe.Pipeline)
			if !ok {
				return fmt.Errorf("unexpected describer for %q: %T", pipeline.ResourceName, info)
			}

			mux.Lock()
			defer mux.Unlock()
			res = append(res, p)
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

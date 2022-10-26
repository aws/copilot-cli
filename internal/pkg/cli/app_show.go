// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"

	"io"
	"sort"
	"sync"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"golang.org/x/sync/errgroup"

	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/spf13/cobra"
)

const (
	appShowNamePrompt     = "Which application would you like to show?"
	appShowNameHelpPrompt = "An application is a collection of related services."
	waitForStackTimeout   = 30 * time.Second
)

type showAppVars struct {
	name             string
	shouldOutputJSON bool
}

type showAppOpts struct {
	showAppVars

	store            store
	w                io.Writer
	sel              appSelector
	deployStore      deployedEnvironmentLister
	codepipeline     pipelineGetter
	pipelineLister   deployedPipelineLister
	newVersionGetter func(string) (versionGetter, error)
}

func newShowAppOpts(vars showAppVars) (*showAppOpts, error) {
	sessProvider := sessions.ImmutableProvider(sessions.UserAgentExtras("app show"))
	defaultSession, err := sessProvider.Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}
	store := config.NewSSMStore(identity.New(defaultSession), ssm.New(defaultSession), aws.StringValue(defaultSession.Config.Region))
	deployStore, err := deploy.NewStore(sessProvider, store)
	if err != nil {
		return nil, fmt.Errorf("connect to deploy store: %w", err)
	}
	return &showAppOpts{
		showAppVars:    vars,
		store:          store,
		w:              log.OutputWriter,
		sel:            selector.NewAppEnvSelector(prompt.New(), store),
		deployStore:    deployStore,
		codepipeline:   codepipeline.New(defaultSession),
		pipelineLister: deploy.NewPipelineStore(rg.New(defaultSession)),
		newVersionGetter: func(s string) (versionGetter, error) {
			d, err := describe.NewAppDescriber(s)
			if err != nil {
				return d, fmt.Errorf("new app describer for application %s: %v", s, err)
			}
			return d, nil
		},
	}, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *showAppOpts) Validate() error {
	if o.name != "" {
		_, err := o.store.GetApplication(o.name)
		if err != nil {
			return fmt.Errorf("get application %s: %w", o.name, err)
		}
	}

	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *showAppOpts) Ask() error {
	if err := o.askName(); err != nil {
		return err
	}

	return nil
}

// Execute writes the application's description.
func (o *showAppOpts) Execute() error {
	description, err := o.description()
	if err != nil {
		return err
	}
	if !o.shouldOutputJSON {
		fmt.Fprint(o.w, description.HumanString())
		return nil
	}
	data, err := description.JSONString()
	if err != nil {
		return fmt.Errorf("get JSON string: %w", err)
	}
	fmt.Fprint(o.w, data)
	return nil
}
func (o *showAppOpts) populateDeployedWorkloads(listWorkloads func(app, env string) ([]string, error), deployedEnvsFor map[string][]string, env string, lock sync.Locker) error {
	deployedworkload, err := listWorkloads(o.name, env)
	if err != nil {
		return fmt.Errorf("list services/jobs deployed to %s: %w", env, err)
	}

	lock.Lock()
	defer lock.Unlock()
	for _, wkld := range deployedworkload {
		deployedEnvsFor[wkld] = append(deployedEnvsFor[wkld], env)
	}
	return nil
}

func (o *showAppOpts) description() (*describe.App, error) {
	app, err := o.store.GetApplication(o.name)
	if err != nil {
		return nil, fmt.Errorf("get application %s: %w", o.name, err)
	}
	envs, err := o.store.ListEnvironments(o.name)
	if err != nil {
		return nil, fmt.Errorf("list environments in application %s: %w", o.name, err)
	}
	svcs, err := o.store.ListServices(o.name)
	if err != nil {
		return nil, fmt.Errorf("list services in application %s: %w", o.name, err)
	}
	jobs, err := o.store.ListJobs(o.name)
	if err != nil {
		return nil, fmt.Errorf("list jobs in application %s: %w", o.name, err)
	}
	wkldDeployedtoEnvs := make(map[string][]string)
	ctx, cancelWait := context.WithTimeout(context.Background(), waitForStackTimeout)
	defer cancelWait()
	g, _ := errgroup.WithContext(ctx)
	var mux sync.Mutex
	for i := range envs {
		env := envs[i]
		g.Go(func() error {
			return o.populateDeployedWorkloads(o.deployStore.ListDeployedJobs, wkldDeployedtoEnvs, env.Name, &mux)
		})
		g.Go(func() error {
			return o.populateDeployedWorkloads(o.deployStore.ListDeployedServices, wkldDeployedtoEnvs, env.Name, &mux)
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	// Sort the map values so that `output` is consistent and the unit test won't be flaky.
	for k := range wkldDeployedtoEnvs {
		sort.Strings(wkldDeployedtoEnvs[k])
	}

	pipelines, err := o.pipelineLister.ListDeployedPipelines(o.name)
	if err != nil {
		return nil, fmt.Errorf("list pipelines in application %s: %w", o.name, err)
	}
	var pipelineInfo []*codepipeline.Pipeline
	for _, pipeline := range pipelines {
		info, err := o.codepipeline.GetPipeline(pipeline.ResourceName)
		if err != nil {
			return nil, fmt.Errorf("get info for pipeline %s: %w", pipeline.Name, err)
		}
		pipelineInfo = append(pipelineInfo, info)
	}

	var trimmedEnvs []*config.Environment
	for _, env := range envs {
		trimmedEnvs = append(trimmedEnvs, &config.Environment{
			Name:      env.Name,
			AccountID: env.AccountID,
			Region:    env.Region,
		})
	}
	var trimmedSvcs []*config.Workload
	for _, svc := range svcs {
		trimmedSvcs = append(trimmedSvcs, &config.Workload{
			Name: svc.Name,
			Type: svc.Type,
		})
	}
	var trimmedJobs []*config.Workload
	for _, job := range jobs {
		trimmedJobs = append(trimmedJobs, &config.Workload{
			Name: job.Name,
			Type: job.Type,
		})
	}
	versionGetter, err := o.newVersionGetter(o.name)
	if err != nil {
		return nil, err
	}
	version, err := versionGetter.Version()
	if err != nil {
		return nil, fmt.Errorf("get version for application %s: %w", o.name, err)
	}
	return &describe.App{
		Name:                app.Name,
		Version:             version,
		URI:                 app.Domain,
		PermissionsBoundary: app.PermissionsBoundary,
		Envs:                trimmedEnvs,
		Services:            trimmedSvcs,
		Jobs:                trimmedJobs,
		Pipelines:           pipelineInfo,
		WkldDeployedtoEnvs:  wkldDeployedtoEnvs,
	}, nil
}

func (o *showAppOpts) askName() error {
	if o.name != "" {
		return nil
	}
	name, err := o.sel.Application(appShowNamePrompt, appShowNameHelpPrompt)
	if err != nil {
		return fmt.Errorf("select application: %w", err)
	}
	o.name = name
	return nil
}

// buildAppShowCmd builds the command for showing details of an application.
func buildAppShowCmd() *cobra.Command {
	vars := showAppVars{}
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Shows info about an application.",
		Long:  "Shows configuration, environments and services for an application.",
		Example: `
  Shows info about the application "my-app"
  /code $ copilot app show -n my-app`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newShowAppOpts(vars)
			if err != nil {
				return err
			}
			return run(opts)
		}),
	}
	// The flags bound by viper are available to all sub-commands through viper.GetString({flagName})
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().StringVarP(&vars.name, nameFlag, nameFlagShort, tryReadingAppName(), appFlagDescription)
	return cmd
}

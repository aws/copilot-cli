// Copyright 2020 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

const (
	applicationShowProjectNamePrompt     = "Which project's applications would you like to show?"
	applicationShowProjectNameHelpPrompt = "A project groups all of your applications together."
	applicationShowAppNamePrompt         = "Which application of %s would you like to show?"
	applicationShowAppNameHelpPrompt     = "The detail of an application will be shown (e.g., endpoint URL, CPU, Memory)."
)

type showAppVars struct {
	*GlobalOpts
	shouldOutputJSON      bool
	shouldOutputResources bool
	appName               string
}

type showAppOpts struct {
	showAppVars

	w             io.Writer
	storeSvc      storeReader
	describer     webAppDescriber
	ws            wsAppReader
	initDescriber func(*showAppOpts) error // Overriden in tests.
}

func newShowAppOpts(vars showAppVars) (*showAppOpts, error) {
	ssmStore, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to environment datastore: %w", err)
	}
	ws, err := workspace.New()
	if err != nil {
		return nil, err
	}

	return &showAppOpts{
		showAppVars: vars,
		storeSvc:    ssmStore,
		ws:          ws,
		w:           log.OutputWriter,
		initDescriber: func(o *showAppOpts) error {
			d, err := describe.NewWebAppDescriber(o.ProjectName(), o.appName)
			if err != nil {
				return fmt.Errorf("creating describer for application %s in project %s: %w", o.appName, o.ProjectName(), err)
			}
			o.describer = d
			return nil
		},
	}, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *showAppOpts) Validate() error {
	if o.ProjectName() != "" {
		if _, err := o.storeSvc.GetProject(o.ProjectName()); err != nil {
			return err
		}
	}
	if o.appName != "" {
		if _, err := o.storeSvc.GetApplication(o.ProjectName(), o.appName); err != nil {
			return err
		}
	}

	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *showAppOpts) Ask() error {
	if err := o.askProject(); err != nil {
		return err
	}
	return o.askAppName()
}

// Execute shows the applications through the prompt.
func (o *showAppOpts) Execute() error {
	if o.appName == "" {
		// If there are no local applications in the workspace, we exit without error.
		return nil
	}

	if err := o.initDescriber(o); err != nil {
		return err
	}
	app, err := o.retrieveData()
	if err != nil {
		return err
	}

	if o.shouldOutputJSON {
		data, err := app.JSONString()
		if err != nil {
			return err
		}
		fmt.Fprintf(o.w, data)
	} else {
		fmt.Fprintf(o.w, app.HumanString())
	}

	return nil
}

func (o *showAppOpts) retrieveData() (*describe.WebApp, error) {
	app, err := o.storeSvc.GetApplication(o.ProjectName(), o.appName)
	if err != nil {
		return nil, fmt.Errorf("get application: %w", err)
	}

	environments, err := o.storeSvc.ListEnvironments(o.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}

	var routes []*describe.WebAppRoute
	var configs []*describe.WebAppConfig
	var envVars []*describe.WebAppEnvVars
	for _, env := range environments {
		webAppURI, err := o.describer.URI(env.Name)
		if err == nil {
			routes = append(routes, &describe.WebAppRoute{
				Environment: env.Name,
				URL:         fmt.Sprintf("%s/%s", webAppURI.DNSName, webAppURI.Path),
			})

			webAppECSParams, err := o.describer.ECSParams(env.Name)
			if err != nil {
				return nil, fmt.Errorf("retrieve application deployment configuration: %w", err)
			}
			configs = append(configs, &describe.WebAppConfig{
				Environment: env.Name,
				Port:        webAppECSParams.ContainerPort,
				Tasks:       webAppECSParams.TaskCount,
				CPU:         webAppECSParams.CPU,
				Memory:      webAppECSParams.Memory,
			})

			webAppEnvVars, err := o.describer.EnvVars(env)
			if err != nil {
				return nil, fmt.Errorf("retrieve environment variables: %w", err)
			}
			envVars = append(envVars, webAppEnvVars...)

			continue
		}
		if !isStackNotExistsErr(err) {
			return nil, fmt.Errorf("retrieve application URI: %w", err)
		}
	}
	sort.SliceStable(envVars, func(i, j int) bool { return envVars[i].Environment < envVars[j].Environment })
	sort.SliceStable(envVars, func(i, j int) bool { return envVars[i].Name < envVars[j].Name })

	resources := make(map[string][]*describe.CfnResource)
	if o.shouldOutputResources {
		for _, env := range environments {
			webAppResources, err := o.describer.StackResources(env.Name)
			if err == nil {
				resources[env.Name] = webAppResources
				continue
			}
			if !isStackNotExistsErr(err) {
				return nil, fmt.Errorf("retrieve application resources: %w", err)
			}
		}
	}

	return &describe.WebApp{
		AppName:        app.Name,
		Type:           app.Type,
		Project:        o.ProjectName(),
		Configurations: configs,
		Routes:         routes,
		Variables:      envVars,
		Resources:      resources,
	}, nil
}

func (o *showAppOpts) askProject() error {
	if o.ProjectName() != "" {
		return nil
	}
	projNames, err := o.retrieveProjects()
	if err != nil {
		return err
	}
	if len(projNames) == 0 {
		return fmt.Errorf("no project found: run %s please", color.HighlightCode("project init"))
	}
	proj, err := o.prompt.SelectOne(
		applicationShowProjectNamePrompt,
		applicationShowProjectNameHelpPrompt,
		projNames,
	)
	if err != nil {
		return fmt.Errorf("select projects: %w", err)
	}
	o.projectName = proj

	return nil
}

func (o *showAppOpts) askAppName() error {
	// return if app name is set by flag
	if o.appName != "" {
		return nil
	}

	appNames, err := o.retrieveLocalApplication()
	if err != nil {
		appNames, err = o.retrieveAllApplications()
		if err != nil {
			return err
		}
	}

	if len(appNames) == 0 {
		log.Infof("No applications found in project %s\n.", color.HighlightUserInput(o.ProjectName()))
		return nil
	}
	if len(appNames) == 1 {
		o.appName = appNames[0]
		return nil
	}
	appName, err := o.prompt.SelectOne(
		fmt.Sprintf(applicationShowAppNamePrompt, color.HighlightUserInput(o.ProjectName())),
		applicationShowAppNameHelpPrompt,
		appNames,
	)
	if err != nil {
		return fmt.Errorf("select applications for project %s: %w", o.ProjectName(), err)
	}
	o.appName = appName

	return nil
}

func (o *showAppOpts) retrieveProjects() ([]string, error) {
	projs, err := o.storeSvc.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	projNames := make([]string, len(projs))
	for ind, proj := range projs {
		projNames[ind] = proj.Name
	}
	return projNames, nil
}

func (o *showAppOpts) retrieveLocalApplication() ([]string, error) {
	localAppNames, err := o.ws.AppNames()
	if err != nil {
		return nil, err
	}
	if len(localAppNames) == 0 {
		return nil, errors.New("no application found")
	}
	return localAppNames, nil
}

func (o *showAppOpts) retrieveAllApplications() ([]string, error) {
	apps, err := o.storeSvc.ListApplications(o.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("list applications for project %s: %w", o.ProjectName(), err)
	}
	appNames := make([]string, len(apps))
	for ind, app := range apps {
		appNames[ind] = app.Name
	}

	return appNames, nil
}

// BuildAppShowCmd builds the command for showing applications in a project.
func BuildAppShowCmd() *cobra.Command {
	vars := showAppVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Shows info about a deployed application per environment.",
		Long:  "Shows info about a deployed application, including endpoints, capacity and related resources per environment.",

		Example: `
  Shows info about the application "my-app"
  /code $ ecs-preview app show -a my-app`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newShowAppOpts(vars)
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
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, "", appFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputResources, resourcesFlag, false, resourcesFlagDescription)
	cmd.Flags().StringVarP(&vars.projectName, projectFlag, projectFlagShort, "", projectFlagDescription)
	return cmd
}

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	environmentShowProjectNamePrompt     = "Which project's environment would you like to show?"
	environmentShowProjectNameHelpPrompt = "A project groups all of your applications together."
	environmentShowEnvNamePrompt         = "Which environment of %s would you like to show?"
	environmentShowEnvNameHelpPrompt     = "The detail of an environment will be shown (e.g., Account ID, Apps, Tags)."
)

type showEnvVars struct {
	*GlobalOpts
	shouldOutputJSON      bool
	shouldOutputResources bool
	envName               string
}

type showEnvOpts struct {
	showEnvVars

	w             io.Writer
	storeSvc      storeReader
	describer     webAppDescriber
	ws            wsAppReader
	initDescriber func(*showEnvOpts) error // Overriden in tests.
}

func newShowEnvOpts(vars showEnvVars) (*showEnvOpts, error) {
	ssmStore, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to environment datastore: %w", err)
	}
	ws, err := workspace.New()
	if err != nil {
		return nil, err
	}

	return &showEnvOpts{
		showEnvVars: 	vars,
		storeSvc:       ssmStore,
		w:              log.OutputWriter,
		initDescriber: func(o *showEnvOpts) error {
			d, err := describe.NewWebAppDescriber(o.ProjectName(), o.envName)
			if err != nil {
				return fmt.Errorf("creating describer for environment %s in project %s: %w", o.envName, o.ProjectName(), err)
			}
			o.describer = d
			return nil
		},
	}, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *showEnvOpts) Validate() error {
	if o.ProjectName() != "" {
		if _, err := o.storeSvc.GetProject(o.ProjectName()); err != nil {
			return err
		}
	}
	if o.envName != "" {
		if _, err := o.storeSvc.GetApplication(o.ProjectName(), o.envName); err != nil {
			return err
		}
	}

	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *showEnvOpts) Ask() error {
	if err := o.askProject(); err != nil {
		return err
	}
	return o.askEnvName()
}

// Execute shows the applications through the prompt.
func (o *showEnvOpts) Execute() error {
	if o.envName == "" {
		// If there are no local applications in the workspace, we exit without error.
		return nil
	}

	if err := o.initDescriber(o); err != nil {
		return err
	}
	env, err := o.retrieveData()
	if err != nil {
		return err
	}

	if o.shouldOutputJSON {
		data, err := env.JSONString()
		if err != nil {
			return err
		}
		fmt.Fprintf(o.w, data)
	} else {
		fmt.Fprintf(o.w, env.HumanString())
	}

	return nil
}

func (o *showEnvOpts) retrieveData() (*describe.WebApp, error) {
	env, err := o.storeSvc.GetEnvironment(o.ProjectName(), o.envName)
	if err != nil {
		return nil, fmt.Errorf("get application: %w", err)
	}

	applications, err := o.storeSvc.ListApplications(o.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}

	var envVars []*describe.WebAppEnvVars
	var apps []*describe.WebApp
	//var routes []*describe.WebAppRoute
	//var configs []*describe.WebAppConfig
	for _, env := range applications {
		webAppURI, err := o.describer.URI(env.Name)
		if err == nil {
			routes = append(routes, &describe.WebAppRoute{
				Environment: env.Name,
				URL:         webAppURI.String(),
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
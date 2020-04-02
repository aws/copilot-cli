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
	// Note: ws will be removed; put it back in for now so code would compile.
	ws 				wsAppReader
	initDescriber func(*showEnvOpts) error // Overriden in tests.
}

func newShowEnvOpts(vars showEnvVars) (*showEnvOpts, error) {
	ssmStore, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to environment datastore: %w", err)
	}
	//ws, err := workspace.New()
	//if err != nil {
	//	return nil, err
	//}

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

// Execute shows the environments through the prompt.
func (o *showEnvOpts) Execute() error {
	if o.envName == "" {
		// If there are no local applications in the workspace, we exit without error.
		return nil
	}

	if err := o.initDescriber(o); err != nil {
		return err
	}
	//env, err := o.retrieveData()
	//if err != nil {
	//	return err
	//}
	//
	//if o.shouldOutputJSON {
	//	data, err := env.JSONString()
	//	if err != nil {
	//		return err
	//	}
	//	fmt.Fprintf(o.w, data)
	//} else {
	//	fmt.Fprintf(o.w, env.HumanString())
	//}

	return nil
}

func (o *showEnvOpts) retrieveData() (*describe.WebAppEnvVars, error) {
	env, err := o.storeSvc.GetEnvironment(o.ProjectName(), o.envName)
	if err != nil {
		return nil, fmt.Errorf("get environment: %w", err)
	}

	//applications, err := o.storeSvc.ListApplications(o.ProjectName())
	//if err != nil {
	//	return nil, fmt.Errorf("list applications: %w", err)
	//}

	environments, err := o.storeSvc.ListEnvironments(o.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}

	var envVars []*describe.WebAppEnvVars
	//var apps []*describe.WebApp
	//var tags
	//var routes []*describe.WebAppRoute
	//var configs []*describe.WebAppConfig
	//for _, app := range applications {
	//	webAppURI, err := o.describer.URI(app.Name)
	//	if err == nil {
	//		apps = append(apps, &describe.WebApp{
	//			AppName: app.Name,
	//			Type:    app.Type,
	//		})
	//
	//		//webAppECSParams, err := o.describer.ECSParams(env.Name)
	//		//if err != nil {
	//		//	return nil, fmt.Errorf("retrieve application deployment configuration: %w", err)
	//		//}
	//		//configs = append(configs, &describe.WebAppConfig{
	//		//	Environment: env.Name,
	//		//	Port:        webAppECSParams.ContainerPort,
	//		//	Tasks:       webAppECSParams.TaskCount,
	//		//	CPU:         webAppECSParams.CPU,
	//		//	Memory:      webAppECSParams.Memory,
	//		//})
	//
	//		webAppEnvVars, err := o.describer.EnvVars(env)
	//		if err != nil {
	//			return nil, fmt.Errorf("retrieve environment variables: %w", err)
	//		}
	//		envVars = append(envVars, webAppEnvVars...)
	//
	//		continue
	//	}
	//	if !isStackNotExistsErr(err) {
	//		return nil, fmt.Errorf("retrieve application URI: %w", err)
	//	}
	//}
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

	return &describe.WebAppEnvVars{
		Environment:        env.Name,
		//Type:           app.Type,
		//Project:        o.ProjectName(),
		//Configurations: configs,
		//Routes:         routes,
		//Tags:      		,
		//Resources:      resources,
	}, nil
}

func (o *showEnvOpts) askProject() error {
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
		environmentShowProjectNamePrompt,
		environmentShowProjectNameHelpPrompt,
		projNames,
	)
	if err != nil {
		return fmt.Errorf("select projects: %w", err)
	}
	o.projectName = proj

	return nil
}

func (o *showEnvOpts) askEnvName() error {
	// return if env name is set by flag
	if o.envName != "" {
		return nil
	}

	envNames, err := o.retrieveLocalEnvironment()
	if err != nil {
		envNames, err = o.retrieveAllEnvironments()
		if err != nil {
			return err
		}
	}

	if len(envNames) == 0 {
		log.Infof("No environments found in project %s\n.", color.HighlightUserInput(o.ProjectName()))
		return nil
	}
	if len(envNames) == 1 {
		o.envName = envNames[0]
		return nil
	}
	envName, err := o.prompt.SelectOne(
		fmt.Sprintf(environmentShowEnvNamePrompt, color.HighlightUserInput(o.ProjectName())),
		environmentShowEnvNameHelpPrompt,
		envNames,
	)
	if err != nil {
		return fmt.Errorf("select environments for project %s: %w", o.ProjectName(), err)
	}
	o.envName = envName

	return nil
}

func (o *showEnvOpts) retrieveProjects() ([]string, error) {
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

func (o *showEnvOpts) retrieveLocalEnvironment() ([]string, error) {
	localAppNames, err := o.ws.AppNames()
	if err != nil {
		return nil, err
	}
	if len(localAppNames) == 0 {
		return nil, errors.New("no application found")
	}
	return localAppNames, nil
}
//func (o *showEnvOpts) retrieveLocalEnvironment() ([]string, error) {
//	localEnvNames, err := o.ws.EnvNames()
//	if err != nil {
//		return nil, err
//	}
//	if len(localEnvNames) == 0 {
//		return nil, errors.New("no application found")
//	}
//	return localEnvNames, nil
//}

func (o *showEnvOpts) retrieveAllEnvironments() ([]string, error) {
	envs, err := o.storeSvc.ListEnvironments(o.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("list environments for project %s: %w", o.ProjectName(), err)
	}
	envNames := make([]string, len(envs))
	for ind, env := range envs {
		envNames[ind] = env.Name
	}

	return envNames, nil
}

// BuildEnvShowCmd builds the command for showing environments in a project.
func BuildEnvShowCmd() *cobra.Command {
	vars := showEnvVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Shows info about a deployed environment.",
		Long:  "Shows info about a deployed environment, including region, account ID, and apps.",

		Example: `
  Shows info about the environment "test"
  /code $ ecs-preview env show -n test`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newShowEnvOpts(vars)
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
	cmd.Flags().StringVarP(&vars.envName, nameFlag, nameFlagShort, "", envFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputResources, resourcesFlag, false, resourcesFlagDescription)
	cmd.Flags().StringVarP(&vars.projectName, projectFlag, projectFlagShort, "", projectFlagDescription)
	return cmd
}
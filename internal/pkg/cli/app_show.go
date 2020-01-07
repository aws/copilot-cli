// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	applicationShowProjectNamePrompt     = "Which project's applications would you like to show?"
	applicationShowProjectNameHelpPrompt = "A project groups all of your applications together."
	applicationShowAppNamePrompt         = "Which application of %s would you like to show?"
	applicationShowAppNameHelpPrompt     = "The detail of an application will be shown (e.g., endpoint URL, CPU, Memory)."
)

// ShowAppOpts contains the fields to collect for showing an application.
type ShowAppOpts struct {
	shouldOutputJSON      bool
	shouldOutputResources bool
	appName               string

	w             io.Writer
	storeSvc      storeReader
	describer     webAppDescriber
	initDescriber func(*ShowAppOpts) error // Overriden in tests.

	*GlobalOpts
}

// Validate returns an error if the values provided by the user are invalid.
func (o *ShowAppOpts) Validate() error {
	if o.ProjectName() != "" {
		_, err := o.storeSvc.GetProject(o.ProjectName())
		if err != nil {
			return err
		}
	}
	if o.appName != "" {
		_, err := o.storeSvc.GetApplication(o.ProjectName(), o.appName)
		if err != nil {
			return err
		}
	}

	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *ShowAppOpts) Ask() error {
	if err := o.askProject(); err != nil {
		return err
	}
	return o.askAppName()
}

// Execute shows the applications through the prompt.
func (o *ShowAppOpts) Execute() error {
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

func (o *ShowAppOpts) retrieveData() (*describe.WebApp, error) {
	app, err := o.storeSvc.GetApplication(o.ProjectName(), o.appName)
	if err != nil {
		return nil, fmt.Errorf("getting application: %w", err)
	}

	environments, err := o.storeSvc.ListEnvironments(o.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("listing environments: %w", err)
	}

	var routes []describe.WebAppRoute
	var configs []describe.WebAppConfig
	for _, env := range environments {
		webAppURI, err := o.describer.URI(env.Name)
		if err == nil {
			routes = append(routes, describe.WebAppRoute{
				Environment: env.Name,
				URL:         webAppURI.DNSName,
				Path:        webAppURI.Path,
			})
			webAppECSParams, err := o.describer.ECSParams(env.Name)
			if err != nil {
				return nil, fmt.Errorf("retrieving application deployment configuration: %w", err)
			}
			configs = append(configs, describe.WebAppConfig{
				Environment: env.Name,
				Port:        webAppECSParams.ContainerPort,
				Tasks:       webAppECSParams.TaskCount,
				CPU:         webAppECSParams.CPU,
				Memory:      webAppECSParams.Memory,
			})
			continue
		}
		if !applicationNotDeployed(err) {
			return nil, fmt.Errorf("retrieving application URI: %w", err)
		}
	}

	if o.shouldOutputResources {
		resources := make(map[string][]*describe.CfnResource)
		for _, env := range environments {
			webAppResources, err := o.describer.StackResources(env.Name)
			if err == nil {
				resources[env.Name] = webAppResources
				continue
			}
			if !applicationNotDeployed(err) {
				return nil, fmt.Errorf("retrieving application resources: %w", err)
			}
		}
		return &describe.WebApp{
			AppName:        app.Name,
			Type:           app.Type,
			Project:        o.ProjectName(),
			Configurations: configs,
			Routes:         routes,
			Resources:      resources,
		}, nil
	}

	return &describe.WebApp{
		AppName:        app.Name,
		Type:           app.Type,
		Project:        o.ProjectName(),
		Configurations: configs,
		Routes:         routes,
	}, nil
}

func applicationNotDeployed(err error) bool {
	for {
		if err == nil {
			return false
		}
		aerr, ok := err.(awserr.Error)
		if !ok {
			return applicationNotDeployed(errors.Unwrap(err))
		}
		if aerr.Code() != "ValidationError" {
			return applicationNotDeployed(errors.Unwrap(err))
		}
		if !strings.Contains(aerr.Message(), "does not exist") {
			return applicationNotDeployed(errors.Unwrap(err))
		}
		return true
	}
}

func (o *ShowAppOpts) askProject() error {
	if o.ProjectName() != "" {
		return nil
	}
	projNames, err := o.retrieveProjects()
	if err != nil {
		return err
	}
	if len(projNames) == 0 {
		log.Infoln("There are no projects to select.")
	}
	proj, err := o.prompt.SelectOne(
		applicationShowProjectNamePrompt,
		applicationShowProjectNameHelpPrompt,
		projNames,
	)
	if err != nil {
		return fmt.Errorf("selecting projects: %w", err)
	}
	o.projectName = proj

	return nil
}

func (o *ShowAppOpts) askAppName() error {
	if o.appName != "" {
		return nil
	}
	appNames, err := o.retrieveApplications()
	if err != nil {
		return err
	}
	if len(appNames) == 0 {
		log.Infof("No applications found in project '%s'\n.", o.ProjectName())
		return nil
	}
	appName, err := o.prompt.SelectOne(
		fmt.Sprintf(applicationShowAppNamePrompt, color.HighlightUserInput(o.ProjectName())),
		applicationShowAppNameHelpPrompt,
		appNames,
	)
	if err != nil {
		return fmt.Errorf("selecting applications for project %s: %w", o.ProjectName(), err)
	}
	o.appName = appName

	return nil
}

func (o *ShowAppOpts) retrieveProjects() ([]string, error) {
	projs, err := o.storeSvc.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	projNames := make([]string, len(projs))
	for ind, proj := range projs {
		projNames[ind] = proj.Name
	}
	return projNames, nil
}

func (o *ShowAppOpts) retrieveApplications() ([]string, error) {
	apps, err := o.storeSvc.ListApplications(o.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("listing applications for project %s: %w", o.ProjectName(), err)
	}
	appNames := make([]string, len(apps))
	for ind, app := range apps {
		appNames[ind] = app.Name
	}

	return appNames, nil
}

// BuildAppShowCmd builds the command for showing applications in a project.
func BuildAppShowCmd() *cobra.Command {
	opts := ShowAppOpts{
		w:          log.OutputWriter,
		GlobalOpts: NewGlobalOpts(),
		initDescriber: func(o *ShowAppOpts) error {
			d, err := describe.NewWebAppDescriber(o.ProjectName(), o.appName)
			if err != nil {
				return fmt.Errorf("creating describer for application %s in project %s: %w", o.appName, o.ProjectName(), err)
			}
			o.describer = d
			return nil
		},
	}
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Displays information about an application per environment.",
		Example: `
  Shows details for the application "my-app"
  /code $ ecs-preview app show -a my-app`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			ssmStore, err := store.New()
			if err != nil {
				return fmt.Errorf("connect to environment datastore: %w", err)
			}
			opts.storeSvc = ssmStore
			return nil
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
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
	cmd.Flags().StringVarP(&opts.appName, appFlag, appFlagShort, "", appFlagDescription)
	cmd.Flags().BoolVar(&opts.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVarP(&opts.shouldOutputResources, resourcesFlag, resourcesFlagShort, false, resourcesFlagDescription)
	cmd.Flags().StringP(projectFlag, projectFlagShort, "" /* default */, projectFlagDescription)
	viper.BindPFlag(projectFlag, cmd.Flags().Lookup(projectFlag))
	return cmd
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
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
	svcName               string
}

type showAppOpts struct {
	showAppVars

	w             io.Writer
	store         store
	describer     describer
	ws            wsAppReader
	initDescriber func(bool) error // Overriden in tests.
}

func newShowAppOpts(vars showAppVars) (*showAppOpts, error) {
	ssmStore, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to environment datastore: %w", err)
	}
	ws, err := workspace.New()
	if err != nil {
		return nil, err
	}

	opts := &showAppOpts{
		showAppVars: vars,
		store:       ssmStore,
		ws:          ws,
		w:           log.OutputWriter,
	}
	opts.initDescriber = func(enableResources bool) error {
		var d describer
		app, err := opts.store.GetService(opts.AppName(), opts.appName)
		if err != nil {
			return err
		}
		switch app.Type {
		case manifest.LoadBalancedWebServiceType:
			if enableResources {
				d, err = describe.NewWebServiceDescriberWithResources(opts.AppName(), opts.svcName)
			} else {
				d, err = describe.NewWebServiceDescriber(opts.AppName(), opts.svcName)
			}
		case manifest.BackendServiceType:
			if enableResources {
				d, err = describe.NewBackendServiceDescriberWithResources(opts.AppName(), opts.svcName)
			} else {
				d, err = describe.NewBackendServiceDescriber(opts.AppName(), opts.svcName)
			}
		default:
			return fmt.Errorf("invalid application type %s", app.Type)
		}

		if err != nil {
			return fmt.Errorf("creating describer for application %s in project %s: %w", opts.appName, opts.AppName(), err)
		}
		opts.describer = d
		return nil
	}
	return opts, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *showAppOpts) Validate() error {
	if o.AppName() != "" {
		if _, err := o.store.GetApplication(o.AppName()); err != nil {
			return err
		}
	}
	if o.svcName != "" {
		if _, err := o.store.GetService(o.AppName(), o.svcName); err != nil {
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
	if o.svcName == "" {
		// If there are no local applications in the workspace, we exit without error.
		return nil
	}
	err := o.initDescriber(o.shouldOutputResources)
	if err != nil {
		return err
	}
	app, err := o.describer.Describe()
	if err != nil {
		return fmt.Errorf("describe application %s: %w", o.svcName, err)
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

func (o *showAppOpts) askProject() error {
	if o.AppName() != "" {
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
	o.appName = proj

	return nil
}

func (o *showAppOpts) askAppName() error {
	// return if app name is set by flag
	if o.svcName != "" {
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
		log.Infof("No applications found in project %s\n.", color.HighlightUserInput(o.AppName()))
		return nil
	}
	if len(appNames) == 1 {
		o.svcName = appNames[0]
		return nil
	}
	appName, err := o.prompt.SelectOne(
		fmt.Sprintf(applicationShowAppNamePrompt, color.HighlightUserInput(o.AppName())),
		applicationShowAppNameHelpPrompt,
		appNames,
	)
	if err != nil {
		return fmt.Errorf("select applications for project %s: %w", o.AppName(), err)
	}
	o.svcName = appName

	return nil
}

func (o *showAppOpts) retrieveProjects() ([]string, error) {
	projs, err := o.store.ListApplications()
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
	localAppNames, err := o.ws.ServiceNames()
	if err != nil {
		return nil, err
	}
	if len(localAppNames) == 0 {
		return nil, errors.New("no application found")
	}
	return localAppNames, nil
}

func (o *showAppOpts) retrieveAllApplications() ([]string, error) {
	apps, err := o.store.ListServices(o.AppName())
	if err != nil {
		return nil, fmt.Errorf("list applications for project %s: %w", o.AppName(), err)
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
  /code $ ecs-preview app show -n my-app`,
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
	cmd.Flags().StringVarP(&vars.svcName, nameFlag, nameFlagShort, "", svcFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputResources, resourcesFlag, false, resourcesFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, projectFlag, projectFlagShort, "", projectFlagDescription)
	return cmd
}

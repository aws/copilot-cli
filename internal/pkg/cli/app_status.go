// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/spf13/cobra"
)

const (
	appStatusProjectNamePrompt     = "Which project's applications status would you like to show?"
	appStatusProjectNameHelpPrompt = "A project groups all of your applications together."
	appStatusAppNamePrompt         = "Which application's status would you like to show?"
	appStatusAppNameHelpPrompt     = "Displays the service's, tasks and CloudWatch alarms status."
)

type appStatusVars struct {
	*GlobalOpts
	shouldOutputJSON bool
	appName          string
	envName          string
}

type appStatusOpts struct {
	appStatusVars

	w                   io.Writer
	storeClient         storeClient
	appDescriber        serviceArnGetter
	statusDescriber     statusDescriber
	initAppDescriber    func(*appStatusOpts, string, string) error
	initStatusDescriber func(*appStatusOpts) error
}

func newAppStatusOpts(vars appStatusVars) (*appStatusOpts, error) {
	ssmStore, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to environment datastore: %w", err)
	}

	return &appStatusOpts{
		appStatusVars: vars,
		storeClient:   ssmStore,
		w:             log.OutputWriter,
		initAppDescriber: func(o *appStatusOpts, envName, appName string) error {
			d, err := describe.NewAppDescriber(o.ProjectName(), envName, appName)
			if err != nil {
				return fmt.Errorf("creating stack describer for project %s: %w", o.ProjectName(), err)
			}
			o.appDescriber = d
			return nil
		},
		initStatusDescriber: func(o *appStatusOpts) error {
			d, err := describe.NewAppStatus(o.ProjectName(), o.envName, o.appName)
			if err != nil {
				return fmt.Errorf("creating status describer for application %s in project %s: %w", o.appName, o.ProjectName(), err)
			}
			o.statusDescriber = d
			return nil
		},
	}, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *appStatusOpts) Validate() error {
	if o.ProjectName() != "" {
		if _, err := o.storeClient.GetApplication(o.ProjectName()); err != nil {
			return err
		}
	}
	if o.appName != "" {
		if _, err := o.storeClient.GetService(o.ProjectName(), o.appName); err != nil {
			return err
		}
	}
	if o.envName != "" {
		if _, err := o.storeClient.GetEnvironment(o.ProjectName(), o.envName); err != nil {
			return err
		}
	}
	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *appStatusOpts) Ask() error {
	if err := o.askProject(); err != nil {
		return err
	}
	return o.askAppEnvName()
}

// Execute shows the applications through the prompt.
func (o *appStatusOpts) Execute() error {
	err := o.initStatusDescriber(o)
	if err != nil {
		return err
	}
	appStatus, err := o.statusDescriber.Describe()
	if err != nil {
		return fmt.Errorf("describe status of application %s: %w", o.appName, err)
	}
	if o.shouldOutputJSON {
		data, err := appStatus.JSONString()
		if err != nil {
			return err
		}
		fmt.Fprintf(o.w, data)
	} else {
		fmt.Fprintf(o.w, appStatus.HumanString())
	}

	return nil
}

func (o *appStatusOpts) askProject() error {
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
		appStatusProjectNamePrompt,
		appStatusProjectNameHelpPrompt,
		projNames,
	)
	if err != nil {
		return fmt.Errorf("select project: %w", err)
	}
	o.projectName = proj

	return nil
}

func (o *appStatusOpts) retrieveProjects() ([]string, error) {
	projs, err := o.storeClient.ListApplications()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	projNames := make([]string, len(projs))
	for ind, proj := range projs {
		projNames[ind] = proj.Name
	}
	return projNames, nil
}

func (o *appStatusOpts) askAppEnvName() error {
	var err error
	appNames := []string{o.appName}
	if o.appName == "" {
		appNames, err = o.retrieveAllAppNames()
		if err != nil {
			return err
		}
		if len(appNames) == 0 {
			return fmt.Errorf("no applications found in project %s", color.HighlightUserInput(o.ProjectName()))
		}
	}

	envNames := []string{o.envName}
	if o.envName == "" {
		envNames, err = o.retrieveAllEnvNames()
		if err != nil {
			return err
		}
		if len(envNames) == 0 {
			return fmt.Errorf("no environments found in project %s", color.HighlightUserInput(o.ProjectName()))
		}
	}

	appEnvs := make(map[string]appEnv)
	var appEnvNames []string
	for _, appName := range appNames {
		for _, envName := range envNames {
			if err := o.initAppDescriber(o, envName, appName); err != nil {
				return err
			}
			_, err := o.appDescriber.GetServiceArn()
			if err != nil {
				if describe.IsStackNotExistsErr(err) {
					continue
				}
				return fmt.Errorf("check if app %s is deployed in env %s: %w", appName, envName, err)
			}
			appEnv := appEnv{
				appName: appName,
				envName: envName,
			}
			appEnvName := appEnv.String()
			appEnvs[appEnvName] = appEnv
			appEnvNames = append(appEnvNames, appEnvName)
		}
	}
	if len(appEnvNames) == 0 {
		return fmt.Errorf("no deployed apps found in project %s", color.HighlightUserInput(o.ProjectName()))
	}

	// return if only one deployed app found
	if len(appEnvNames) == 1 {
		log.Infof("Only found one deployed app, defaulting to: %s\n", color.HighlightUserInput(appEnvNames[0]))
		o.appName = appEnvs[appEnvNames[0]].appName
		o.envName = appEnvs[appEnvNames[0]].envName

		return nil
	}

	appEnvName, err := o.prompt.SelectOne(
		applicationLogAppNamePrompt,
		applicationLogAppNameHelpPrompt,
		appEnvNames,
	)
	if err != nil {
		return fmt.Errorf("select deployed applications for project %s: %w", o.ProjectName(), err)
	}
	o.appName = appEnvs[appEnvName].appName
	o.envName = appEnvs[appEnvName].envName

	return nil
}

func (o *appStatusOpts) retrieveAllAppNames() ([]string, error) {
	apps, err := o.storeClient.ListServices(o.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("list applications for project %s: %w", o.ProjectName(), err)
	}
	appNames := make([]string, len(apps))
	for ind, app := range apps {
		appNames[ind] = app.Name
	}

	return appNames, nil
}

func (o *appStatusOpts) retrieveAllEnvNames() ([]string, error) {
	envs, err := o.storeClient.ListEnvironments(o.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("list environments for project %s: %w", o.ProjectName(), err)
	}
	envNames := make([]string, len(envs))
	for ind, env := range envs {
		envNames[ind] = env.Name
	}

	return envNames, nil
}

// BuildAppStatusCmd builds the command for showing the status of a deployed application.
func BuildAppStatusCmd() *cobra.Command {
	vars := appStatusVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Shows status of a deployed application.",
		Long:  "Shows status of a deployed application, including service status, task status, and related CloudWatch alarms.",

		Example: `
  Shows status of the deployed application "my-app"
  /code $ ecs-preview app status -n my-app`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newAppStatusOpts(vars)
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
	cmd.Flags().StringVarP(&vars.appName, nameFlag, nameFlagShort, "", appFlagDescription)
	cmd.Flags().StringVarP(&vars.envName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().StringVarP(&vars.projectName, projectFlag, projectFlagShort, "", projectFlagDescription)
	return cmd
}

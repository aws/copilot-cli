// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

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

type serializedWebAppConfig struct {
	Environment string `json:"environment"`
	Port        string `json:"port"`
	Tasks       string `json:"tasks"`
	CPU         string `json:"cpu"`
	Memory      string `json:"memory"`
	URL         string `json:"url"`
	Path        string `json:"path"`
}

type serializedWebApp struct {
	AppName       string                   `json:"appName"`
	Type          string                   `json:"type"`
	Project       string                   `json:"project"`
	DeployConfigs []serializedWebAppConfig `json:"deployConfig"`
}

// ShowAppOpts contains the fields to collect for showing an application.
type ShowAppOpts struct {
	ShouldOutputJSON bool

	app serializedWebApp

	appName string

	storeSvc  storeReader
	describer webAppDescriber

	w io.Writer

	*GlobalOpts
}

// Ask asks for fields that are required but not passed in.
func (o *ShowAppOpts) Ask() error {
	if err := o.askProject(); err != nil {
		return err
	}
	return o.askAppName()
}

// Validate returns an error if the values provided by the user are invalid.
func (o *ShowAppOpts) Validate() error {
	if o.ProjectName() != "" {
		names, err := o.retrieveProjects()
		if err != nil {
			return err
		}
		if !contains(o.ProjectName(), names) {
			return fmt.Errorf("project '%s' does not exist in the workspace", o.ProjectName())
		}
	}
	if o.appName != "" {
		names, err := o.retrieveApplications()
		if err != nil {
			return err
		}
		if !contains(o.appName, names) {
			return fmt.Errorf("application '%s' does not exist in project '%s'", o.appName, o.ProjectName())
		}
	}

	return nil
}

// Execute shows the applications through the prompt.
func (o *ShowAppOpts) Execute() error {
	if o.appName != "" {
		if err := o.retrieveData(); err != nil {
			return err
		}
	} else {
		o.app = serializedWebApp{}
	}

	if o.ShouldOutputJSON {
		data, err := o.jsonOutput()
		if err != nil {
			return err
		}
		fmt.Fprintf(o.w, data)
	} else {
		o.humanOutput()
	}

	return nil
}

func (o *ShowAppOpts) retrieveData() error {
	app, err := o.storeSvc.GetApplication(o.ProjectName(), o.appName)
	if err != nil {
		return fmt.Errorf("getting application: %w", err)
	}
	o.app = serializedWebApp{
		AppName: app.Name,
		Type:    app.Type,
		Project: o.ProjectName(),
	}

	environments, err := o.storeSvc.ListEnvironments(o.ProjectName())
	if err != nil {
		return fmt.Errorf("listing environments: %w", err)
	}

	var serializedDeployConfigs []serializedWebAppConfig
	for _, env := range environments {
		webAppURI, err := o.describer.URI(env.Name)
		if err == nil {
			webAppECSParams, err := o.describer.ECSParams(env.Name)
			if err != nil {
				return fmt.Errorf("retrieving application deployment configuration: %w", err)
			}
			serializedDeployConfigs = append(serializedDeployConfigs, serializedWebAppConfig{
				Environment: env.Name,
				URL:         webAppURI.DNSName,
				Path:        webAppURI.Path,
				Port:        webAppECSParams.ContainerPort,
				Tasks:       webAppECSParams.TaskCount,
				CPU:         webAppECSParams.CPU,
				Memory:      webAppECSParams.Memory,
			})
			continue
		}
		if !applicationNotDeployed(err) {
			return fmt.Errorf("retrieving application URI: %w", err)
		}
	}
	o.app.DeployConfigs = serializedDeployConfigs

	return nil
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

func (o *ShowAppOpts) humanOutput() {
	writer := tabwriter.NewWriter(o.w, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	writer = tabwriter.NewWriter(o.w, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, "  %s\t%s\n", "Project", o.app.Project)
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", o.app.AppName)
	fmt.Fprintf(writer, "  %s\t%s\n", "Type", o.app.Type)
	writer.Flush()
	fmt.Fprintf(writer, color.Bold.Sprint("\nConfigurations\n\n"))
	writer = tabwriter.NewWriter(o.w, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\t%s\n", "Environment", "CPU (vCPU)", "Memory (MiB)", "Port", "Tasks")
	for _, config := range o.app.DeployConfigs {
		fmt.Fprintf(writer, "  %s\t%s\t%s\t%s\t%s\n", config.Environment, cpuToString(config.CPU), config.Memory, config.Port, config.Tasks)
	}
	writer.Flush()
	fmt.Fprintf(writer, color.Bold.Sprint("\nRoutes\n\n"))
	writer = tabwriter.NewWriter(o.w, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, "  %s\t%s\t%s\n", "Environment", "URL", "Path")
	for _, config := range o.app.DeployConfigs {
		fmt.Fprintf(writer, "  %s\t%s\t%s\n", config.Environment, config.URL, config.Path)
	}
	writer.Flush()
}

func cpuToString(s string) string {
	cpuInt, _ := strconv.Atoi(s)
	cpuFloat := float64(cpuInt) / 1024
	return fmt.Sprintf("%g", cpuFloat)
}

func (o *ShowAppOpts) jsonOutput() (string, error) {
	b, err := json.Marshal(o.app)
	if err != nil {
		return "", fmt.Errorf("marshal applications: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
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
	}
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Displays information about an application per environment.",
		Long:  "For Load Balanced Web Applications, displays the URL and path the application can be accessed at.",
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
			if opts.appName != "" {
				describer, err := describe.NewWebAppDescriber(opts.ProjectName(), opts.appName)
				if err != nil {
					return fmt.Errorf("creating describer for application %s in project %s: %w", opts.appName, opts.ProjectName(), err)
				}
				opts.describer = describer
			}
			if err := opts.Execute(); err != nil {
				return err
			}

			return nil
		}),
	}
	// The flags bound by viper are available to all sub-commands through viper.GetString({flagName})
	cmd.Flags().StringVarP(&opts.appName, appFlag, appFlagShort, "", appFlagDescription)
	cmd.Flags().BoolVar(&opts.ShouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().StringP(projectFlag, projectFlagShort, "" /* default */, projectFlagDescription)
	viper.BindPFlag(projectFlag, cmd.Flags().Lookup(projectFlag))
	return cmd
}

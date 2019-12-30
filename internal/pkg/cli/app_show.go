// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
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

type serializedLoadBalancedWebAppEnv struct {
	Name   string `json:"name"`
	Region string `json:"region"`
	Prod   bool   `json:"prod"`
	URL    string `json:"url"`
	Path   string `json:"path"`
}

type serializedLoadBalancedWebApp struct {
	AppName      string                            `json:"appName"`
	Type         string                            `json:"type"`
	Project      string                            `json:"project"`
	Account      string                            `json:"account"`
	Environments []serializedLoadBalancedWebAppEnv `json:"environments"`
	// Service      serializedService `json:"service"`
}

// ShowAppOpts contains the fields to collect for showing an application.
type ShowAppOpts struct {
	ShouldOutputJSON bool

	proj *archer.Project
	app  serializedLoadBalancedWebApp

	appName string

	storeSvc   storeReader
	identifier resourceIdentifier

	w io.Writer

	*GlobalOpts
}

// Ask asks for fields that are required but not passed in.
func (opts *ShowAppOpts) Ask() error {
	// get project
	if opts.ProjectName() == "" {
		projectName, err := opts.selectProject()
		if err != nil {
			return err
		}
		opts.projectName = projectName
	}

	// get application of the project
	if opts.appName == "" {
		appName, err := opts.selectApplication()
		if err != nil {
			return err
		}
		opts.appName = appName
	}

	return nil
}

// Validate returns an error if the user inputs are invalid.
func (opts *ShowAppOpts) Validate() error {
	proj, err := opts.storeSvc.GetProject(opts.ProjectName())
	if err != nil {
		return fmt.Errorf("getting project: %w", err)
	}
	opts.proj = proj

	return nil
}

// Execute shows the applications through the prompt.
func (opts *ShowAppOpts) Execute() error {
	if opts.appName != "" {
		if err := opts.retrieveData(); err != nil {
			return err
		}
	} else {
		opts.app = serializedLoadBalancedWebApp{}
	}

	if opts.ShouldOutputJSON {
		data, err := opts.jsonOutput()
		if err != nil {
			return err
		}
		fmt.Fprintf(opts.w, data)
	} else {
		opts.humanOutput()
	}

	return nil
}

func (opts *ShowAppOpts) retrieveData() error {
	app, err := opts.storeSvc.GetApplication(opts.ProjectName(), opts.appName)
	if err != nil {
		return fmt.Errorf("getting application: %w", err)
	}
	opts.app = serializedLoadBalancedWebApp{
		AppName: app.Name,
		Type:    app.Type,
		Project: opts.ProjectName(),
		Account: opts.proj.AccountID,
	}

	environments, err := opts.storeSvc.ListEnvironments(opts.ProjectName())
	if err != nil {
		return fmt.Errorf("listing environments: %w", err)
	}

	var serializedEnvs []serializedLoadBalancedWebAppEnv
	for _, env := range environments {
		webAppURI, err := opts.identifier.URI(env.Name)
		if err == nil {
			serializedEnvs = append(serializedEnvs, serializedLoadBalancedWebAppEnv{
				Name:   env.Name,
				Region: env.Region,
				Prod:   env.Prod,
				URL:    webAppURI.DNSName,
				Path:   webAppURI.Path,
			})
			continue
		}
		if !applicationNotDeployed(err) {
			return fmt.Errorf("retrieving application URI: %w", err)
		}
	}
	opts.app.Environments = serializedEnvs

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

func (opts *ShowAppOpts) humanOutput() {
	writer := tabwriter.NewWriter(opts.w, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", "Environment", "Is Production?", "Path", "URL")
	envLengthMax := len("Environment")
	prodLengthMax := len("Is Production?")
	pathLengthMax := len("Path")
	urlLengthMax := len("URL")
	for _, env := range opts.app.Environments {
		envLengthMax = int(math.Max(float64(envLengthMax), float64(len(env.Name))))
		prodLengthMax = int(math.Max(float64(prodLengthMax), float64(len(strconv.FormatBool(env.Prod)))))
		pathLengthMax = int(math.Max(float64(pathLengthMax), float64(len(env.Path))))
		urlLengthMax = int(math.Max(float64(urlLengthMax), float64(len(env.URL))))
	}
	fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", strings.Repeat("-", envLengthMax), strings.Repeat("-", prodLengthMax), strings.Repeat("-", pathLengthMax), strings.Repeat("-", urlLengthMax))
	for _, env := range opts.app.Environments {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", env.Name, strconv.FormatBool(env.Prod), env.Path, env.URL)
	}
	writer.Flush()
}

func (opts *ShowAppOpts) jsonOutput() (string, error) {
	b, err := json.Marshal(opts.app)
	if err != nil {
		return "", fmt.Errorf("marshal applications: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

func (opts *ShowAppOpts) selectProject() (string, error) {
	projNames, err := opts.retrieveProjects()
	if err != nil {
		return "", err
	}
	if len(projNames) == 0 {
		log.Infoln("There are no projects to select.")
	}
	proj, err := opts.prompt.SelectOne(
		applicationShowProjectNamePrompt,
		applicationShowProjectNameHelpPrompt,
		projNames,
	)
	if err != nil {
		return "", fmt.Errorf("selecting projects: %w", err)
	}
	return proj, nil
}

func (opts *ShowAppOpts) selectApplication() (string, error) {
	appNames, err := opts.retrieveApplications()
	if err != nil {
		return "", err
	}
	if len(appNames) == 0 {
		return "", nil
	}
	appName, err := opts.prompt.SelectOne(
		fmt.Sprintf(applicationShowAppNamePrompt, color.HighlightUserInput(opts.ProjectName())),
		applicationShowAppNameHelpPrompt,
		appNames,
	)
	if err != nil {
		return "", fmt.Errorf("selecting applications for project %s: %w", opts.ProjectName(), err)
	}
	return appName, nil
}

func (opts *ShowAppOpts) retrieveProjects() ([]string, error) {
	projs, err := opts.storeSvc.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	projNames := make([]string, len(projs))
	for ind, proj := range projs {
		projNames[ind] = proj.Name
	}
	return projNames, nil
}

func (opts *ShowAppOpts) retrieveApplications() ([]string, error) {
	apps, err := opts.storeSvc.ListApplications(opts.ProjectName())
	if err != nil {
		return nil, fmt.Errorf("listing applications for project %s: %w", opts.ProjectName(), err)
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
			if err := opts.Ask(); err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			if opts.appName != "" {
				identifier, err := describe.NewWebAppDescriber(opts.ProjectName(), opts.appName)
				if err != nil {
					return fmt.Errorf("creating identifier for application %s in project %s: %w", opts.appName, opts.ProjectName(), err)
				}
				opts.identifier = identifier
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

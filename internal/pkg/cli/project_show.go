// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ShowProjectOpts contains the fields to collect for showing a project.
type ShowProjectOpts struct {
	shouldOutputJSON bool

	storeSvc storeReader

	w io.Writer

	*GlobalOpts
}

// Validate returns an error if the values provided by the user are invalid.
func (o *ShowProjectOpts) Validate() error {
	if o.ProjectName() != "" {
		_, err := o.storeSvc.GetProject(o.ProjectName())
		if err != nil {
			return err
		}
	}

	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *ShowProjectOpts) Ask() error {
	if err := o.askProject(); err != nil {
		return err
	}

	return nil
}

// Execute shows the applications through the prompt.
func (o *ShowProjectOpts) Execute() error {
	proj, err := o.storeSvc.GetProject(o.ProjectName())
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}
	envs, err := o.storeSvc.ListEnvironments(o.ProjectName())
	if err != nil {
		return fmt.Errorf("list environment: %w", err)
	}
	apps, err := o.storeSvc.ListApplications(o.ProjectName())
	if err != nil {
		return fmt.Errorf("list application: %w", err)
	}

	if o.shouldOutputJSON {
		data, err := o.jsonOutPut(proj, envs, apps)
		if err != nil {
			return err
		}
		fmt.Fprintf(o.w, data)
	} else {
		fmt.Fprintf(o.w, o.humanOutPut(proj, envs, apps))
	}

	return nil
}

func (o *ShowProjectOpts) humanOutPut(proj *archer.Project, envs []*archer.Environment, apps []*archer.Application) string {
	var b bytes.Buffer
	writer := tabwriter.NewWriter(&b, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	fmt.Fprintf(writer, color.Bold.Sprint("About\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", proj.Name)
	fmt.Fprintf(writer, "  %s\t%s\n", "URI", proj.Domain)
	fmt.Fprintf(writer, color.Bold.Sprint("\nEnvironments\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\t%s\n", "Name", "AccountID", "Region")
	for _, env := range envs {
		fmt.Fprintf(writer, "  %s\t%s\t%s\n", env.Name, env.AccountID, env.Region)
	}
	fmt.Fprintf(writer, color.Bold.Sprint("\nApplications\n\n"))
	writer.Flush()
	fmt.Fprintf(writer, "  %s\t%s\n", "Name", "Type")
	for _, app := range apps {
		fmt.Fprintf(writer, "  %s\t%s\n", app.Name, app.Type)
	}
	writer.Flush()
	return b.String()
}

func (o *ShowProjectOpts) jsonOutPut(proj *archer.Project, envs []*archer.Environment, apps []*archer.Application) (string, error) {
	type environment struct {
		Name      string `json:"name"`
		AccountID string `json:"accountID"`
		Region    string `json:"region"`
	}
	type application struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	type project struct {
		Name string         `json:"name"`
		URI  string         `json:"uri"`
		Envs []*environment `json:"environments"`
		Apps []*application `json:"applications"`
	}
	var envsToSerialize []*environment
	for _, env := range envs {
		envsToSerialize = append(envsToSerialize, &environment{
			Name:      env.Name,
			AccountID: env.AccountID,
			Region:    env.Region,
		})
	}
	var appsToSerialize []*application
	for _, app := range apps {
		appsToSerialize = append(appsToSerialize, &application{
			Name: app.Name,
			Type: app.Type,
		})
	}
	projectToSerialize := &project{
		Name: proj.Name,
		URI:  proj.Domain,
		Envs: envsToSerialize,
		Apps: appsToSerialize,
	}
	b, err := json.Marshal(projectToSerialize)
	if err != nil {
		return "", fmt.Errorf("marshal project: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

func (o *ShowProjectOpts) askProject() error {
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
		return fmt.Errorf("select project: %w", err)
	}
	o.projectName = proj

	return nil
}

func (o *ShowProjectOpts) retrieveProjects() ([]string, error) {
	projs, err := o.storeSvc.ListProjects()
	if err != nil {
		return nil, fmt.Errorf("list project: %w", err)
	}
	projNames := make([]string, len(projs))
	for ind, proj := range projs {
		projNames[ind] = proj.Name
	}
	return projNames, nil
}

// BuildProjectShowCmd builds the command for showing details of a project.
func BuildProjectShowCmd() *cobra.Command {
	opts := ShowProjectOpts{
		w:          log.OutputWriter,
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show details of a project.",
		Example: `
  Shows details for the project "my-project"
  /code $ ecs-preview project show -p my-project`,
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
			if err := opts.Execute(); err != nil {
				return err
			}

			return nil
		}),
	}
	// The flags bound by viper are available to all sub-commands through viper.GetString({flagName})
	cmd.Flags().BoolVar(&opts.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().StringP(projectFlag, projectFlagShort, "" /* default */, projectFlagDescription)
	viper.BindPFlag(projectFlag, cmd.Flags().Lookup(projectFlag))
	return cmd
}

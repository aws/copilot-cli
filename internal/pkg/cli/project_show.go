// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	appShowAppNamePrompt     = "Which application would you like to show?"
	appShowAppNameHelpPrompt = "An application groups all of your services together."
)

type showProjectVars struct {
	*GlobalOpts
	shouldOutputJSON bool
}

type showProjectOpts struct {
	showProjectVars

	store store
	w     io.Writer
}

func newShowProjectOpts(vars showProjectVars) (*showProjectOpts, error) {
	ssmStore, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to environment datastore: %w", err)
	}

	return &showProjectOpts{
		showProjectVars: vars,
		store:           ssmStore,
		w:               log.OutputWriter,
	}, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *showProjectOpts) Validate() error {
	if o.AppName() != "" {
		_, err := o.store.GetApplication(o.AppName())
		if err != nil {
			return err
		}
	}

	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *showProjectOpts) Ask() error {
	if err := o.askProject(); err != nil {
		return err
	}

	return nil
}

// Execute shows the applications through the prompt.
func (o *showProjectOpts) Execute() error {
	projectToSerialize, err := o.retrieveData()
	if err != nil {
		return err
	}
	if o.shouldOutputJSON {
		data, err := projectToSerialize.JSONString()
		if err != nil {
			return err
		}
		fmt.Fprintf(o.w, data)
	} else {
		fmt.Fprintf(o.w, projectToSerialize.HumanString())
	}

	return nil
}

func (o *showProjectOpts) retrieveData() (*describe.App, error) {
	proj, err := o.store.GetApplication(o.AppName())
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	envs, err := o.store.ListEnvironments(o.AppName())
	if err != nil {
		return nil, fmt.Errorf("list environment: %w", err)
	}
	apps, err := o.store.ListServices(o.AppName())
	if err != nil {
		return nil, fmt.Errorf("list application: %w", err)
	}
	var envsToSerialize []*config.Environment
	for _, env := range envs {
		envsToSerialize = append(envsToSerialize, &config.Environment{
			Name:      env.Name,
			AccountID: env.AccountID,
			Region:    env.Region,
			Prod:      env.Prod,
		})
	}
	var appsToSerialize []*config.Service
	for _, app := range apps {
		appsToSerialize = append(appsToSerialize, &config.Service{
			Name: app.Name,
			Type: app.Type,
		})
	}
	return &describe.App{
		Name:     proj.Name,
		URI:      proj.Domain,
		Envs:     envsToSerialize,
		Services: appsToSerialize,
	}, nil
}

func (o *showProjectOpts) askProject() error {
	if o.AppName() != "" {
		return nil
	}
	projNames, err := o.retrieveProjects()
	if err != nil {
		return err
	}
	if len(projNames) == 0 {
		return fmt.Errorf("no project found: run %s to set one up, or %s into your workspace please", color.HighlightCode("project init"), color.HighlightCode("cd"))
	}
	proj, err := o.prompt.SelectOne(
		appShowAppNamePrompt,
		appShowAppNameHelpPrompt,
		projNames,
	)
	if err != nil {
		return fmt.Errorf("select project: %w", err)
	}
	o.appName = proj

	return nil
}

func (o *showProjectOpts) retrieveProjects() ([]string, error) {
	projs, err := o.store.ListApplications()
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
	vars := showProjectVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Shows info about a project.",
		Long:  "Shows configuration, environments and applications for a project.",
		Example: `
  Shows info about the project "my-project"
  /code $ ecs-preview project show -n my-project`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newShowProjectOpts(vars)
			if err != nil {
				return err
			}
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
	cmd.Flags().BoolVar(&vars.shouldOutputJSON, jsonFlag, false, jsonFlagDescription)
	cmd.Flags().StringVarP(&vars.appName, nameFlag, nameFlagShort, "" /* default */, appFlagDescription)
	return cmd
}

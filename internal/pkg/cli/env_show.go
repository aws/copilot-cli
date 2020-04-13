// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/spf13/cobra"
)

const (
	environmentShowProjectNamePrompt     = "Which project's environments would you like to show?"
	environmentShowProjectNameHelpPrompt = "A project groups all of your applications together."
	fmtEnvironmentShowEnvNamePrompt      = "Which environment of %s would you like to show?"
	environmentShowEnvNameHelpPrompt     = "The detail of an environment will be shown (e.g., region, account ID, apps)."
)

type showEnvVars struct {
	*GlobalOpts
	shouldOutputJSON      bool
	shouldOutputResources bool
	envName               string
}

type showEnvOpts struct {
	showEnvVars

	w        io.Writer
	storeSvc storeReader
}

func newShowEnvOpts(vars showEnvVars) (*showEnvOpts, error) {
	ssmStore, err := store.New()
	if err != nil {
		return nil, fmt.Errorf("connect to environment datastore: %w", err)
	}

	return &showEnvOpts{
		showEnvVars: vars,
		storeSvc:    ssmStore,
		w:           log.OutputWriter,
		// initDescriber: make EnvDescriber here
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
		if _, err := o.storeSvc.GetEnvironment(o.ProjectName(), o.envName); err != nil {
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
	return nil
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
	if len(projNames) == 1 {
		o.projectName = projNames[0]
		return nil
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
	//return if env name is set by flag
	if o.envName != "" {
		return nil
	}

	envNames, err := o.retrieveAllEnvironments()
	if err != nil {
		return err
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
		fmt.Sprintf(fmtEnvironmentShowEnvNamePrompt, color.HighlightUserInput(o.ProjectName())), environmentShowEnvNameHelpPrompt, envNames,
	)
	if err != nil {
		return fmt.Errorf("select environment for project %s: %w", o.ProjectName(), err)
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
		Hidden: true, //TODO remove when ready for production!
		Use:    "show",
		Short:  "Shows info about a deployed environment.",
		Long:   "Shows info about a deployed environment, including region, account ID, and apps.",

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

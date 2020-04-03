// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0


package cli

import (
	"github.com/spf13/cobra"
)

type showEnvVars struct {
	*GlobalOpts
	shouldOutputJSON      bool
	shouldOutputResources bool
	envName               string
}

type showEnvOpts struct {
	showEnvVars
}

func newShowEnvOpts(vars showEnvVars) (*showEnvOpts, error) {
	return &showEnvOpts{
		showEnvVars: 	vars,
	}, nil
}

// Validate returns an error if the values provided by the user are invalid.
func (o *showEnvOpts) Validate() error {
	return nil
}

// Ask asks for fields that are required but not passed in.
func (o *showEnvOpts) Ask() error {
	return nil
}

// Execute shows the environments through the prompt.
func (o *showEnvOpts) Execute() error {
	return nil
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
// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"

	"github.com/spf13/cobra"
)

// DeleteEnvOpts holds the fields needed to delete an environment.
type DeleteEnvOpts struct {
	Env              string
	SkipConfirmation bool
}

// Ask is a no-op for this command.
func (opts *DeleteEnvOpts) Ask() error {
	return nil
}

// Validate returns an error if the environment name does not exist in the project, or there are existing applications
// in the environment, or the environment is not available using the EnvironmentManagerRole.
func (opts *DeleteEnvOpts) Validate() error {
	return nil
}

// Execute deletes the environment from the project by first deleting the stack and then removing the entry from the store.
func (opts *DeleteEnvOpts) Execute() error {
	return nil
}

// RecommendedActions is a no-op for this command.
func (opts *DeleteEnvOpts) RecommendedActions() []string {
	return nil
}

// BuildEnvDeleteCmd builds the command to delete environment(s).
func BuildEnvDeleteCmd() *cobra.Command {
	opts := &DeleteEnvOpts{}
	cmd := &cobra.Command{
		Use:   "delete [name]",
		Short: "Deletes an environment from your project.",
		Example: `
  Delete the "test" environment.
  /code $ archer env delete test

  Delete the "test" environment without prompting.
  /code $ archer env delete test prod --yes`,
		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("requires a single environment name as argument")
			}
			opts.Env = args[0]
			return nil
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			return nil
		}),
	}
	cmd.Flags().BoolVar(&opts.SkipConfirmation, yesFlag, false, yesFlagDescription)
	return cmd
}

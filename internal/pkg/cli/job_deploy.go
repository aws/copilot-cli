// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/command"
	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/cobra"
)

type deployJobVars struct {
	*GlobalOpts
	Name         string
	EnvName      string
	ImageTag     string
	ResourceTags map[string]string
}

type deployJobOpts struct {
	deployJobVars

	store        store
	ws           wsSvcDirReader
	unmarshal    func(in []byte) (interface{}, error)
	cmd          runner
	sessProvider sessionProvider

	spinner progress
	sel     wsSelector

	// imageBuilderPusher imageBuilderPusher
	// s3           artifactUploader
	// addons       templater
	// appCFN       appResourcesGetter
	// jobCFN       cloudformation.CloudFormation

	// // cached variables
	// targetApp         *config.Application
	// targetEnvironment *config.Environment
	// targetWorkload    *config.Service
}

func newJobDeployOpts(vars deployJobVars) (*deployJobOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace: %w", err)
	}
	return &deployJobOpts{
		deployJobVars: vars,

		store:        store,
		ws:           ws,
		unmarshal:    manifest.UnmarshalService,
		spinner:      termprogress.NewSpinner(),
		sel:          selector.NewWorkspaceSelect(vars.prompt, store, ws),
		cmd:          command.New(),
		sessProvider: sessions.NewProvider(),
	}, nil
}

// Validate returns an error if the user inputs are invalid.
func (o *deployJobOpts) Validate() error {
	return nil
}

// Ask prompts the user for any required fields that are not provided.
func (o *deployJobOpts) Ask() error {
	return nil
}

// Execute builds and pushes the container image for the job,
func (o *deployJobOpts) Execute() error {
	return nil
}

// RecommendedActions returns follow-up actions the user can take after successfully executing the command.
func (o *deployJobOpts) RecommendedActions() []string {
	return nil
}

// BuildJobDeployCmd builds the `job deploy` subcommand.
func BuildJobDeployCmd() *cobra.Command {
	vars := deployJobVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploys a job to an environment.",
		Long:  `Deploys a job to an environment.`,
		Example: `
  Deploys a job named "report-gen" to a "test" environment.
  /code $ copilot job deploy --name report-gen --env test
  Deploys a job with additional resource tags.
  /code $ copilot job deploy --resource-tags source/revision=bb133e7,deployment/initiator=manual`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newJobDeployOpts(vars)
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
	cmd.Flags().StringVarP(&vars.Name, nameFlag, nameFlagShort, "", jobFlagDescription)
	cmd.Flags().StringVarP(&vars.EnvName, envFlag, envFlagShort, "", envFlagDescription)
	cmd.Flags().StringVar(&vars.ImageTag, imageTagFlag, "", imageTagFlagDescription)
	cmd.Flags().StringToStringVar(&vars.ResourceTags, resourceTagsFlag, nil, resourceTagsFlagDescription)

	return cmd
}

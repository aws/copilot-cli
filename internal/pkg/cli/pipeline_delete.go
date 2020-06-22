// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/secretsmanager"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"

	"github.com/spf13/cobra"
)

const (
	pipelineDeleteConfirmPrompt       = "Are you sure you want to delete pipeline %s from application %s?"
	pipelineDeleteConfirmHelp         = "This will delete the deployment pipeline for the services in the workspace."
	pipelineSecretDeleteConfirmPrompt = "Are you sure you want to delete the source secret %s associated with pipeline %s?"
	pipelineDeleteSecretConfirmHelp   = "This will delete the token associated with the source of your pipeline."

	fmtDeletePipelineStart    = "Deleting pipeline %s from application %s."
	fmtDeletePipelineFailed   = "Failed to delete pipeline %s from application %s: %v."
	fmtDeletePipelineComplete = "Deleted pipeline %s from application %s."
)

var (
	errPipelineDeleteCancelled = errors.New("pipeline delete cancelled - no changes made")
)

type deletePipelineVars struct {
	*GlobalOpts
	SkipConfirmation bool
	DeleteSecret     bool
}

type deletePipelineOpts struct {
	deletePipelineVars

	PipelineName   string
	PipelineSecret string

	// Interfaces to dependencies
	pipelineDeployer pipelineDeployer
	prog             progress
	secretsmanager   secretsManager
	ws               wsPipelineReader
}

func newDeletePipelineOpts(vars deletePipelineVars) (*deletePipelineOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace client: %w", err)
	}

	secretsmanager, err := secretsmanager.New()
	if err != nil {
		return nil, fmt.Errorf("new secrets manager client: %w", err)
	}

	defaultSess, err := session.NewProvider().Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}

	opts := &deletePipelineOpts{
		deletePipelineVars: vars,
		prog:               termprogress.NewSpinner(),
		secretsmanager:     secretsmanager,
		pipelineDeployer:   cloudformation.New(defaultSess),
		ws:                 ws,
	}

	return opts, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *deletePipelineOpts) Validate() error {
	if o.AppName() == "" {
		return errNoAppInWorkspace
	}

	if err := o.readPipelineManifest(); err != nil {
		return err
	}

	return nil
}

// Ask prompts for fields that are required but not passed in.
func (o *deletePipelineOpts) Ask() error {
	if o.SkipConfirmation {
		return nil
	}

	deleteConfirmed, err := o.prompt.Confirm(
		fmt.Sprintf(pipelineDeleteConfirmPrompt, o.PipelineName, o.AppName()),
		pipelineDeleteConfirmHelp)

	if err != nil {
		return fmt.Errorf("pipeline delete confirmation prompt: %w", err)
	}

	if !deleteConfirmed {
		return errPipelineDeleteCancelled
	}

	return nil
}

// Execute deletes the secret and pipeline stack.
func (o *deletePipelineOpts) Execute() error {
	if err := o.deleteSecret(); err != nil {
		return err
	}

	if err := o.deleteStack(); err != nil {
		return err
	}

	return nil
}

func (o *deletePipelineOpts) readPipelineManifest() error {
	data, err := o.ws.ReadPipelineManifest()
	if err != nil {
		if err == workspace.ErrNoPipelineInWorkspace {
			return err
		}
		return fmt.Errorf("read pipeline manifest: %w", err)
	}

	pipeline, err := manifest.UnmarshalPipeline(data)
	if err != nil {
		return fmt.Errorf("unmarshal pipeline manifest: %w", err)
	}

	o.PipelineName = pipeline.Name

	if secret, ok := (pipeline.Source.Properties["access_token_secret"]).(string); ok {
		o.PipelineSecret = secret
	}

	return nil
}

func (o *deletePipelineOpts) deleteSecret() error {
	if !o.DeleteSecret {
		confirmDeletion, err := o.prompt.Confirm(
			fmt.Sprintf(pipelineSecretDeleteConfirmPrompt, o.PipelineSecret, o.PipelineName),
			pipelineDeleteSecretConfirmHelp,
		)
		if err != nil {
			return fmt.Errorf("pipeline delete secret confirmation prompt: %w", err)
		}

		if !confirmDeletion {
			log.Infof("Skipping deletion of secret %s.\n", o.PipelineSecret)
			return nil
		}
	}

	if err := o.secretsmanager.DeleteSecret(o.PipelineSecret); err != nil {
		return err
	}

	log.Successf("Deleted secret %s.\n", o.PipelineSecret)

	return nil
}

func (o *deletePipelineOpts) deleteStack() error {
	o.prog.Start(fmt.Sprintf(fmtDeletePipelineStart, o.PipelineName, o.AppName()))
	if err := o.pipelineDeployer.DeletePipeline(o.PipelineName); err != nil {
		o.prog.Stop(log.Serrorf(fmtDeletePipelineFailed, o.PipelineName, o.AppName(), err))
		return err
	}
	o.prog.Stop(log.Ssuccessf(fmtDeletePipelineComplete, o.PipelineName, o.AppName()))
	return nil
}

// RecommendedActions is a no-op for this command.
func (o *deletePipelineOpts) RecommendedActions() []string {
	return nil
}

// Run validates user input, asks for any missing flags, and then executes the command.
func (o *deletePipelineOpts) Run() error {
	if err := o.Validate(); err != nil {
		return err
	}
	if err := o.Ask(); err != nil {
		return err
	}
	if err := o.Execute(); err != nil {
		return err
	}
	return nil
}

// BuildPipelineDeleteCmd build the command for deleting an existing pipeline.
func BuildPipelineDeleteCmd() *cobra.Command {
	vars := deletePipelineVars{
		GlobalOpts: NewGlobalOpts(),
	}
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Deletes the pipeline associated with your workspace.",
		Example: `
  Delete the pipeline associated with your workspace.
  /code $ copilot pipeline delete`,

		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newDeletePipelineOpts(vars)
			if err != nil {
				return err
			}
			return opts.Run()
		}),
	}
	cmd.Flags().BoolVar(&vars.SkipConfirmation, yesFlag, false, yesFlagDescription)
	cmd.Flags().BoolVar(&vars.DeleteSecret, deleteSecretFlag, false, deleteSecretFlagDescription)
	return cmd
}

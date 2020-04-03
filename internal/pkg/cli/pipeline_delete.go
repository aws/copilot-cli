// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
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
	pipelineDeleteConfirmPrompt       = "Are you sure you want to delete pipeline %s from project %s?"
	pipelineDeleteConfirmHelp         = "This will delete the deployment pipeline for the app(s) in the current project workspace."
	pipelineSecretDeleteConfirmPrompt = "Are you sure you want to delete the source secret %s associated with pipeline %s?"
	pipelineDeleteSecretConfirmHelp   = "This will delete the secret that stores the token associated with the source of your pipeline in the current project workspace."

	fmtDeletePipelineStart    = "Deleting pipeline %s from project %s."
	fmtDeletePipelineFailed   = "Failed to delete pipeline %s from project %s: %v."
	fmtDeletePipelineComplete = "Deleted pipeline %s from project %s."
)

var (
	errNoPipelineInWorkspace = errors.New("there was no pipeline manifest found in your workspace. Please run `ecs-preview pipeline init` to create an pipeline")

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
	secretsmanager   archer.SecretsManager
	ws               wsPipelineDeleter
}

func newDeletePipelineOpts(vars deletePipelineVars) (*deletePipelineOpts, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}

	data, err := ws.ReadPipelineManifest()
	if err != nil {
		return nil, fmt.Errorf("read pipeline manifest: %w", err)
	}

	pipeline, err := manifest.UnmarshalPipeline(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal pipeline manifest: %w", err)
	}

	secretsmanager, err := secretsmanager.New()
	if err != nil {
		return nil, fmt.Errorf("couldn't create secrets manager: %w", err)
	}

	p := session.NewProvider()
	defaultSession, err := p.Default()
	if err != nil {
		return nil, err
	}

	opts := &deletePipelineOpts{
		deletePipelineVars: vars,
		prog:               termprogress.NewSpinner(),
		secretsmanager:     secretsmanager,
		pipelineDeployer:   cloudformation.New(defaultSession),
		ws:                 ws,
	}
	opts.PipelineName = pipeline.Name
	if secret, ok := (pipeline.Source.Properties["access_token_secret"]).(string); ok {
		opts.PipelineSecret = secret
	}

	return opts, nil
}

// Ask prompts for fields that are required but not passed in.
func (o *deletePipelineOpts) Ask() error {
	if o.SkipConfirmation {
		return nil
	}

	deleteConfirmed, err := o.prompt.Confirm(
		fmt.Sprintf(pipelineDeleteConfirmPrompt, o.PipelineName, o.ProjectName()),
		pipelineDeleteConfirmHelp)

	if err != nil {
		return fmt.Errorf("pipeline delete confirmation prompt: %w", err)
	}

	if !deleteConfirmed {
		return errPipelineDeleteCancelled
	}

	return nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *deletePipelineOpts) Validate() error {
	if o.ProjectName() == "" {
		return errNoProjectInWorkspace
	}

	if o.PipelineName == "" {
		return errNoPipelineInWorkspace
	}

	return nil
}

// Execute deletes the secret and pipeline stack
func (o *deletePipelineOpts) Execute() error {
	if err := o.deleteSecret(); err != nil {
		return err
	}

	if err := o.deleteStack(); err != nil {
		return err
	}

	if err := o.deletePipelineFile(); err != nil {
		return err
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

func (o *deletePipelineOpts) stackName() string {
	return o.ProjectName() + "-" + o.PipelineName // based on stack.pipelineStackConfig.StackName()
}

func (o *deletePipelineOpts) deleteStack() error {
	o.prog.Start(fmt.Sprintf(fmtDeletePipelineStart, o.PipelineName, o.ProjectName()))
	stackName := o.stackName()

	if err := o.pipelineDeployer.DeletePipeline(stackName); err != nil {
		o.prog.Stop(log.Serrorf(fmtDeletePipelineFailed, o.PipelineName, o.ProjectName(), err))
		return err
	}

	o.prog.Stop(log.Ssuccessf(fmtDeletePipelineComplete, o.PipelineName, o.ProjectName()))

	return nil
}

func (o *deletePipelineOpts) deletePipelineFile() error {
	err := o.ws.DeletePipelineManifest()
	if err == nil {
		log.Successln("Deleted pipeline manifest from workspace.")
	}

	return err
}

// RecommendedActions is a no-op for this command.
func (o *deletePipelineOpts) RecommendedActions() []string {
	return nil
}


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
			Delete the pipeline associated with your workspace:
			/code $ ecs-preview pipeline delete`,

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

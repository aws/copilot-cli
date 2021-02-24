// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	deploycfn "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/aws/aws-sdk-go/aws"

	termprogress "github.com/aws/copilot-cli/internal/pkg/term/progress"
	"github.com/spf13/cobra"
)

const (
	fmtPipelineUpdateResourcesStart    = "Adding pipeline resources to your application: %s"
	fmtPipelineUpdateResourcesFailed   = "Failed to add pipeline resources to your application: %s\n"
	fmtPipelineUpdateResourcesComplete = "Successfully added pipeline resources to your application: %s\n"

	fmtPipelineUpdateStart    = "Creating a new pipeline: %s"
	fmtPipelineUpdateFailed   = "Failed to create a new pipeline: %s.\n"
	fmtPipelineUpdateComplete = "Successfully created a new pipeline: %s\n"

	fmtPipelineUpdateProposalStart    = "Proposing infrastructure changes for the pipeline: %s"
	fmtPipelineUpdateProposalFailed   = "Failed to accept changes for pipeline: %s.\n"
	fmtPipelineUpdateProposalComplete = "Successfully updated pipeline: %s\n"

	fmtPipelineUpdateExistPrompt = "Are you sure you want to update an existing pipeline: %s?"
)

type updatePipelineVars struct {
	appName          string
	skipConfirmation bool
}

type updatePipelineOpts struct {
	updatePipelineVars

	pipelineDeployer pipelineDeployer
	app              *config.Application
	prog             progress
	prompt           prompter
	region           string
	envStore         environmentStore
	ws               wsPipelineReader

	pipelineName string
}

func newUpdatePipelineOpts(vars updatePipelineVars) (*updatePipelineOpts, error) {
	store, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("new config store client: %w", err)
	}

	app, err := store.GetApplication(vars.appName)
	if err != nil {
		return nil, fmt.Errorf("get application %s: %w", vars.appName, err)
	}

	defaultSession, err := sessions.NewProvider().Default()
	if err != nil {
		return nil, fmt.Errorf("default session: %w", err)
	}

	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("new workspace client: %w", err)
	}

	return &updatePipelineOpts{
		app:                app,
		pipelineDeployer:   deploycfn.New(defaultSession),
		region:             aws.StringValue(defaultSession.Config.Region),
		updatePipelineVars: vars,
		envStore:           store,
		ws:                 ws,
		prog:               termprogress.NewSpinner(log.DiagnosticWriter),
		prompt:             prompt.New(),
	}, nil
}

// Validate returns an error if the flag values passed by the user are invalid.
func (o *updatePipelineOpts) Validate() error {
	return nil
}

func (o *updatePipelineOpts) convertStages(manifestStages []manifest.PipelineStage) ([]deploy.PipelineStage, error) {
	var stages []deploy.PipelineStage
	workloads, err := o.ws.WorkloadNames()
	if err != nil {
		return nil, fmt.Errorf("get workload names from workspace: %w", err)
	}

	for _, stage := range manifestStages {
		env, err := o.envStore.GetEnvironment(o.appName, stage.Name)
		if err != nil {
			return nil, fmt.Errorf("get environment %s in application %s: %w", stage.Name, o.appName, err)
		}

		pipelineStage := deploy.PipelineStage{
			LocalWorkloads: workloads,
			AssociatedEnvironment: &deploy.AssociatedEnvironment{
				Name:      stage.Name,
				Region:    env.Region,
				AccountID: env.AccountID,
			},
			RequiresApproval: stage.RequiresApproval,
			TestCommands:     stage.TestCommands,
		}
		stages = append(stages, pipelineStage)
	}

	return stages, nil
}

func (o *updatePipelineOpts) getArtifactBuckets() ([]deploy.ArtifactBucket, error) {
	regionalResources, err := o.pipelineDeployer.GetRegionalAppResources(o.app)
	if err != nil {
		return nil, err
	}

	var buckets []deploy.ArtifactBucket
	for _, resource := range regionalResources {
		bucket := deploy.ArtifactBucket{
			BucketName: resource.S3Bucket,
			KeyArn:     resource.KMSKeyARN,
		}
		buckets = append(buckets, bucket)
	}

	return buckets, nil
}

func (o *updatePipelineOpts) shouldUpdate() (bool, error) {
	if o.skipConfirmation {
		return true, nil
	}

	shouldUpdate, err := o.prompt.Confirm(fmt.Sprintf(fmtPipelineUpdateExistPrompt, o.pipelineName), "")
	if err != nil {
		return false, fmt.Errorf("prompt for pipeline update: %w", err)
	}
	return shouldUpdate, nil
}

func (o *updatePipelineOpts) deployPipeline(in *deploy.CreatePipelineInput) error {
	exist, err := o.pipelineDeployer.PipelineExists(in)
	if err != nil {
		return fmt.Errorf("check if pipeline exists: %w", err)
	}
	if !exist {
		o.prog.Start(fmt.Sprintf(fmtPipelineUpdateStart, color.HighlightUserInput(o.pipelineName)))
		if err := o.pipelineDeployer.CreatePipeline(in); err != nil {
			var alreadyExists *cloudformation.ErrStackAlreadyExists
			if !errors.As(err, &alreadyExists) {
				o.prog.Stop(log.Serrorf(fmtPipelineUpdateFailed, color.HighlightUserInput(o.pipelineName)))
				return fmt.Errorf("create pipeline: %w", err)
			}
		}
		o.prog.Stop(log.Ssuccessf(fmtPipelineUpdateComplete, color.HighlightUserInput(o.pipelineName)))
		return nil
	}

	// If the stack already exists - we update it
	shouldUpdate, err := o.shouldUpdate()
	if err != nil {
		return err
	}
	if !shouldUpdate {
		return nil
	}
	o.prog.Start(fmt.Sprintf(fmtPipelineUpdateProposalStart, color.HighlightUserInput(o.pipelineName)))
	if err := o.pipelineDeployer.UpdatePipeline(in); err != nil {
		o.prog.Stop(log.Serrorf(fmtPipelineUpdateProposalFailed, color.HighlightUserInput(o.pipelineName)))
		return fmt.Errorf("update pipeline: %w", err)
	}
	o.prog.Stop(log.Ssuccessf(fmtPipelineUpdateProposalComplete, color.HighlightUserInput(o.pipelineName)))
	return nil
}

// Execute create a new pipeline or update the current pipeline if it already exists.
func (o *updatePipelineOpts) Execute() error {
	// bootstrap pipeline resources
	o.prog.Start(fmt.Sprintf(fmtPipelineUpdateResourcesStart, color.HighlightUserInput(o.appName)))
	err := o.pipelineDeployer.AddPipelineResourcesToApp(o.app, o.region)
	if err != nil {
		o.prog.Stop(log.Serrorf(fmtPipelineUpdateResourcesFailed, color.HighlightUserInput(o.appName)))
		return fmt.Errorf("add pipeline resources to application %s in %s: %w", o.appName, o.region, err)
	}
	o.prog.Stop(log.Ssuccessf(fmtPipelineUpdateResourcesComplete, color.HighlightUserInput(o.appName)))

	// read pipeline manifest
	data, err := o.ws.ReadPipelineManifest()
	if err != nil {
		return fmt.Errorf("read pipeline manifest: %w", err)
	}
	pipeline, err := manifest.UnmarshalPipeline(data)
	if err != nil {
		return fmt.Errorf("unmarshal pipeline manifest: %w", err)
	}
	if len(pipeline.Name) > 100 {
		return fmt.Errorf(`pipeline name '%s' must be shorter than 100 characters`, pipeline.Name)
	}
	o.pipelineName = pipeline.Name

	var source interface{}
	switch pipeline.Source.ProviderName {
	case ghProviderName:
		source = &deploy.GitHubSource{
			ProviderName:                ghProviderName,
			Branch:                      (pipeline.Source.Properties["branch"]).(string),
			RepositoryURL:               (pipeline.Source.Properties["repository"]).(string),
			PersonalAccessTokenSecretID: (pipeline.Source.Properties["access_token_secret"]).(string),
		}
	case ccProviderName:
		source = &deploy.CodeCommitSource{
			ProviderName:  ccProviderName,
			Branch:        (pipeline.Source.Properties["branch"]).(string),
			RepositoryURL: (pipeline.Source.Properties["repository"]).(string),
		}
	default:
		return fmt.Errorf("invalid repo source provider: %s", pipeline.Source.ProviderName)
	}

	// convert environments to deployment stages
	stages, err := o.convertStages(pipeline.Stages)
	if err != nil {
		return fmt.Errorf("convert environments to deployment stage: %w", err)
	}

	// get cross-regional resources
	artifactBuckets, err := o.getArtifactBuckets()
	if err != nil {
		return fmt.Errorf("get cross-regional resources: %w", err)
	}

	deployPipelineInput := &deploy.CreatePipelineInput{
		AppName:         o.appName,
		Name:            pipeline.Name,
		Source:          source,
		Stages:          stages,
		ArtifactBuckets: artifactBuckets,
		AdditionalTags:  o.app.Tags,
	}

	if err := o.deployPipeline(deployPipelineInput); err != nil {
		return err
	}

	return nil
}

// BuildPipelineUpdateCmd build the command for deploying a new pipeline or updating an existing pipeline.
func buildPipelineUpdateCmd() *cobra.Command {
	vars := updatePipelineVars{}
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Deploys a pipeline for the services in your workspace.",
		Long:  `Deploys a pipeline for the services in your workspace, using the environments associated with the application.`,
		Example: `
  Deploys an updated pipeline for the services in your workspace.
  /code $ copilot pipeline update`,
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			opts, err := newUpdatePipelineOpts(vars)
			if err != nil {
				return err
			}
			if err := opts.Validate(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	cmd.Flags().StringVarP(&vars.appName, appFlag, appFlagShort, tryReadingAppName(), appFlagDescription)
	cmd.Flags().BoolVar(&vars.skipConfirmation, yesFlag, false, yesFlagDescription)
	return cmd
}

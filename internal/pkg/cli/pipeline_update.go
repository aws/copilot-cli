// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"

	"github.com/aws/aws-sdk-go/aws"

	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/spf13/cobra"
)

const (
	fmtAddPipelineResourcesFailed   = "Failed to add pipeline resources to your project: %s"
	fmtAddPipelineResourcesStart    = "Adding pipeline resources to your project: %s"
	fmtAddPipelineResourcesComplete = "Successfully added pipeline resources to your project: %s"

	fmtCreatePipelineFailed   = "Failed to create a new pipeline: %s."
	fmtCreatePipelineStart    = "Creating a new pipeline: %s"
	fmtCreatePipelineComplete = "Successfully created a new pipeline: %s"

	fmtUpdatePipelineFailed   = "Failed to accept changes for pipeline: %s."
	fmtUpdatePipelineStart    = "Proposing infrastructure changes for the pipeline: %s"
	fmtUpdatePipelineComplete = "Successfully updated pipeline: %s"

	fmtUpdateEnvPrompt = "Are you sure you want to update an existing pipeline: %s?"
)

var errNoPipelineFile = errors.New("there was no pipeline manifest found in your workspace. Please run `ecs-preview pipeline init` to create an pipeline")

type updatePipelineOpts struct {
	PipelineFile     string
	PipelineName     string
	SkipConfirmation bool

	pipelineDeployer pipelineDeployer
	project          *archer.Project
	prog             progress
	region           string
	envStore         archer.EnvironmentStore
	ws               archer.Workspace

	*GlobalOpts
}

func newUpdatePipelineOpts() *updatePipelineOpts {
	return &updatePipelineOpts{
		GlobalOpts: NewGlobalOpts(),
		prog:       termprogress.NewSpinner(),
	}
}

// Validate returns an error if the flag values passed by the user are invalid.
func (opts *updatePipelineOpts) Validate() error {
	if opts.PipelineFile == "" {
		return errNoPipelineFile
	}

	return nil
}

func (opts *updatePipelineOpts) convertStages(manifestStages []manifest.PipelineStage) ([]deploy.PipelineStage, error) {
	var stages []deploy.PipelineStage
	apps, err := opts.ws.Apps()
	if err != nil {
		return nil, err
	}
	// TODO: Will fast follow with another PR to actually support #443
	appNames := make([]string, 0, len(apps))
	for _, app := range apps {
		appNames = append(appNames, app.AppName())
	}

	for _, stage := range manifestStages {
		env, err := opts.envStore.GetEnvironment(opts.ProjectName(), stage.Name)
		if err != nil {
			return nil, err
		}

		pipelineStage := deploy.PipelineStage{
			LocalApplications: appNames,
			AssociatedEnvironment: &deploy.AssociatedEnvironment{
				Name:      stage.Name,
				Region:    env.Region,
				AccountID: env.AccountID,
				Prod:      env.Prod,
			},
		}
		stages = append(stages, pipelineStage)
	}

	return stages, nil
}

func (opts *updatePipelineOpts) getArtifactBuckets() ([]deploy.ArtifactBucket, error) {
	regionalResources, err := opts.pipelineDeployer.GetRegionalProjectResources(opts.project)
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

func (opts *updatePipelineOpts) shouldUpdate() (bool, error) {
	if opts.SkipConfirmation {
		return true, nil
	}

	shouldUpdate, err := opts.prompt.Confirm(fmt.Sprintf(fmtUpdateEnvPrompt, opts.PipelineName), "")
	if err != nil {
		return false, fmt.Errorf("prompt for pipeline update: %w", err)
	}
	return shouldUpdate, nil
}

func (opts *updatePipelineOpts) deployPipeline(in *deploy.CreatePipelineInput) error {
	exist, err := opts.pipelineDeployer.PipelineExists(in)
	if err != nil {
		return fmt.Errorf("check if pipeline exists: %w", err)
	}
	if !exist {
		opts.prog.Start(fmt.Sprintf(fmtCreatePipelineStart, color.HighlightUserInput(opts.PipelineName)))
		if err := opts.pipelineDeployer.CreatePipeline(in); err != nil {
			var alreadyExists *cloudformation.ErrStackAlreadyExists
			if !errors.As(err, &alreadyExists) {
				opts.prog.Stop(log.Serrorf(fmtCreatePipelineFailed, color.HighlightUserInput(opts.PipelineName)))
				return fmt.Errorf("create pipeline: %w", err)
			}
		}
		opts.prog.Stop(log.Ssuccessf(fmtCreatePipelineComplete, color.HighlightUserInput(opts.PipelineName)))
		return nil
	}

	// If the stack already exists - we update it
	shouldUpdate, err := opts.shouldUpdate()
	if err != nil {
		return err
	}
	if !shouldUpdate {
		return nil
	}
	opts.prog.Start(fmt.Sprintf(fmtUpdatePipelineStart, color.HighlightUserInput(opts.PipelineName)))
	if err := opts.pipelineDeployer.UpdatePipeline(in); err != nil {
		opts.prog.Stop(log.Serrorf(fmtUpdatePipelineFailed, color.HighlightUserInput(opts.PipelineName)))
		return fmt.Errorf("update pipeline: %w", err)
	}
	opts.prog.Stop(log.Ssuccessf(fmtUpdatePipelineComplete, color.HighlightUserInput(opts.PipelineName)))
	return nil
}

// Execute create a new pipeline or update the current pipeline if it already exists.
func (opts *updatePipelineOpts) Execute() error {
	// bootstrap pipeline resources
	opts.prog.Start(fmt.Sprintf(fmtAddPipelineResourcesStart, color.HighlightUserInput(opts.ProjectName())))
	err := opts.pipelineDeployer.AddPipelineResourcesToProject(opts.project, opts.region)
	if err != nil {
		opts.prog.Stop(log.Serrorf(fmtAddPipelineResourcesFailed, color.HighlightUserInput(opts.ProjectName())))
		return fmt.Errorf("add pipeline resources to project %s in %s: %w", opts.ProjectName(), opts.region, err)
	}
	opts.prog.Stop(log.Ssuccessf(fmtAddPipelineResourcesComplete, color.HighlightUserInput(opts.ProjectName())))

	// read pipeline manifest
	data, err := opts.ws.ReadFile(workspace.PipelineFileName)
	if err != nil {
		return fmt.Errorf("read pipeline file %s: %w", workspace.PipelineFileName, err)
	}
	pipeline, err := manifest.UnmarshalPipeline(data)
	if err != nil {
		return fmt.Errorf("unmarshal pipeline file %s: %w", workspace.PipelineFileName, err)
	}
	opts.PipelineName = pipeline.Name
	source := &deploy.Source{
		ProviderName: pipeline.Source.ProviderName,
		Properties:   pipeline.Source.Properties,
	}

	// convert environments to deployment stages
	stages, err := opts.convertStages(pipeline.Stages)
	if err != nil {
		return fmt.Errorf("convert environments to deployment stage: %w", err)
	}

	// get cross-regional resources
	artifactBuckets, err := opts.getArtifactBuckets()
	if err != nil {
		return fmt.Errorf("get cross-regional resources: %w", err)
	}

	deployPipelineInput := &deploy.CreatePipelineInput{
		ProjectName:     opts.ProjectName(),
		Name:            pipeline.Name,
		Source:          source,
		Stages:          stages,
		ArtifactBuckets: artifactBuckets,
	}

	if err := opts.deployPipeline(deployPipelineInput); err != nil {
		return err
	}

	return nil
}

// BuildPipelineUpdateCmd build the command for deploying a new pipeline or updating an existing pipeline.
func BuildPipelineUpdateCmd() *cobra.Command {
	opts := newUpdatePipelineOpts()
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Deploys a pipeline for applications in your workspace.",
		Long:  `Deploys a pipeline for the applications in your workspace, using the environments associated with the applications.`,
		Example: `
  Deploy an updated pipeline for the applications in your workspace:
  /code $ ecs-preview pipeline update`,

		PreRunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			store, err := store.New()
			if err != nil {
				return fmt.Errorf("couldn't connect to project datastore: %w", err)
			}
			opts.envStore = store

			project, err := store.GetProject(opts.ProjectName())
			if err != nil {
				return fmt.Errorf("get project %s: %w", opts.ProjectName(), err)
			}
			opts.project = project

			p := session.NewProvider()
			defaultSession, err := p.Default()
			if err != nil {
				return err
			}
			opts.pipelineDeployer = cloudformation.New(defaultSession)

			region := aws.StringValue(defaultSession.Config.Region)
			opts.region = region

			ws, err := workspace.New()
			if err != nil {
				return err
			}
			opts.ws = ws
			return nil
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			if err := opts.Validate(); err != nil {
				return err
			}
			return opts.Execute()
		}),
	}
	cmd.Flags().StringVarP(&opts.PipelineFile, pipelineFileFlag, pipelineFileFlagShort, workspace.PipelineFileName, pipelineFileFlagDescription)
	cmd.Flags().BoolVar(&opts.SkipConfirmation, yesFlag, false, yesFlagDescription)

	return cmd
}

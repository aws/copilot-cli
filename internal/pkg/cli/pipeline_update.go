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
	fmtAddPipelineResourcesStart    = "Adding pipeline resources to your project: %s"
	fmtAddPipelineResourcesComplete = "Successfully added pipeline resources to your project: %s"

	fmtUpdatePipelineFailed   = "Failed to accept changes for pipeline: %s."
	fmtUpdatePipelineStart    = "Proposing infrastructure changes for the pipeline: %s"
	fmtUpdatePipelineComplete = "Successfully updated pipeline: %s"
)

var errNoPipelineFile = errors.New("There was no pipeline manifest found in your workspace. Please run `archer pipeline init` to create an pipeline.")

// UpdatePipelineOpts holds the configuration needed to create or update a pipeline
type UpdatePipelineOpts struct {
	PipelineFile string
	// Deploy bool

	pipelineDeployer pipelineDeployer
	project          *archer.Project
	prog             progress
	region           string
	envStore         archer.EnvironmentStore
	ws               archer.Workspace

	*GlobalOpts
}

func NewUpdatePipelineOpts() *UpdatePipelineOpts {
	return &UpdatePipelineOpts{
		GlobalOpts: NewGlobalOpts(),
		prog:       termprogress.NewSpinner(),
	}
}

// Validate returns an error if the flag values passed by the user are invalid.
func (opts *UpdatePipelineOpts) Validate() error {
	if opts.PipelineFile == "" {
		return errNoPipelineFile
	}

	return nil
}

func (opts *UpdatePipelineOpts) convertStages(manifestStages []manifest.PipelineStage) ([]deploy.PipelineStage, error) {
	var stages []deploy.PipelineStage
	apps, err := opts.ws.AppNames()
	if err != nil {
		return nil, err
	}

	for _, stage := range manifestStages {
		env, err := opts.envStore.GetEnvironment(opts.ProjectName(), stage.Name)
		if err != nil {
			return nil, err
		}

		pipelineStage := deploy.PipelineStage{
			LocalApplications: apps,
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

func (opts *UpdatePipelineOpts) getArtifactBuckets() ([]deploy.ArtifactBucket, error) {
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

func (opts *UpdatePipelineOpts) Execute() error {
	// bootstrap pipeline resources
	opts.prog.Start(fmt.Sprintf(fmtAddPipelineResourcesStart, color.HighlightUserInput(opts.ProjectName())))
	err := opts.pipelineDeployer.AddPipelineResourcesToProject(opts.project, opts.region)
	if err != nil {
		return nil
	}
	opts.prog.Stop(log.Ssuccessf(fmtAddPipelineResourcesComplete, color.HighlightUserInput(opts.ProjectName())))

	// read pipeline manifest
	data, err := opts.ws.ReadFile(workspace.PipelineFileName)
	if err != nil {
		return nil
	}
	pipeline, err := manifest.UnmarshalPipeline(data)
	// TODO nil check source?
	source := &deploy.Source{
		ProviderName: pipeline.Source.ProviderName,
		Properties:   pipeline.Source.Properties,
	}

	// convert environments to deployment stages
	stages, err := opts.convertStages(pipeline.Stages)
	if err != nil {
		return nil
	}

	// get cross-regional resources
	artifactBuckets, err := opts.getArtifactBuckets()
	if err != nil {
		return nil
	}

	deployPipelineInput := &deploy.CreatePipelineInput{
		ProjectName:     opts.ProjectName(),
		Name:            pipeline.Name,
		Source:          source,
		Stages:          stages,
		ArtifactBuckets: artifactBuckets,
	}

	// deploy pipeline
	opts.prog.Start(fmt.Sprintf(fmtUpdatePipelineStart, color.HighlightUserInput(opts.PipelineFile)))

	if err := opts.pipelineDeployer.DeployPipeline(deployPipelineInput); err != nil {
		// TODO if pipeline already exists, still update?
		opts.prog.Stop(log.Serrorf(fmtUpdatePipelineFailed, color.HighlightUserInput(opts.PipelineFile)))
		return fmt.Errorf("deploy pipeline: %w", err)
	}
	// TODO stream events

	opts.prog.Stop(log.Ssuccessf(fmtUpdatePipelineComplete, color.HighlightUserInput(opts.PipelineFile))) // change to pipeline name?

	return nil
}

// BuildPipelineUpdateCmd build the command for deploying a new pipeline or updating an existing pipeline.
func BuildPipelineUpdateCmd() *cobra.Command {
	opts := NewUpdatePipelineOpts()
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Deploys a pipeline for applications in your workspace.",
		Long:  `Deploys a pipeline for the applications in your workspace, using the environments associated with the applications.`,
		Example: `
  Deploy an updated pipeline for the applications in your workspace:
  /code $ archer pipeline update`,

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

			defaultSession, err := session.Default()
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

			return opts.Validate()
		}),
		RunE: runCmdE(func(cmd *cobra.Command, args []string) error {
			return opts.Execute()
		}),
	}
	cmd.Flags().StringVarP(&opts.PipelineFile, pipelineFileFlag, pipelineFileFlagShort, workspace.PipelineFileName, pipelineFileFlagDescription)
	// cmd.Flags().BoolVar(&opts.Deploy, deployFlag, false, deployFlagDescription)

	return cmd
}

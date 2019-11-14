// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"

	"github.com/aws/aws-sdk-go/aws"

	termprogress "github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/progress"
	"github.com/spf13/cobra"
)

const (
	defaultPipelineFilename = "pipeline.yml"

	fmtUpdatePipelineFailed   = "Failed to accept changes for pipeline: %s."
	fmtUpdatePipelineStart    = "Proposing infrastructure changes for the pipeline: %s"
	fmtUpdatePipelineComplete = "Successfully updated pipeline: %s"
)

var errNoPipelineFile = errors.New("There was no pipeline manifest found in your workspace. Please run `archer pipeline init` to create an pipeline.")

// UpdatePipelineOpts holds the configuration needed to update an existing pipeilne
type UpdatePipelineOpts struct {
	// Fields with matching flags.
	PipelineFile string
	// Deploy bool

	pipelineDeployer pipelineDeployer
	project          *archer.Project
	prog             progress
	region           string
	account          string
	ws               archer.Workspace

	*GlobalOpts
}

func NewUpdatePipelineOpts() *UpdatePipelineOpts {
	return &UpdatePipelineOpts{
		GlobalOpts: NewGlobalOpts(),
		prog:       termprogress.NewSpinner(),
	}
}

// func (opts *UpdatePipelineOpts) Ask() error {
// 	if opts.PipelineFile() == "" {
// 		return errNoPipelineFile
// 	}

// 	return nil
// }

// Validate returns an error if the flag values passed by the user are invalid.
func (opts *UpdatePipelineOpts) Validate() error {
	if opts.PipelineFile == "" {
		return errNoPipelineFile
	}

	return nil
}

func (opts *UpdatePipelineOpts) convertStages(manifestStages []manifest.PipelineStage) ([]deploy.PipelineStage, error) {
	stages := []deploy.PipelineStage{}
	apps, err := opts.ws.AppNames()
	if err != nil {
		return nil, err
	}

	for _, mstage := range manifestStages {
		dstage := deploy.PipelineStage{
			LocalApplications: apps,
			AssociatedEnvironment: &deploy.AssociatedEnvironment{
				Name: mstage.Name,
				Region: opts.region, // FIXME
				AccountID: opts.account, // FIXME
				Prod: false,
			},
		}
		stages = append(stages, dstage)
	}

	return stages, nil
}

func (opts *UpdatePipelineOpts) getArtifactBuckets() ([]deploy.ArtifactBucket, error) {
	regionalResources, err := opts.pipelineDeployer.GetRegionalProjectResources(opts.project)
	if err != nil {
		return nil, err
	}

	buckets := []deploy.ArtifactBucket{}
	for _, resource := range regionalResources {
		bucket := deploy.ArtifactBucket{
			BucketName: resource.S3Bucket,
			KeyArn: resource.KMSKeyARN,
		}
		buckets = append(buckets, bucket)
	}

	return buckets, nil
}

func (opts *UpdatePipelineOpts) Execute() error {

	// read pipeline manifest
	data, err := opts.ws.ReadFile(defaultPipelineFilename)
	if err != nil {
		return nil
	}

	pipeline, err := manifest.UnmarshalPipeline(data)
	// convert source TODO combine?
	// TODO nil check source?
	source := &deploy.Source{
		ProviderName: pipeline.Source.ProviderName,
		Properties: pipeline.Source.Properties,
	}
	// convert stages to deployment stages
	stages, err := opts.convertStages(pipeline.Stages)
	if err != nil {
		return nil
	}
	// get cross-regional resources
	artifacts, err := opts.getArtifactBuckets()
	if err != nil {
		return nil
	}

	deployPipelineInput := &deploy.CreatePipelineInput{
		ProjectName: opts.ProjectName(),
		Name:        pipeline.Name,
		Source: source,
		Stages: stages,
		ArtifactBuckets: artifacts,
	}

	// convert it to cloudfomration

	// deploy it via CFN client
	opts.prog.Start(fmt.Sprintf(fmtUpdatePipelineStart, color.HighlightUserInput(opts.PipelineFile)))

	if err := opts.pipelineDeployer.DeployPipeline(deployPipelineInput); err != nil {
		// check if pipeline already exists?
		opts.prog.Stop(log.Serrorf(fmtUpdatePipelineFailed, color.HighlightUserInput(opts.PipelineFile)))
		return fmt.Errorf("deploy pipeline: %w", err)
	}
	// TODO stream events

	opts.prog.Stop(log.Ssuccessf(fmtUpdatePipelineComplete, color.HighlightUserInput(opts.PipelineFile))) // change to pipeline name?

	return nil
}

// BuildPipelineUpdateCmd build the command for creating a new pipeline.
func BuildPipelineUpdateCmd() *cobra.Command {
	opts := NewUpdatePipelineOpts()
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Deploys a pipeline for applications in your workspace.",
		Long:  `Deploys a pipeline for the applications in your workspace, using the environments associated with the applications.`,
		Example: `
  Deploy an updated pipeline for the applications in your workspace:
  /code $ archer pipeline update`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			store, err := store.New()
			if err != nil {
				return fmt.Errorf("couldn't connect to project datastore: %w", err)
			}
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

			err = opts.pipelineDeployer.AddPipelineResourcesToProject(project, region)
			if err != nil {
				return nil
			}

			identityService := identity.New(defaultSession)
			caller, err := identityService.Get()
			if err != nil {
				return nil
			}
			account := caller.Account
			opts.account = account
			opts.region = region

			ws, err := workspace.New()
			if err != nil {
				return err
			}
			opts.ws = ws

			return opts.Validate()
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			// if err := opts.Ask(); err != nil {
			// 	return err
			// }
			// if err := opts.Validate(); err != nil {
			// 	return err
			// }
			return opts.Execute()
		},
	}
	// TODO defaultPipelineFilename -- need to store that somewhere on pipeline init?
	cmd.Flags().StringVarP(&opts.PipelineFile, pipelineFileFlag, pipelineFileFlagShort, defaultPipelineFilename, pipelineFileFlagDescription)
	// cmd.Flags().BoolVar(&opts.Deploy, deployFlag, false, deployFlagDescription)

	return cmd
}

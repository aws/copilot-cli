// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
)

type pipeline struct {
	// Name of the project this pipeline belongs to
	ProjectName string
	// Name of the pipeline
	Name   string
	Source *source
	Stages []pipelineStage
}

// source defines the source of the artifacts to be built and deployed.
type source struct {
	ProviderName string
	Properties   map[string]interface{}
}

// pipelineStage represents configuration for each deployment stage
// of a workspace. A stage consists of the Archer Environment the pipeline
// is deloying to and the containerized applications that will be deployed.
type pipelineStage struct {
	*associatedEnvironment
	LocalApplications []string
}

// associatedEnvironment defines the necessary information a pipline stage
// needs for an Archer Environment.
type associatedEnvironment struct {
	// Name of the environment, must be unique within a project.
	// This is also the name of the pipeline stage.
	Name string
	// T region this environment is stored in.
	Region string
	// AccountID of the account this environment is stored in.
	AccountID string
	// Whether or not this environment is a production environment.
	Prod bool
}

func newPipelineStackConfig(
	envG archer.EnvironmentGetter, ws archer.Workspace,
	m *manifest.PipelineManifest) (*pipeline, error) {

	summary, err := ws.Summary()
	if err != nil {
		return nil, err
	}

	apps, err := ws.AppNames()
	if err != nil {
		return nil, err
	}

	stages := make([]pipelineStage, 0, len(m.Stages))
	for _, stage := range m.Stages {
		env, err := envG.GetEnvironment(summary.ProjectName, stage.Name)
		if err != nil {
			return nil, err
		}
		stages = append(stages, pipelineStage{
			associatedEnvironment: &associatedEnvironment{
				Name:      stage.Name,
				Region:    env.Region,
				AccountID: env.AccountID,
				Prod:      env.Prod,
			},
			LocalApplications: apps,
		})
	}

	return &pipeline{
		ProjectName: summary.ProjectName,
		Name:        m.Name,
		Source: &source{
			ProviderName: m.Source.ProviderName,
			Properties:   m.Source.Properties,
		},
		Stages: stages,
	}, nil
}

func (p *pipeline) StackName() string {
	return p.ProjectName + "-" + p.Name
}

func (p *pipeline) Template() (string, error) {
	// TODO: Render the template
	return "", nil
}

func (p *pipeline) Parameters() []*cloudformation.Parameter {
	return nil
}

func (p *pipeline) Tags() []*cloudformation.Tag {
	return []*cloudformation.Tag{
		{
			Key:   aws.String(projectTagKey),
			Value: aws.String(p.ProjectName),
		},
	}
}

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
)

// Interfaces for deploying resources through CloudFormation. Facilitates mocking.
type environmentDeployer interface {
	DeployEnvironment(env *deploy.CreateEnvironmentInput) error
	StreamEnvironmentCreation(env *deploy.CreateEnvironmentInput) (<-chan []deploy.ResourceEvent, <-chan deploy.CreateEnvironmentResponse)
	DeleteEnvironment(projName, envName string) error
}

type pipelineDeployer interface {
	CreatePipeline(env *deploy.CreatePipelineInput) error
	UpdatePipeline(env *deploy.CreatePipelineInput) error
	PipelineExists(env *deploy.CreatePipelineInput) (bool, error)
	DeletePipeline(pipelineName string) error
	AddPipelineResourcesToProject(project *archer.Project, region string) error
	projectResourcesGetter
	// TODO: Add StreamPipelineCreation method
}

type projectDeployer interface {
	DeployProject(in *deploy.CreateProjectInput) error
	AddAppToProject(project *archer.Project, appName string) error
	AddEnvToProject(project *archer.Project, env *archer.Environment) error
	DelegateDNSPermissions(project *archer.Project, accountID string) error
	DeleteProject(name string) error
}

type projectResourcesGetter interface {
	GetProjectResourcesByRegion(project *archer.Project, region string) (*archer.ProjectRegionalResources, error)
	GetRegionalProjectResources(project *archer.Project) ([]*archer.ProjectRegionalResources, error)
}

type deployer interface {
	environmentDeployer
	projectDeployer
	pipelineDeployer
}

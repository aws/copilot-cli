// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy applications and environments.
package deploy

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
)

// CreateEnvironmentInput represents the fields required to deploy an environment.
type CreateEnvironmentInput struct {
	Project                  string // Name of the project this environment belongs to.
	Name                     string // Name of the environment, must be unique within a project.
	Prod                     bool   // Whether or not this environment is a production environment.
	PublicLoadBalancer       bool   // Whether or not this environment should contain a shared public load balancer between applications.
	ToolsAccountPrincipalARN string // The Principal ARN of the tools account.
}

// CreatePipelineInput represents the fields required to deploy a pipeline.
type CreatePipelineInput struct {
	// Name of the project this pipeline belongs to
	ProjectName string
	// Name of the pipeline
	Name   string
	Source *Source
	Stages []PipelineStage
}

// Source defines the source of the artifacts to be built and deployed.
type Source struct {
	ProviderName string
	Properties   map[string]interface{}
}

// PipelineStage represents configuration for each deployment stage
// of a workspace. A stage consists of the Archer Environment the pipeline
// is deloying to and the containerized applications that will be deployed.
type PipelineStage struct {
	*AssociatedEnvironment
	LocalApplications []string
}

// AssociatedEnvironment defines the necessary information a pipline stage
// needs for an Archer Environment.
type AssociatedEnvironment struct {
	// Name of the environment, must be unique within a project.
	// This is also the name of the pipeline stage.
	Name string
	// T region this environment is stored in.
	Region string
	// AccountId of the account this environment is stored in.
	AccountId string
	// Whether or not this environment is a production environment.
	Prod bool
}

// CreateLBFargateAppInput holds the fields required to deploy a load-balanced AWS Fargate application.
type CreateLBFargateAppInput struct {
	App      *manifest.LBFargateManifest
	Env      *archer.Environment
	ImageTag string
}

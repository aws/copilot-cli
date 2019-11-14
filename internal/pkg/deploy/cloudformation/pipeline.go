// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy archer resources with AWS CloudFormation.
// This file defines API for deploying a pipeline.
package cloudformation

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
)

// DeployPipeline sets up everything required for a pipeline with cross-region stages.
// These resources include things that are regional, rather than scoped to a particular
// environment, such as ECR Repos, CodePipeline KMS keys & S3 buckets.
// We deploy pipeline resources through StackSets - that way we can have one
// template that we update and all regional stacks are updated.
func (cf CloudFormation) DeployPipeline(in *deploy.CreatePipelineInput) error {
	pipelineConfig := stack.NewPipelineStackConfig(in)

	return cf.deploy(pipelineConfig)
}

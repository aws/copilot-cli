// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy archer resources with AWS CloudFormation.
// This file defines API for deploying a pipeline.
package cloudformation

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
)

// DeployPipeline sets up a CodePipeline for deploying applications.
// Project-level regional resources (such as KMS keys for de/encrypting &
// S3 buckets for storing pipeline artifacts) should be provisioned using
// `AddPipelineResourcesToProject()` before calling this function.
func (cf CloudFormation) DeployPipeline(in *deploy.CreatePipelineInput) error {
	pipelineConfig := stack.NewPipelineStackConfig(in)

	return cf.deploy(pipelineConfig)
}

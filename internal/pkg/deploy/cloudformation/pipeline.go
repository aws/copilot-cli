// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy archer resources with AWS CloudFormation.
// This file defines API for deploying a pipeline.
package cloudformation

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// DeployPipeline sets up a CodePipeline for deploying applications.
// Project-level regional resources (such as KMS keys for de/encrypting &
// S3 buckets for storing pipeline artifacts) should be provisioned using
// `AddPipelineResourcesToProject()` before calling this function.
func (cf CloudFormation) DeployPipeline(in *deploy.CreatePipelineInput) error {
	pipelineConfig := stack.NewPipelineStackConfig(in)

	// First attempt to create the pipeline stack
	err := cf.create(pipelineConfig)
	if err == nil {
		_, err := cf.waitForStackCreation(pipelineConfig)
		if err != nil {
			return err
		}
	}

	// If the stack already exists - we update it
	var alreadyExists *ErrStackAlreadyExists
	if !errors.As(err, &alreadyExists) {
		return err
	}

	if err := cf.update(pipelineConfig); err != nil {
		return fmt.Errorf("updating pipeline: %w", err)
	}

	return cf.client.WaitUntilStackUpdateCompleteWithContext(context.Background(),
		&cloudformation.DescribeStacksInput{
			StackName: aws.String(pipelineConfig.StackName()),
		}, cf.waiters...)
}

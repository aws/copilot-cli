// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy ECS resources with AWS CloudFormation.
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

// PipelineExists checks if the pipeline with the provided config exists.
func (cf CloudFormation) PipelineExists(in *deploy.CreatePipelineInput) (bool, error) {
	stackConfig := stack.NewPipelineStackConfig(in)
	// TODO refactor to use cf.describeStack?
	describeStackInput := &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackConfig.StackName()),
	}
	_, err := cf.describeStack(describeStackInput)
	if err != nil {
		var stackNotFound *ErrStackNotFound
		if !errors.As(err, &stackNotFound) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

// CreatePipeline sets up a new CodePipeline for deploying applications.
func (cf CloudFormation) CreatePipeline(in *deploy.CreatePipelineInput) error {
	pipelineConfig := stack.NewPipelineStackConfig(in)

	// First attempt to create the pipeline stack
	err := cf.create(pipelineConfig)
	if err == nil {
		_, err := cf.waitForStackCreation(pipelineConfig)
		if err != nil {
			return err
		}
		return nil
	}
	return err
}

// UpdatePipeline updates an existing CodePipeline for deploying applications.
func (cf CloudFormation) UpdatePipeline(in *deploy.CreatePipelineInput) error {
	pipelineConfig := stack.NewPipelineStackConfig(in)
	if err := cf.update(pipelineConfig); err != nil {
		if err == errChangeSetEmpty {
			// If there are no updates, then exit successfully.
			return nil
		}
		return fmt.Errorf("updating pipeline: %w", err)
	}

	return cf.client.WaitUntilStackUpdateCompleteWithContext(context.Background(),
		&cloudformation.DescribeStacksInput{
			StackName: aws.String(pipelineConfig.StackName()),
		}, cf.waiters...)
}

func (cf CloudFormation) DeletePipeline(stackName string) error {
	// Check if the stack exists
	out, err := cf.describeStack(&cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return err
	}

	return cf.delete(*out.StackId)
}

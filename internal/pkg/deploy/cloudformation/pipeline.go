// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy ECS resources with AWS CloudFormation.
// This file defines API for deploying a pipeline.
package cloudformation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
)

const (
	sourceStage      = "Source"
	connectionARNKey = "PipelineConnectionARN"
)

// PipelineExists checks if the pipeline with the provided config exists.
func (cf CloudFormation) PipelineExists(in *deploy.CreatePipelineInput) (bool, error) {
	stackConfig := stack.NewPipelineStackConfig(in)
	_, err := cf.cfnClient.Describe(stackConfig.StackName())
	if err != nil {
		var stackNotFound *cloudformation.ErrStackNotFound
		if !errors.As(err, &stackNotFound) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

// CreatePipeline sets up a new CodePipeline for deploying services.
func (cf CloudFormation) CreatePipeline(in *deploy.CreatePipelineInput) error {
	s, err := toStack(stack.NewPipelineStackConfig(in))
	if err != nil {
		return err
	}
	err = cf.cfnClient.CreateAndWait(s)
	if err != nil {
		return err
	}

	output, err := cf.cfnClient.Outputs(s)
	if err != nil {
		return err
	}
	// If the pipeline has a PipelineConnectionARN in the output map, indicating that it is has a CodeStarConnections source provider, the user needs to update the connection status; Copilot will wait until that happens.
	if output[connectionARNKey] == "" {
		return nil
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(45*time.Minute))
	defer cancel()
	if err = cf.codeStarClient.WaitUntilConnectionStatusAvailable(ctx, output[connectionARNKey]); err != nil {
		return err
	}
	if err = cf.cpClient.RetryStageExecution(in.Name, sourceStage); err != nil {
		return err
	}

	return nil
}

// UpdatePipeline updates an existing CodePipeline for deploying services.
func (cf CloudFormation) UpdatePipeline(in *deploy.CreatePipelineInput) error {
	s, err := toStack(stack.NewPipelineStackConfig(in))
	if err != nil {
		return err
	}
	if err := cf.cfnClient.UpdateAndWait(s); err != nil {
		var errNoUpdates *cloudformation.ErrChangeSetEmpty
		if errors.As(err, &errNoUpdates) {
			return nil
		}
		return fmt.Errorf("update pipeline: %w", err)
	}
	return nil
}

// DeletePipeline removes the CodePipeline stack.
func (cf CloudFormation) DeletePipeline(stackName string) error {
	return cf.cfnClient.DeleteAndWait(stackName)
}

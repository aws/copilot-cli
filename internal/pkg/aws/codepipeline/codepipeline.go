// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package codepipeline provides a client to make API requests to Amazon Elastic Container Service.
package codepipeline

import (
	"fmt"

	rg "github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/resourcegroups"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	cp "github.com/aws/aws-sdk-go/service/codepipeline"
)

const (
	pipelineResourceType = "AWS::CodePipeline::Pipeline"
)

type api interface {
	GetPipeline(*cp.GetPipelineInput) (*cp.GetPipelineOutput, error)
	GetPipelineState(*cp.GetPipelineStateInput) (*cp.GetPipelineStateOutput, error)
}

// CodePipeline wraps the AWS CodePipeline client.
type CodePipeline struct {
	client   api
	rgClient rg.ResourceGroupsClient
}

// Pipeline represents an existing CodePipeline resource.
type Pipeline struct {
	Name string `json:"name"`
}

// PipelineStatus represents a Pipeline's status.
type PipelineState struct {
	PipelineName string `json:"Pipeline"`
	StageStates []*cp.StageState `json:"StageStates"`
}

// New returns a CodePipeline client configured against the input session.
func New(s *session.Session) *CodePipeline {
	return &CodePipeline{
		client:   cp.New(s),
		rgClient: rg.New(s),
	}
}

// GetPipeline retrieves information from a given pipeline.
func (c *CodePipeline) GetPipeline(name string) (*Pipeline, error) {
	input := &cp.GetPipelineInput{
		Name: aws.String(name),
	}
	resp, err := c.client.GetPipeline(input)

	if err != nil {
		return nil, fmt.Errorf("get pipeline %s: %w", name, err)
	}
	pipeline := &Pipeline{
		Name: aws.StringValue(resp.Pipeline.Name),
	}

	return pipeline, nil
}

// GetPipelineStatus retrieves status information from a given pipeline.
func (c *CodePipeline) GetPipelineState(name string) (*PipelineState, error) {
	input := &cp.GetPipelineStateInput{
		Name: aws.String(name),
	}
	resp, err := c.client.GetPipelineState(input)

	if err != nil {
		return nil, fmt.Errorf("get pipeline state %s: %w", name, err)
	}
	pipelineState := &PipelineState{
		PipelineName: aws.StringValue(resp.PipelineName),
		StageStates: resp.StageStates,
	}

	return pipelineState, nil
}

// ListPipelineNamesByTags retrieves the names of all pipelines for a project.
func (c *CodePipeline) ListPipelineNamesByTags(tags map[string]string) ([]string, error) {
	var pipelineNames []string
	arns, err := c.rgClient.GetResourcesByTags(pipelineResourceType, tags)
	if err != nil {
		return nil, err
	}

	for _, arn := range arns {
		name, err := c.getPipelineName(arn)
		if err != nil {
			return nil, err
		}
		pipelineNames = append(pipelineNames, name)
	}

	return pipelineNames, nil
}

func (c *CodePipeline) getPipelineName(resourceArn string) (string, error) {
	parsedArn, err := arn.Parse(resourceArn)
	if err != nil {
		return "", fmt.Errorf("parse pipeline ARN: %s", resourceArn)
	}

	return parsedArn.Resource, nil
}

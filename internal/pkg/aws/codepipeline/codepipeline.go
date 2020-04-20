// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package codepipeline provides a client to make API requests to Amazon Elastic Container Service.
package codepipeline

import (
	"fmt"
	"strings"

	rg "github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/resourcegroups"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	cp "github.com/aws/aws-sdk-go/service/codepipeline"
)

const (
	pipelineResourceType = "AWS::CodePipeline::Pipeline"
)

type api interface {
	GetPipeline(*cp.GetPipelineInput) (*cp.GetPipelineOutput, error)
}

// CodePipeline wraps the AWS CodePipeline client.
type CodePipeline struct {
	client   api
	rgClient rg.ResourceGroupsClient
}

// Pipeline contains information about the pipeline
// TODO wrap nested resources or just use what the SDK provides?
type Pipeline struct {
	Name string `json:"name"`
	// Stages        []Stage       `json:"stages"`
	// ArtifactStore ArtifactStore `json:"artifactStore"`
}

// Stage wraps the codepipeline pipeline stage
type Stage cp.StageDeclaration

// ArtifactStore wraps the artifact store for the pipeline
type ArtifactStore cp.ArtifactStore

// New returns a CodePipeline client configured against the input session.
func New(s *session.Session) *CodePipeline {
	return &CodePipeline{
		client:   cp.New(s),
		rgClient: rg.New(s),
	}
}

// GetPipeline retrieves information from a given pipeline
func (c *CodePipeline) GetPipeline(pipelineName string) (*Pipeline, error) {
	input := &cp.GetPipelineInput{
		Name: aws.String(pipelineName),
	}
	resp, err := c.client.GetPipeline(input)

	if err != nil {
		return nil, fmt.Errorf("get pipeline %s: %w", pipelineName, err)
	}
	pipeline := &Pipeline{
		Name: aws.StringValue(resp.Pipeline.Name),
	}

	return pipeline, nil
}

// ListPipelines retrieves the names of all pipelines for a project
func (c *CodePipeline) ListPipelinesForProject(projectName string) ([]string, error) {
	var pipelineNames []string

	tags := map[string]string{
		"ecs-project": projectName,
	}

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

func (c *CodePipeline) getPipelineName(arn string) (string, error) {
	i := strings.LastIndex(arn, ":")
	if i == -1 {
		return "", fmt.Errorf("cannot parse pipeline ARN: %s", arn)
	}
	name := arn[i+1:]

	return name, nil
}

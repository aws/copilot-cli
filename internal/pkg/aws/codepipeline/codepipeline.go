// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package codepipeline provides a client to make API requests to Amazon Elastic Container Service.
package codepipeline

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	cp "github.com/aws/aws-sdk-go/service/codepipeline"
)

type api interface {
	GetPipeline(*cp.GetPipelineInput) (*cp.GetPipelineOutput, error)
	ListPipelines(*cp.ListPipelinesInput) (*cp.ListPipelinesOutput, error)
}

// CodePipeline wraps the AWS CodePipeline client.
type CodePipeline struct {
	client api
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
		client: cp.New(s),
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

// ListPipelines retrieves summaries of all pipelines for a project
func (c *CodePipeline) ListPipelines() ([]string, error) {
	input := &cp.ListPipelinesInput{}
	resp, err := c.client.ListPipelines(input)
	if err != nil {
		return nil, fmt.Errorf("list pipelines: %w", err)
	}

	var pipelines []string

	for _, ps := range resp.Pipelines {
		p := aws.StringValue(ps.Name)
		pipelines = append(pipelines, p)
	}

	return pipelines, nil
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package codepipeline provides a client to make API requests to Amazon Elastic Container Service.
package codepipeline

import (
	"fmt"
	"time"

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
}

// CodePipeline wraps the AWS CodePipeline client.
type CodePipeline struct {
	client   api
	rgClient rg.ResourceGroupsClient
}

// Pipeline represents an existing CodePipeline resource.
type Pipeline struct {
	Name      string  `json:"name"`
	Region    string  `json:"region"`
	AccountID string  `json:"accountId"`
	Stages    []Stage `json:"stages"`
	CreatedAt *time.Time
	UpdatedAt *time.Time
}

// Stage wraps the codepipeline pipeline stage
type Stage struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Provider string `json:"provider"`
	Details  string `json:"details"`
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

	pipeline := resp.Pipeline
	metadata := resp.Metadata
	arn := aws.StringValue(metadata.PipelineArn)

	parsedArn, err := arn.Parse(resourceArn)
	if err != nil {
		return "", fmt.Errorf("parse pipeline ARN: %s", resourceArn)
	}

	var stages []Stage
	for _, s := range pipeline.Stages {
		name := aws.StringValue(s.Name)
		action := s.Actions[0]

		category := aws.StringValue(action.ActionTypeId.Category)
		provider := aws.StringValue(action.ActionTypeId.Provider)
		config := action.Configuration

		var details string
		switch category {
		case "Source":
			details = fmt.Sprintf("Repository: %s/%s", aws.StringValue(config["Owner"]), aws.StringValue(config["Repo"]))
		case "Build":
			details = fmt.Sprintf("BuildProject: %s", aws.StringValue(config["ProjectName"]))
		case "Deploy":
			details = fmt.Sprintf("StackName: %s", aws.StringValue(config["StackName"]))
		default:
		}

		stage := Stage{
			Name:     name,
			Category: category,
			Provider: provider,
			Details:  details,
		}
		stages = append(stages, stage)
	}

	return &Pipeline{
		Name:      aws.StringValue(pipeline.Name),
		Region:    parsedArn.Region,
		AccountID: parsedArn.AccountID,
		Stages:    stages,
		CreatedAt: metadata.Created,
		UpdatedAt: metadata.Updated,
	}, nil
}

// HumanString returns the stringified Stage struct with human readable format.
// Example output:
//   DeployTo-test	Deploy	Cloudformation	stackname: dinder-test-test
func (s *Stage) HumanString() string {
	return fmt.Sprintf("  %s\t%s\t%s\t%s\n", s.Name, s.Category, s.Provider, s.Details)
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

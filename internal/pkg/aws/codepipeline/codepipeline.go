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
	GetPipelineState(*cp.GetPipelineStateInput) (*cp.GetPipelineStateOutput, error)
}

// CodePipeline wraps the AWS CodePipeline client.
type CodePipeline struct {
	client   api
	rgClient rg.ResourceGroupsClient
}

// Pipeline represents an existing CodePipeline resource.
type Pipeline struct {
	Name      string     `json:"name"`
	Region    string     `json:"region"`
	AccountID string     `json:"accountId"`
	Stages    []*Stage   `json:"stages"`
	CreatedAt *time.Time `json:"createdAt"`
	UpdatedAt *time.Time `json:"updatedAt"`
}

// Stage wraps the codepipeline pipeline stage
type Stage struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Provider string `json:"provider"`
	Details  string `json:"details"`
}

// PipelineStatus represents a Pipeline's status.
type PipelineState struct {
	PipelineName string        `json:"pipelineName"`
	StageStates  []*StageState `json:"stageStates"`
	UpdatedAt    *time.Time    `json:"updatedAt"`
}

// StageState wraps a CodePipeline stage state
type StageState struct {
	StageName  string `json:"stageName"`
	Status     string `json:"status"`
	Transition string `json:"transition"`
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
	pipelineArn := aws.StringValue(metadata.PipelineArn)

	parsedArn, err := arn.Parse(pipelineArn)
	if err != nil {
		return nil, fmt.Errorf("parse pipeline ARN: %s", pipelineArn)
	}

	var stages []*Stage
	for _, s := range pipeline.Stages {
		stage, err := c.getStage(s)
		if err != nil {
			return nil, fmt.Errorf("get stage for pipeline: %s", pipelineArn)
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

func (c *CodePipeline) getStage(s *cp.StageDeclaration) (*Stage, error) {
	name := aws.StringValue(s.Name)
	var category, provider, details string

	if len(s.Actions) > 0 {
		// Currently, we only support Source, Build and Deploy stages, all of which must contain at least one action.
		action := s.Actions[0]
		category = aws.StringValue(action.ActionTypeId.Category)
		provider = aws.StringValue(action.ActionTypeId.Provider)

		config := action.Configuration

		switch category {

		case "Source":
			// Currently, our only source provider is GitHub: https://docs.aws.amazon.com/codepipeline/latest/userguide/reference-pipeline-structure.html#structure-configuration-examples
			details = fmt.Sprintf("Repository: %s/%s", aws.StringValue(config["Owner"]), aws.StringValue(config["Repo"]))
		case "Build":
			// Currently, we use CodeBuild only for the build stage: https://docs.aws.amazon.com/codepipeline/latest/userguide/action-reference-CodeBuild.html#action-reference-CodeBuild-config
			details = fmt.Sprintf("BuildProject: %s", aws.StringValue(config["ProjectName"]))
		case "Deploy":
			// Currently, we use Cloudformation only for he build stage: https://docs.aws.amazon.com/codepipeline/latest/userguide/action-reference-CloudFormation.html#action-reference-CloudFormation-config
			details = fmt.Sprintf("StackName: %s", aws.StringValue(config["StackName"]))
		default:
			// not a currently recognized stage - empty string
		}
	}

	stage := &Stage{
		Name:     name,
		Category: category,
		Provider: provider,
		Details:  details,
	}
	return stage, nil
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

// GetPipelineStatus retrieves status information from a given pipeline.
func (c *CodePipeline) GetPipelineState(name string) (*PipelineState, error) {
	input := &cp.GetPipelineStateInput{
		Name: aws.String(name),
	}
	resp, err := c.client.GetPipelineState(input)

	if err != nil {
		return nil, fmt.Errorf("get pipeline state %s: %w", name, err)
	}
	var stageStates []*StageState
	for _, stage := range resp.StageStates {
		var stageName string
		if stage.StageName != nil {
			stageName = aws.StringValue(stage.StageName)
		}
		var transition string
		if stage.InboundTransitionState != nil {
			transition = "DISABLED"
			if *stage.InboundTransitionState.Enabled == true {
				transition = "ENABLED"
			}
		}
		var status string
		for _, actionState := range stage.ActionStates {
			if actionState.LatestExecution != nil {
				status = aws.StringValue(actionState.LatestExecution.Status)
			}
		}
		stageStates = append(stageStates, &StageState{
			StageName:  stageName,
			Status:     status,
			Transition: transition,
		})
	}
	return &PipelineState{
		PipelineName: aws.StringValue(resp.PipelineName),
		StageStates:  stageStates,
		UpdatedAt:    resp.Updated,
	}, nil
}

// HumanString returns the stringified PipelineState struct with human readable format.
// Example output:
//   DeployTo-test	Deploy	Cloudformation	stackname: dinder-test-test
func (s *StageState) HumanString() string {
	const empty = "  -"
status := s.Status
transition := s.Transition
if status == "" {
  status = empty
}
if transition == "" {
  transition = empty
}
return fmt.Sprintf("  %s\t%s\t%s\n", s.StageName, status, transition) 
}

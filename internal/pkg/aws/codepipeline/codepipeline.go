// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package codepipeline provides a client to make API requests to Amazon Elastic Container Service.
package codepipeline

import (
	"errors"
	"fmt"
	"time"

	"github.com/xlab/treeprint"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/session"
	cp "github.com/aws/aws-sdk-go/service/codepipeline"
	rg "github.com/aws/copilot-cli/internal/pkg/aws/resourcegroups"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

type api interface {
	GetPipeline(*cp.GetPipelineInput) (*cp.GetPipelineOutput, error)
	GetPipelineState(*cp.GetPipelineStateInput) (*cp.GetPipelineStateOutput, error)
	ListPipelineExecutions(input *cp.ListPipelineExecutionsInput) (*cp.ListPipelineExecutionsOutput, error)
	RetryStageExecution(input *cp.RetryStageExecutionInput) (*cp.RetryStageExecutionOutput, error)
}

type resourceGetter interface {
	GetResourcesByTags(resourceType string, tags map[string]string) ([]*rg.Resource, error)
}

// CodePipeline wraps the AWS CodePipeline client.
type CodePipeline struct {
	client   api
	rgClient resourceGetter
}

// Pipeline represents an existing CodePipeline resource.
type Pipeline struct {
	// Name is the resource name of the pipeline in CodePipeline, e.g. myapp-mypipeline-RANDOMSTRING.
	Name      string    `json:"pipelineName"`
	Region    string    `json:"region"`
	AccountID string    `json:"accountId"`
	Stages    []*Stage  `json:"stages"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Stage wraps the codepipeline pipeline stage.
type Stage struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Provider string `json:"provider"`
	Details  string `json:"details"`
}

// PipelineState represents a Pipeline's status.
type PipelineState struct {
	PipelineName string        `json:"pipelineName"`
	StageStates  []*StageState `json:"stageStates"`
	UpdatedAt    time.Time     `json:"updatedAt"`
}

// StageState wraps a CodePipeline stage state.
type StageState struct {
	StageName  string        `json:"stageName"`
	Actions    []StageAction `json:"actions,omitempty"`
	Transition string        `json:"transition"`
}

// StageAction wraps a CodePipeline stage action.
type StageAction struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// AggregateStatus returns the collective status of a stage by looking at each individual action's status.
// It returns "InProgress" if there are any actions that are in progress.
// It returns "Failed" if there are actions that failed or were abandoned.
// It returns "Succeeded" if all actions succeeded.
// It returns "" if there is no prior execution.
func (ss StageState) AggregateStatus() string {
	status := map[string]int{
		"":           0,
		"InProgress": 0,
		"Failed":     0,
		"Abandoned":  0,
		"Succeeded":  0,
	}
	for _, action := range ss.Actions {
		status[action.Status]++
	}
	if status["InProgress"] > 0 {
		return "InProgress"
	} else if status["Failed"]+status["Abandoned"] > 0 {
		return "Failed"
	} else if status["Succeeded"] > 0 && status["Succeeded"] == len(ss.Actions) {
		return "Succeeded"
	}
	return ""
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
		CreatedAt: *metadata.Created,
		UpdatedAt: *metadata.Updated,
	}, nil
}

// HumanString returns the stringified Stage struct with human readable format.
// Example output:
//   DeployTo-test	Deploy	Cloudformation	stackname: dinder-test-test
func (s *Stage) HumanString() string {
	return fmt.Sprintf("%s\t%s\t%s\t%s\n", s.Name, s.Category, s.Provider, s.Details)
}

// RetryStageExecution tries to re-initiate a failed stage for the given pipeline.
func (c *CodePipeline) RetryStageExecution(pipelineName, stageName string) error {
	executionID, err := c.pipelineExecutionID(pipelineName)
	if err != nil {
		return fmt.Errorf("retrieve pipeline execution ID: %w", err)
	}

	if _, err = c.client.RetryStageExecution(&cp.RetryStageExecutionInput{
		PipelineExecutionId: &executionID,
		PipelineName:        &pipelineName,
		RetryMode:           aws.String(cp.StageRetryModeFailedActions),
		StageName:           &stageName,
	}); err != nil {
		noFailedActions := &cp.StageNotRetryableException{}
		if !errors.As(err, &noFailedActions) {
			return fmt.Errorf("retry pipeline source stage: %w", err)
		}
	}
	return nil
}

// GetPipelineState retrieves status information from a given pipeline.
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
			if *stage.InboundTransitionState.Enabled {
				transition = "ENABLED"
			}
		}
		var actions []StageAction
		for _, actionState := range stage.ActionStates {
			if actionState.LatestExecution != nil {
				actions = append(actions, StageAction{
					Name:   aws.StringValue(actionState.ActionName),
					Status: aws.StringValue(actionState.LatestExecution.Status),
				})
			}
		}
		stageStates = append(stageStates, &StageState{
			StageName:  stageName,
			Actions:    actions,
			Transition: transition,
		})
	}
	return &PipelineState{
		PipelineName: aws.StringValue(resp.PipelineName),
		StageStates:  stageStates,
		UpdatedAt:    *resp.Updated,
	}, nil
}

// HumanString returns the stringified PipelineState struct with human readable format.
// Example output:
//   DeployTo-test	Deploy	Cloudformation	stackname: dinder-test-test
func (ss *StageState) HumanString() string {
	status := ss.AggregateStatus()
	transition := ss.Transition
	stageString := fmt.Sprintf("%s\t%s\t%s", ss.StageName, fmtStatus(transition), fmtStatus(status))
	tree := treeprint.NewWithRoot(stageString)
	for _, action := range ss.Actions {
		tree.AddNode(action.humanString())
	}
	return tree.String()
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
			// https://docs.aws.amazon.com/codepipeline/latest/userguide/reference-pipeline-structure.html#structure-configuration-examples
			switch provider {
			case "GitHub":
				details = fmt.Sprintf("Repository: %s/%s", aws.StringValue(config["Owner"]), aws.StringValue(config["Repo"]))
			case "CodeCommit":
				details = fmt.Sprintf("Repository: %s", aws.StringValue(config["RepositoryName"]))
			case "CodeStarSourceConnection":
				details = fmt.Sprintf("Repository: %s", aws.StringValue(config["FullRepositoryId"]))
			}
		case "Build":
			// Currently, we use CodeBuild only for the build stage: https://docs.aws.amazon.com/codepipeline/latest/userguide/action-reference-CodeBuild.html#action-reference-CodeBuild-config
			details = fmt.Sprintf("BuildProject: %s", aws.StringValue(config["ProjectName"]))
		case "Deploy":
			// Currently, we use Cloudformation only for the build stage: https://docs.aws.amazon.com/codepipeline/latest/userguide/action-reference-CloudFormation.html#action-reference-CloudFormation-config
			details = fmt.Sprintf("StackName: %s", aws.StringValue(config["StackName"]))
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

// pipelineExecutionID returns the ExecutionID of the most recent execution of a pipeline.
func (c *CodePipeline) pipelineExecutionID(pipelineName string) (string, error) {
	input := &cp.ListPipelineExecutionsInput{
		MaxResults:   aws.Int64(1),
		PipelineName: &pipelineName,
	}
	output, err := c.client.ListPipelineExecutions(input)
	if err != nil {
		return "", fmt.Errorf("list pipeline execution for %s: %w", pipelineName, err)
	}
	if len(output.PipelineExecutionSummaries) == 0 {
		return "", fmt.Errorf("no pipeline execution IDs found for %s", pipelineName)
	}
	return aws.StringValue(output.PipelineExecutionSummaries[0].PipelineExecutionId), nil
}

func (sa StageAction) humanString() string {
	return sa.Name + "\t\t" + fmtStatus(sa.Status)
}

func fmtStatus(status string) string {
	const empty = "  -"
	switch status {
	case "":
		return empty
	case "InProgress":
		return color.Emphasize(status)
	case "Failed":
		return color.Red.Sprint(status)
	case "DISABLED":
		return color.Faint.Sprint(status)
	default:
		return status
	}
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//Package job provides support for invoking scheduled jobs

package job

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/aws/stepfunctions"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
)

type jobExecutor interface {
	Execute(stateMachineARN string) error
}

type StackRetriever interface {
	StackResources(name string) ([]*cloudformation.StackResource, error)
}

type JobRunner struct {
	Executor jobExecutor

	//App Env and Job name to retrieve cloudformation stack
	App string
	Env string
	Job string

	StackRetriever StackRetriever
}

type JobRunnerConfig struct {
	App  string
	Env  string
	Job  string
	Sess *session.Session
}

func NewJobRunner(opts *JobRunnerConfig) *JobRunner {
	executor := stepfunctions.New(opts.Sess)

	retriever := cloudformation.New(opts.Sess)

	return &JobRunner{
		Executor:       executor,
		StackRetriever: retriever,
		App:            opts.App,
		Env:            opts.Env,
		Job:            opts.Job,
	}

}

func (r *JobRunner) Run() error {

	resources, err := r.StackRetriever.StackResources(stack.NameForService(r.App, r.Env, r.Job))

	if err != nil {
		return fmt.Errorf("describe stack %s: %v", stack.NameForService(r.App, r.Env, r.Job), err)
	}

	var stateMachineARN string

	for _, resource := range resources {

		if aws.StringValue(resource.ResourceType) == "AWS::StepFunctions::StateMachine" {
			stateMachineARN = aws.StringValue(resource.PhysicalResourceId)
			break
		}
	}

	if stateMachineARN == "" {
		return fmt.Errorf("statemachine not found")
	}

	err = r.Executor.Execute(stateMachineARN)

	if err != nil {
		return fmt.Errorf("statemachine execution: %v", err)
	}

	return nil
}

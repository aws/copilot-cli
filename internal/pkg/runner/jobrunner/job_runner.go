// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//Package jobrunner provides support for invoking scheduled jobs

package jobrunner

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
	//Executes the job statemachine
	executor jobExecutor

	//Application of the job
	app string

	// Environment where the job will be executed
	env string

	//The name of the Job to be executed
	job string

	//Retrieves the stack resources from cloudformation
	stackRetriever StackRetriever
}

type JobRunnerConfig struct {
	App  string
	Env  string
	Job  string
	Sess *session.Session
}

func New(opts *JobRunnerConfig) *JobRunner {
	executor := stepfunctions.New(opts.Sess)

	retriever := cloudformation.New(opts.Sess)

	return &JobRunner{
		executor:       executor,
		stackRetriever: retriever,
		app:            opts.App,
		env:            opts.Env,
		job:            opts.Job,
	}

}

func (r *JobRunner) Run() error {

	resources, err := r.stackRetriever.StackResources(stack.NameForService(r.app, r.env, r.job))

	if err != nil {
		return fmt.Errorf("describe stack %s: %v", stack.NameForService(r.app, r.env, r.job), err)
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

	err = r.executor.Execute(stateMachineARN)

	if err != nil {
		return fmt.Errorf("statemachine execution: %v", err)
	}

	return nil
}

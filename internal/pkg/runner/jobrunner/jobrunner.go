// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package jobrunner provides support for invoking jobs.
package jobrunner

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
)

// StateMachineExecutor is the interface that implements the Execute method to invoke a state machine.
type StateMachineExecutor interface {
	Execute(stateMachineARN string) error
}

// CFNStackResourceLister is the interface to list CloudFormation stack resources.
type CFNStackResourceLister interface {
	StackResources(name string) ([]*cloudformation.StackResource, error)
}

// JobRunner can invoke a job.
type JobRunner struct {
	app string
	env string
	job string

	cfn          CFNStackResourceLister
	stateMachine StateMachineExecutor
}

// Config hold the data needed to create a JobRunner.
type Config struct {
	App string // Name of the application.
	Env string // Name of the environment.
	Job string // Name of the job.

	// Dependencies to invoke a job.
	CFN          CFNStackResourceLister // CloudFormation client to list stack resources.
	StateMachine StateMachineExecutor   // StepFunction client to execute a state machine.
}

// New creates a new JobRunner.
func New(cfg *Config) *JobRunner {
	return &JobRunner{
		app:          cfg.App,
		env:          cfg.Env,
		job:          cfg.Job,
		cfn:          cfg.CFN,
		stateMachine: cfg.StateMachine,
	}

}

// Run invokes a job.
// An error is returned if the state machine's ARN can not be derived from the job, or the execution fails.
func (job *JobRunner) Run() error {
	resources, err := job.cfn.StackResources(stack.NameForWorkload(job.app, job.env, job.job))
	if err != nil {
		return fmt.Errorf("describe stack %q: %v", stack.NameForWorkload(job.app, job.env, job.job), err)
	}

	var arn string
	for _, resource := range resources {
		if aws.StringValue(resource.ResourceType) == "AWS::StepFunctions::StateMachine" {
			arn = aws.StringValue(resource.PhysicalResourceId)
			break
		}
	}
	if arn == "" {
		return fmt.Errorf("state machine for job %q is not found in environment %q and application %q", job.job, job.env, job.app)
	}
	if err := job.stateMachine.Execute(arn); err != nil {
		return fmt.Errorf("execute state machine %q: %v", arn, err)
	}
	return nil
}

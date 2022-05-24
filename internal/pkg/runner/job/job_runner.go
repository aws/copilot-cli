// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

//Package job provides support for invoking scheduled jobs

package job

import (
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
)

type JobExecutor interface {
	Execute(stateMachineARN string) error
}

type StackRetriever interface {
	StackResources(name string) ([]*cloudformation.StackResource, error)
}

type JobRunner struct {
	Executor JobExecutor

	//App Env and Job name to retrieve cloudformation stack
	App string
	Env string
	Job string

	StackRetriever StackRetriever
}

func (r *JobRunner) Run() error {

	resources, err := r.StackRetriever.StackResources(fmt.Sprintf("%s-%s-%s", r.App, r.Env, r.Job))

	if err != nil {
		return fmt.Errorf("describe stack error: %v", err)
	}

	var stateMachineARN string

	for _, resource := range resources {

		if *resource.ResourceType == "AWS::StepFunctions::StateMachine" {
			stateMachineARN = *resource.PhysicalResourceId
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

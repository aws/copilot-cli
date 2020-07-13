// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides functionality to deploy ECS resources with AWS CloudFormation.
package cloudformation

import (
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
)

// DeployEnvironment creates the CloudFormation stack for an environment by creating and executing a change set.
//
// If the deployment succeeds, returns nil.
// If the stack already exists, returns a ErrStackAlreadyExists.
// If the change set to create the stack cannot be executed, returns a ErrNotExecutableChangeSet.
// Otherwise, returns a wrapped error.
func (cf CloudFormation) DeployEnvironment(env *deploy.CreateEnvironmentInput) error {
	s, err := toStack(stack.NewEnvStackConfig(env))
	if err != nil {
		return err
	}
	return cf.cfnClient.Create(s)
}

// StreamEnvironmentCreation streams resource update events while a deployment is taking place.
// Once the CloudFormation stack operation halts, the update channel is closed and a
// CreateEnvironmentResponse is sent to the second channel.
func (cf CloudFormation) StreamEnvironmentCreation(env *deploy.CreateEnvironmentInput) (<-chan []deploy.ResourceEvent, <-chan deploy.CreateEnvironmentResponse) {
	done := make(chan struct{})
	events := make(chan []deploy.ResourceEvent)
	resp := make(chan deploy.CreateEnvironmentResponse, 1)

	stack := stack.NewEnvStackConfig(env)
	go cf.streamResourceEvents(done, events, stack.StackName())
	go cf.streamEnvironmentResponse(done, resp, stack)
	return events, resp
}

// DeleteEnvironment deletes the CloudFormation stack of an environment.
func (cf CloudFormation) DeleteEnvironment(appName, envName string) error {
	conf := stack.NewEnvStackConfig(&deploy.CreateEnvironmentInput{
		AppName: appName,
		Name:    envName,
	})
	return cf.cfnClient.DeleteAndWait(conf.StackName())
}

// streamEnvironmentResponse sends a CreateEnvironmentResponse to the response channel once the stack creation halts.
// The done channel is closed once this method exits to notify other streams that they should stop working.
func (cf CloudFormation) streamEnvironmentResponse(done chan struct{}, resp chan deploy.CreateEnvironmentResponse, stack *stack.EnvStackConfig) {
	defer close(done)
	if err := cf.cfnClient.WaitForCreate(stack.StackName()); err != nil {
		resp <- deploy.CreateEnvironmentResponse{Err: err}
		return
	}
	descr, err := cf.cfnClient.Describe(stack.StackName())
	if err != nil {
		resp <- deploy.CreateEnvironmentResponse{Err: err}
		return
	}
	env, err := stack.ToEnv(descr.SDK())
	resp <- deploy.CreateEnvironmentResponse{
		Env: env,
		Err: err,
	}
}

// GetEnvironment returns the Environment metadata from the CloudFormation stack.
func (cf CloudFormation) GetEnvironment(appName, envName string) (*config.Environment, error) {
	conf := stack.NewEnvStackConfig(&deploy.CreateEnvironmentInput{
		AppName: appName,
		Name:    envName,
	})
	descr, err := cf.cfnClient.Describe(conf.StackName())
	if err != nil {
		return nil, err
	}
	return conf.ToEnv(descr.SDK())
}

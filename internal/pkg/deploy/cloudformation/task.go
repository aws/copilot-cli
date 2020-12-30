// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
)

const taskStackPrefix = "task-"

// DeployTask deploys a task stack and waits until the deployment is done.
// If the task stack doesn't exist, then it creates the stack.
// If the task stack already exists, it updates the stack.
// If the task stack doesn't have any changes, it returns nil
func (cf CloudFormation) DeployTask(input *deploy.CreateTaskResourcesInput, opts ...cloudformation.StackOption) error {
	conf := stack.NewTaskStackConfig(input)
	stack, err := toStack(conf)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(stack)
	}

	err = cf.cfnClient.CreateAndWait(stack)
	if err == nil {
		return nil
	}

	var errAlreadyExists *cloudformation.ErrStackAlreadyExists
	if !errors.As(err, &errAlreadyExists) {
		return fmt.Errorf("create stack: %w", err)
	}

	err = cf.cfnClient.UpdateAndWait(stack)
	if err == nil {
		return nil
	}

	var errChangeSetEmpty *cloudformation.ErrChangeSetEmpty
	if !errors.As(err, &errChangeSetEmpty) {
		return fmt.Errorf("update stack: %w", err)
	}

	return nil
}

// ListTaskStacks returns all the CF stacks which represent one-off copilot tasks in a given application's environments
func (cf CloudFormation) ListTaskStacks(appName, envName string) ([]deploy.TaskStackInfo, error) {
	tasks, err := cf.cfnClient.ListStacksWithPrefix(taskStackPrefix)
	if err != nil {
		return nil, err
	}
	var outputTaskStacks []deploy.TaskStackInfo
	for _, task := range tasks {
		var hasTaskTag, hasAppTag, hasEnvTag bool
		for _, tag := range task.Tags {
			if aws.StringValue(tag.Key) == deploy.TaskTagKey {
				hasTaskTag = true
			}

			if aws.StringValue(tag.Key) == deploy.AppTagKey && aws.StringValue(tag.Value) == appName {
				hasAppTag = true
			}

			if aws.StringValue(tag.Key) == deploy.EnvTagKey && aws.StringValue(tag.Value) == envName {
				hasEnvTag = true
			}
		}

		if !hasTaskTag || !hasAppTag || !hasEnvTag {
			continue
		}

		outputTaskStacks = append(outputTaskStacks, deploy.TaskStackInfo{
			StackName: aws.StringValue(task.StackName),
			App:       appName,
			Env:       envName,

			RoleARN: aws.StringValue(task.RoleARN),
		})
	}
	return outputTaskStacks, nil
}

// ListDefaultTaskStacks returns all the CF stacks created by copilot but not associated with an application.
func (cf CloudFormation) ListDefaultTaskStacks() ([]deploy.TaskStackInfo, error) {
	tasks, err := cf.cfnClient.ListStacksWithPrefix(taskStackPrefix)
	if err != nil {
		return nil, err
	}
	var outputTaskStacks []deploy.TaskStackInfo
	for _, task := range tasks {
		var hasTaskTag, hasAppTag bool
		for _, tag := range task.Tags {
			if aws.StringValue(tag.Key) == deploy.TaskTagKey {
				hasTaskTag = true
			}
			if aws.StringValue(tag.Key) == deploy.AppTagKey {
				hasAppTag = true
			}
		}
		if !hasTaskTag || hasAppTag {
			continue
		}

		// Check the RoleArn of the task to see if it's created using a particular role, which would disqualify it.
		if aws.StringValue(task.RoleARN) != "" {
			continue
		}
		outputTaskStacks = append(outputTaskStacks, deploy.TaskStackInfo{
			StackName: aws.StringValue(task.StackName),
			App:       "",
			Env:       "",

			RoleARN: "",
		})
	}
	return outputTaskStacks, nil
}

// DeleteTask deletes a Copilot-created one-off task stack using the RoleARN that stack was created with.
func (cf CloudFormation) DeleteTask(task deploy.TaskStackInfo) error {
	return cf.cfnClient.DeleteAndWaitWithRoleARN(task.StackName, task.RoleARN)
}

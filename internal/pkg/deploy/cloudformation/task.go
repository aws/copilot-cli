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

// DeployTask deploys a task stack, renders the deployment to out until it is done.
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

	if err := cf.executeAndRenderChangeSet(cf.newUpsertChangeSetInput(cf.console, stack)); err != nil {
		var errChangeSetEmpty *cloudformation.ErrChangeSetEmpty
		if !errors.As(err, &errChangeSetEmpty) {
			return err
		}
	}
	return nil
}

// ListTaskStacks returns all the CF stacks which represent one-off copilot tasks in a given application's environments.
func (cf CloudFormation) ListTaskStacks(appName, envName string) ([]deploy.TaskStackInfo, error) {
	taskAppEnvTags := map[string]string{
		deploy.TaskTagKey: "",
		deploy.AppTagKey:  appName,
		deploy.EnvTagKey:  envName,
	}
	tasks, err := cf.cfnClient.ListStacksWithTags(taskAppEnvTags)

	if err != nil {
		return nil, err
	}
	var outputTaskStacks []deploy.TaskStackInfo
	for _, task := range tasks {

		outputTaskStacks = append(outputTaskStacks, deploy.TaskStackInfo{
			StackName: aws.StringValue(task.StackName),
			App:       appName,
			Env:       envName,

			RoleARN: aws.StringValue(task.RoleARN),
		})
	}
	return outputTaskStacks, nil
}

// GetTaskStack grabs information about the given one-off task's cloudformation stack
// and returns it to the user in a convenient struct.
func (cf CloudFormation) GetTaskStack(taskName string) (*deploy.TaskStackInfo, error) {
	stackName := string(stack.NameForTask(taskName))
	desc, err := cf.cfnClient.Describe(stackName)
	if err != nil {
		return nil, err
	}
	info := deploy.TaskStackInfo{
		StackName: stackName,
		RoleARN:   aws.StringValue(desc.RoleARN),
	}
	var isTask bool
	for _, tag := range desc.Tags {
		switch aws.StringValue(tag.Key) {
		case deploy.AppTagKey:
			info.App = aws.StringValue(tag.Value)
		case deploy.EnvTagKey:
			info.Env = aws.StringValue(tag.Value)
		case deploy.TaskTagKey:
			isTask = true
		}
	}
	for _, out := range desc.Outputs {
		switch aws.StringValue(out.OutputKey) {
		case stack.TaskOutputS3Bucket:
			info.BucketName = aws.StringValue(out.OutputValue)
		}
	}
	if !isTask {
		return nil, fmt.Errorf("%s is not a Copilot task stack", stackName)
	}
	return &info, nil
}

// ListDefaultTaskStacks returns all the CF stacks created by copilot but not associated with an application.
func (cf CloudFormation) ListDefaultTaskStacks() ([]deploy.TaskStackInfo, error) {
	tasks, err := cf.cfnClient.ListStacksWithTags(map[string]string{deploy.TaskTagKey: ""})
	if err != nil {
		return nil, err
	}
	var outputTaskStacks []deploy.TaskStackInfo
	for _, task := range tasks {
		// Eliminate tasks which are tagged for a particular copilot app or env.
		var hasAppTag, hasEnvTag bool
		for _, tag := range task.Tags {
			if aws.StringValue(tag.Key) == deploy.AppTagKey {
				hasAppTag = true
			}
			if aws.StringValue(tag.Key) == deploy.EnvTagKey {
				hasEnvTag = true
			}
		}
		if hasAppTag || hasEnvTag {
			continue
		}
		outputTaskStacks = append(outputTaskStacks, deploy.TaskStackInfo{
			StackName: aws.StringValue(task.StackName),
		})
	}
	return outputTaskStacks, nil
}

// DeleteTask deletes a Copilot-created one-off task stack using the RoleARN that stack was created with.
// If there is no role arn specified, it tries to delete the stack using the default session.
func (cf CloudFormation) DeleteTask(task deploy.TaskStackInfo) error {
	if task.RoleARN != "" {
		return cf.cfnClient.DeleteAndWaitWithRoleARN(task.StackName, task.RoleARN)
	}
	return cf.cfnClient.DeleteAndWait(task.StackName)
}

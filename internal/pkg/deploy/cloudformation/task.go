// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
)

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

// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
)

// DeployApp deploys an application stack and waits until the deployment is done.
// If the application doesn't exist, then it creates the stack.
// If the application already exists, it updates the stack.
func (cf CloudFormation) DeployApp(conf StackConfiguration, opts ...cloudformation.StackOption) error {
	stack, err := toStack(conf)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(stack)
	}

	err = cf.cfnClient.CreateAndWait(stack)
	if err == nil { // Created a new stack, stop execution.
		return nil
	}
	// The stack already exists, we need to update it instead.
	var errAlreadyExists *cloudformation.ErrStackAlreadyExists
	if !errors.As(err, &errAlreadyExists) {
		return err
	}
	return cf.cfnClient.UpdateAndWait(stack)
}

// DeleteApp removes the CloudFormation stack of a deployed application.
func (cf CloudFormation) DeleteApp(in deploy.DeleteAppInput) error {
	return cf.cfnClient.DeleteAndWait(fmt.Sprintf("%s-%s-%s", in.ProjectName, in.EnvName, in.AppName))
}

// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
)

// DeployService deploys a service stack and waits until the deployment is done.
// If the service stack doesn't exist, then it creates the stack.
// If the service stack already exists, it updates the stack.
func (cf CloudFormation) DeployService(conf StackConfiguration, opts ...cloudformation.StackOption) error {
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

// DeleteService removes the CloudFormation stack of a deployed service.
func (cf CloudFormation) DeleteService(in deploy.DeleteServiceInput) error {
	return cf.cfnClient.DeleteAndWait(fmt.Sprintf("%s-%s-%s", in.AppName, in.EnvName, in.Name))
}

// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"errors"
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudformation"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
)

// DeployApp wraps the application deployment flow and handles orchestration of
// creating a stack versus updating a stack.
func (cf CloudFormation) DeployApp(template, stackName, changeSetName, cfExecutionRole string, tags map[string]string, parameters map[string]string) error {
	stack := cloudformation.NewStack(stackName, template,
		cloudformation.WithParameters(parameters),
		cloudformation.WithTags(tags),
		cloudformation.WithRoleARN(cfExecutionRole))

	err := cf.cfnClient.CreateAndWait(stack)
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

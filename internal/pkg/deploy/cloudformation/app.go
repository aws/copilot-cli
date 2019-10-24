// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// DeployApp wraps the application deployment flow and handles orchestration of
// creating a stack versus updating a stack.
func (cf CloudFormation) DeployApp(template, stackName, changeSetName string) error {
	_, err := cf.client.CreateStack(&cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(template),
		Capabilities: aws.StringSlice([]string{cloudformation.CapabilityCapabilityIam}),
	})

	// If CreateStack does not return an error we'll wait for StackCreateComplete status.
	if err == nil {
		err = cf.client.WaitUntilStackCreateComplete(&cloudformation.DescribeStacksInput{
			StackName: aws.String(stackName),
		})

		if err != nil {
			return fmt.Errorf("wait for stack completion: %w", err)
		}

		return nil
	}

	// If error returned from CreateStack is ErrCodeAlreadyExistsException move on to CreateChangeSet flow.
	if err != nil {
		awsErr, ok := err.(awserr.Error)
		if !ok {
			return fmt.Errorf("create stack: %w", err)
		}
		if awsErr.Code() != cloudformation.ErrCodeAlreadyExistsException {
			return fmt.Errorf("create stack: %w", err)
		}
	}

	_, err = cf.client.CreateChangeSet(&cloudformation.CreateChangeSetInput{
		ChangeSetName: aws.String(changeSetName),
		StackName:     aws.String(stackName),
		TemplateBody:  aws.String(template),
		Capabilities:  aws.StringSlice([]string{cloudformation.CapabilityCapabilityIam}),
		ChangeSetType: aws.String("UPDATE"),
	})

	if err != nil {
		// TODO: gracefully handle stack already in UPDATE_IN_PROGRESS state
		return fmt.Errorf("create change set: %w", err)
	}

	err = cf.client.WaitUntilChangeSetCreateComplete(&cloudformation.DescribeChangeSetInput{
		ChangeSetName: aws.String(changeSetName),
		StackName:     aws.String(stackName),
	})

	// NOTE: WaitUntilChangeSetCreateComplete just straight up fails if there are no changes to apply.
	if err != nil {
		// TODO: find a better way to handle no ChangeSet changes.
		// Chances are here that there are no changes to apply, but it's janky to handle.
		return fmt.Errorf("wait for change set create complete: %w", err)
	}

	_, err = cf.client.ExecuteChangeSet(&cloudformation.ExecuteChangeSetInput{
		ChangeSetName: aws.String(changeSetName),
		StackName:     aws.String(stackName),
	})

	if err != nil {
		return fmt.Errorf("execute change set: %w", err)
	}

	if err := cf.client.WaitUntilStackUpdateComplete(&cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}); err != nil {
		return fmt.Errorf("wait for stack update: %w", err)
	}

	return nil
}

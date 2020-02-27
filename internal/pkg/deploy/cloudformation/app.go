// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// DeployApp wraps the application deployment flow and handles orchestration of
// creating a stack versus updating a stack.
func (cf CloudFormation) DeployApp(template, stackName, changeSetName, cfExecutionRole string, tags map[string]string, parameters map[string]string) error {
	var cfnTags []*cloudformation.Tag
	for k, v := range tags {
		cfnTags = append(cfnTags, &cloudformation.Tag{
			Key:   aws.String(k),
			Value: aws.String(v),
		})
	}
	var cfnParams []*cloudformation.Parameter
	for k, v := range parameters {
		cfnParams = append(cfnParams, &cloudformation.Parameter{
			ParameterKey:   aws.String(k),
			ParameterValue: aws.String(v),
		})
	}

	_, err := cf.client.CreateStack(&cloudformation.CreateStackInput{
		StackName:    aws.String(stackName),
		TemplateBody: aws.String(template),
		Capabilities: aws.StringSlice([]string{cloudformation.CapabilityCapabilityIam}),
		Tags:         cfnTags,
		Parameters:   cfnParams,
		RoleARN:      aws.String(cfExecutionRole),
	})

	// If CreateStack does not return an error we'll wait for StackCreateComplete status.
	if err == nil {
		err = cf.client.WaitUntilStackCreateCompleteWithContext(context.Background(), &cloudformation.DescribeStacksInput{
			StackName: aws.String(stackName),
		}, cf.waiters...)

		if err != nil {
			return fmt.Errorf("wait for stack completion: %w", err)
		}

		return nil
	}

	// If error returned from CreateStack is ErrCodeAlreadyExistsException move on to CreateChangeSet flow.
	awsErr, ok := err.(awserr.Error)
	if !ok {
		return fmt.Errorf("create stack: %w", err)
	}
	if awsErr.Code() != cloudformation.ErrCodeAlreadyExistsException {
		return fmt.Errorf("create stack: %w", err)
	}

	_, err = cf.client.CreateChangeSet(&cloudformation.CreateChangeSetInput{
		ChangeSetName: aws.String(changeSetName),
		StackName:     aws.String(stackName),
		TemplateBody:  aws.String(template),
		Capabilities:  aws.StringSlice([]string{cloudformation.CapabilityCapabilityIam}),
		ChangeSetType: aws.String(cloudformation.ChangeSetTypeUpdate),
		Tags:          cfnTags,
		Parameters:    cfnParams,
		RoleARN:       aws.String(cfExecutionRole),
	})

	if err != nil {
		// TODO: gracefully handle stack already in UPDATE_IN_PROGRESS state
		return fmt.Errorf("create change set: %w", err)
	}

	describeChangeSetInput := &cloudformation.DescribeChangeSetInput{
		ChangeSetName: aws.String(changeSetName),
		StackName:     aws.String(stackName),
	}

	err = cf.client.WaitUntilChangeSetCreateCompleteWithContext(context.Background(), describeChangeSetInput, cf.waiters...)

	// NOTE: If WaitUntilChangeSetCreateComplete returns an error it's possible that there
	// are simply no changes between the previous and proposed Stack ChangeSets. We make a call to
	// DescribeChangeSet to see if that is indeed the case and handle it gracefully.
	if err != nil {
		out, describeChangeSetErr := cf.client.DescribeChangeSet(describeChangeSetInput)

		if describeChangeSetErr != nil {
			return fmt.Errorf("describe change set: %w", describeChangeSetErr)
		}

		if len(out.Changes) == 0 {
			return nil
		}

		return fmt.Errorf("wait for change set create complete: %w", describeChangeSetErr)
	}

	_, err = cf.client.ExecuteChangeSet(&cloudformation.ExecuteChangeSetInput{
		ChangeSetName: aws.String(changeSetName),
		StackName:     aws.String(stackName),
	})

	if err != nil {
		return fmt.Errorf("execute change set: %w", err)
	}

	if err := cf.client.WaitUntilStackUpdateCompleteWithContext(context.Background(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, cf.waiters...); err != nil {
		return fmt.Errorf("wait for stack update: %w", err)
	}

	return nil
}

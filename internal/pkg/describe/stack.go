// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

type cfnStackDescriber interface {
	DescribeStacks(input *cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
	DescribeStackResources(input *cloudformation.DescribeStackResourcesInput) (*cloudformation.DescribeStackResourcesOutput, error)
}

// StackDescriber retrieves information of a CloudFormation Stack.
type StackDescriber struct {
	stackDescribers cfnStackDescriber
}

// NewStackDescriber instantiates a new StackDescriber struct.
func NewStackDescriber(s *session.Session) *StackDescriber {
	return &StackDescriber{
		stackDescribers: cloudformation.New(s),
	}
}

// Stack returns the CloudFormation stack information.
func (d *StackDescriber) Stack(stackName string) (*cloudformation.Stack, error) {
	out, err := d.stackDescribers.DescribeStacks(&cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return nil, fmt.Errorf("describe stack %s: %w", stackName, err)
	}
	if len(out.Stacks) == 0 {
		return nil, fmt.Errorf("stack %s not found", stackName)
	}
	return out.Stacks[0], nil
}

// StackResources returns the CloudFormation stack resources information.
func (d *StackDescriber) StackResources(stackName string) ([]*cloudformation.StackResource, error) {
	out, err := d.stackDescribers.DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return nil, fmt.Errorf("describe resources for stack %s: %w", stackName, err)
	}
	return out.StackResources, nil
}

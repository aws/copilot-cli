// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
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

// stackDescriber retrieves information of a CloudFormation Stack.
type stackDescriber struct {
	stackDescribers cfnStackDescriber
}

// newStackDescriber instantiates a new StackDescriber struct.
func newStackDescriber(s *session.Session) *stackDescriber {
	return &stackDescriber{
		stackDescribers: cloudformation.New(s),
	}
}

// Stack returns the CloudFormation stack information.
func (d *stackDescriber) Stack(stackName string) (*cloudformation.Stack, error) {
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
func (d *stackDescriber) StackResources(stackName string) ([]*cloudformation.StackResource, error) {
	out, err := d.stackDescribers.DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		return nil, fmt.Errorf("describe resources for stack %s: %w", stackName, err)
	}
	return out.StackResources, nil
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides a client to make API requests to AWS CloudFormation.
package cloudformation

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

var waiters = []request.WaiterOption{
	request.WithWaiterDelay(request.ConstantWaiterDelay(3 * time.Second)), // Poll for cfn updates every 3 seconds.
	request.WithWaiterMaxAttempts(1800),                                   // Wait for at most 90 mins for any cfn action.
}

type api interface {
	changeSetAPI

	DescribeStacks(*cloudformation.DescribeStacksInput) (*cloudformation.DescribeStacksOutput, error)
	DeleteStack(*cloudformation.DeleteStackInput) (*cloudformation.DeleteStackOutput, error)
	WaitUntilStackDeleteCompleteWithContext(aws.Context, *cloudformation.DescribeStacksInput, ...request.WaiterOption) error
}

// CloudFormation represents a client to make requests to AWS CloudFormation.
type CloudFormation struct {
	client api
}

// New creates a new CloudFormation client.
func New(s *session.Session) *CloudFormation {
	return &CloudFormation{
		client: cloudformation.New(s),
	}
}

// Create deploys a new CloudFormation stack.
func (c *CloudFormation) Create(stack *Stack) error {
	descr, err := c.describe(stack.name)
	if err != nil {
		var stackNotFound *errStackNotFound
		if !errors.As(err, &stackNotFound) {
			return err
		}
		// If the stack does not exist, create it.
		return c.createChangeSet(stack)
	}
	status := stackStatus(aws.StringValue(descr.StackStatus))
	if status.requiresCleanup() {
		// If the stack exists, but failed to create, we'll clean it up and then re-create it.
		if err := c.delete(stack.name); err != nil {
			return fmt.Errorf("cleanup previously failed stack %s: %w", stack.name, err)
		}
		return c.createChangeSet(stack)
	}
	if status.inProgress() {
		return &errStackUpdateInProgress{
			name: stack.name,
		}
	}
	return &ErrStackAlreadyExists{
		name: stack.name,
	}
}

func (c *CloudFormation) describe(name string) (*cloudformation.Stack, error) {
	out, err := c.client.DescribeStacks(&cloudformation.DescribeStacksInput{
		StackName: aws.String(name),
	})
	if err != nil {
		if stackDoesNotExist(err) {
			return nil, &errStackNotFound{name: name}
		}
		return nil, err
	}
	if len(out.Stacks) == 0 {
		return nil, &errStackNotFound{name: name}
	}
	return out.Stacks[0], nil
}

func (c *CloudFormation) delete(stackName string) error {
	_, err := c.client.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		if stackDoesNotExist(err) {
			return nil
		}
		return fmt.Errorf("delete stack %s: %w", stackName, err)
	}
	err = c.client.WaitUntilStackDeleteCompleteWithContext(context.Background(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, waiters...)
	if err != nil {
		return fmt.Errorf("wait until stack %s delete is complete: %w", stackName, err)
	}
	return nil
}

func (c *CloudFormation) createChangeSet(stack *Stack) error {
	cs, err := newChangeSet(c.client, stack.name)
	if err != nil {
		return err
	}
	if err := cs.create(stack.template, stack.parameters, stack.tags); err != nil {
		// It's possible that there are no changes between the previous and proposed stack change sets.
		// We make a call to describe the change set to see if that is indeed the case and handle it gracefully.
		descr, descrErr := cs.describe()
		if descrErr != nil {
			return descrErr
		}
		// The change set was empty - so we clean it up.
		// We have to clean up the change set because there's a limit on the number
		// of failed change sets a customer can have on a particular stack.
		if len(descr.changes) == 0 {
			cs.delete()
			return &ErrChangeSetEmpty{
				cs: cs,
			}
		}
		return err
	}
	return cs.execute()
}

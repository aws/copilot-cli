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

// Create deploys a new CloudFormation stack using Change Sets.
// If the stack already exists in a failed state, deletes the stack and re-creates it.
func (c *CloudFormation) Create(stack *Stack) error {
	descr, err := c.describe(stack.name)
	if err != nil {
		var stackNotFound *ErrStackNotFound
		if !errors.As(err, &stackNotFound) {
			return err
		}
		// If the stack does not exist, create it.
		return c.create(stack)
	}
	status := stackStatus(aws.StringValue(descr.StackStatus))
	if status.requiresCleanup() {
		// If the stack exists, but failed to create, we'll clean it up and then re-create it.
		if err := c.Delete(stack.name); err != nil {
			return fmt.Errorf("cleanup previously failed stack %s: %w", stack.name, err)
		}
		return c.create(stack)
	}
	if status.inProgress() {
		return &errStackUpdateInProgress{
			name: stack.name,
		}
	}
	return &ErrStackAlreadyExists{
		Name:  stack.name,
		Stack: descr,
	}
}

// CreateAndWait calls Create and then blocks until the stack is created or until the max attempt window expires.
func (c *CloudFormation) CreateAndWait(stack *Stack) error {
	if err := c.Create(stack); err != nil {
		return err
	}

	err := c.client.WaitUntilStackCreateCompleteWithContext(context.Background(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(stack.name),
	}, waiters...)
	if err != nil {
		return fmt.Errorf("wait until stack %s create is complete: %w", stack.name, err)
	}
	return nil
}

// Update updates an existing CloudFormation with the new configuration.
// If there are no changes for the stack, deletes the empty change set and returns ErrChangeSetEmpty.
func (c *CloudFormation) Update(stack *Stack) error {
	descr, err := c.describe(stack.name)
	if err != nil {
		return err
	}
	status := stackStatus(aws.StringValue(descr.StackStatus))
	if status.inProgress() {
		return &errStackUpdateInProgress{
			name: stack.name,
		}
	}
	return c.update(stack)
}

// UpdateAndWait calls Update and then blocks until the stack is updated or until the max attempt window expires.
func (c *CloudFormation) UpdateAndWait(stack *Stack) error {
	if err := c.Update(stack); err != nil {
		return err
	}

	err := c.client.WaitUntilStackUpdateCompleteWithContext(context.Background(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(stack.name),
	}, waiters...)
	if err != nil {
		return fmt.Errorf("wait until stack %s update is complete: %w", stack.name, err)
	}
	return nil
}

// Delete removes an existing CloudFormation stack.
// If the stack doesn't exist then do nothing.
func (c *CloudFormation) Delete(stackName string) error {
	_, err := c.client.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		if !stackDoesNotExist(err) {
			return fmt.Errorf("delete stack %s: %w", stackName, err)
		}
		// Move on if stack is already deleted.
	}
	return nil
}

// DeleteAndWait calls Delete then blocks until the stack is deleted or until the max attempt window expires.
func (c *CloudFormation) DeleteAndWait(stackName string) error {
	_, err := c.client.DeleteStack(&cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	})
	if err != nil {
		if !stackDoesNotExist(err) {
			return fmt.Errorf("delete stack %s: %w", stackName, err)
		}
		return nil // If the stack is already deleted, don't wait for it.
	}

	err = c.client.WaitUntilStackDeleteCompleteWithContext(context.Background(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, waiters...)
	if err != nil {
		return fmt.Errorf("wait until stack %s delete is complete: %w", stackName, err)
	}
	return nil
}

func (c *CloudFormation) describe(name string) (*cloudformation.Stack, error) {
	out, err := c.client.DescribeStacks(&cloudformation.DescribeStacksInput{
		StackName: aws.String(name),
	})
	if err != nil {
		if stackDoesNotExist(err) {
			return nil, &ErrStackNotFound{name: name}
		}
		return nil, fmt.Errorf("describe stack %s: %w", name, err)
	}
	if len(out.Stacks) == 0 {
		return nil, &ErrStackNotFound{name: name}
	}
	return out.Stacks[0], nil
}

func (c *CloudFormation) create(stack *Stack) error {
	return c.deployChangeSet(stack, cloudformation.ChangeSetTypeCreate)
}

func (c *CloudFormation) update(stack *Stack) error {
	return c.deployChangeSet(stack, cloudformation.ChangeSetTypeUpdate)
}

func (c *CloudFormation) deployChangeSet(stack *Stack, changeSetType string) error {
	cs, err := newChangeSet(c.client, stack.name)
	if err != nil {
		return err
	}
	if err := cs.create(stack.stackConfig, changeSetType); err != nil {
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

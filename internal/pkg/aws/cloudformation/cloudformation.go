// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudformation provides a client to make API requests to AWS CloudFormation.
package cloudformation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

type eventMatcher func(*cloudformation.StackEvent) bool

var eventErrorStates = []string{
	cloudformation.ResourceStatusCreateFailed,
	cloudformation.ResourceStatusDeleteFailed,
	cloudformation.ResourceStatusImportFailed,
	cloudformation.ResourceStatusUpdateFailed,
	cloudformation.ResourceStatusImportRollbackFailed,
}

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
	descr, err := c.Describe(stack.Name)
	if err != nil {
		var stackNotFound *ErrStackNotFound
		if !errors.As(err, &stackNotFound) {
			return err
		}
		// If the stack does not exist, create it.
		return c.create(stack)
	}
	status := StackStatus(aws.StringValue(descr.StackStatus))
	if status.requiresCleanup() {
		// If the stack exists, but failed to create, we'll clean it up and then re-create it.
		if err := c.Delete(stack.Name); err != nil {
			return fmt.Errorf("cleanup previously failed stack %s: %w", stack.Name, err)
		}
		return c.create(stack)
	}
	if status.InProgress() {
		return &ErrStackUpdateInProgress{
			Name: stack.Name,
		}
	}
	return &ErrStackAlreadyExists{
		Name:  stack.Name,
		Stack: descr,
	}
}

// CreateAndWait calls Create and then WaitForCreate.
func (c *CloudFormation) CreateAndWait(stack *Stack) error {
	if err := c.Create(stack); err != nil {
		return err
	}
	return c.WaitForCreate(stack.Name)
}

// WaitForCreate blocks until the stack is created or until the max attempt window expires.
func (c *CloudFormation) WaitForCreate(stackName string) error {
	err := c.client.WaitUntilStackCreateCompleteWithContext(context.Background(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, waiters...)
	if err != nil {
		return fmt.Errorf("wait until stack %s create is complete: %w", stackName, err)
	}
	return nil
}

// Update updates an existing CloudFormation with the new configuration.
// If there are no changes for the stack, deletes the empty change set and returns ErrChangeSetEmpty.
func (c *CloudFormation) Update(stack *Stack) error {
	descr, err := c.Describe(stack.Name)
	if err != nil {
		return err
	}
	status := StackStatus(aws.StringValue(descr.StackStatus))
	if status.InProgress() {
		return &ErrStackUpdateInProgress{
			Name: stack.Name,
		}
	}
	return c.update(stack)
}

// UpdateAndWait calls Update and then blocks until the stack is updated or until the max attempt window expires.
func (c *CloudFormation) UpdateAndWait(stack *Stack) error {
	if err := c.Update(stack); err != nil {
		return err
	}
	return c.WaitForUpdate(stack.Name)
}

// WaitForUpdate blocks until the stack is updated or until the max attempt window expires.
func (c *CloudFormation) WaitForUpdate(stackName string) error {
	err := c.client.WaitUntilStackUpdateCompleteWithContext(context.Background(), &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, waiters...)
	if err != nil {
		return fmt.Errorf("wait until stack %s update is complete: %w", stackName, err)
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
	return c.deleteAndWait(&cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
	})
}

// DeleteAndWaitWithRoleARN is DeleteAndWait but with a role ARN that AWS CloudFormation assumes to delete the stack.
func (c *CloudFormation) DeleteAndWaitWithRoleARN(stackName, roleARN string) error {
	return c.deleteAndWait(&cloudformation.DeleteStackInput{
		StackName: aws.String(stackName),
		RoleARN:   aws.String(roleARN),
	})
}

// Describe returns a description of an existing stack.
// If the stack does not exist, returns ErrStackNotFound.
func (c *CloudFormation) Describe(name string) (*StackDescription, error) {
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
	descr := StackDescription(*out.Stacks[0])
	return &descr, nil
}

// TemplateBody returns the template body of an existing stack.
// If the stack does not exist, returns ErrStackNotFound.
func (c *CloudFormation) TemplateBody(name string) (string, error) {
	out, err := c.client.GetTemplate(&cloudformation.GetTemplateInput{
		StackName: aws.String(name),
	})
	if err != nil {
		if stackDoesNotExist(err) {
			return "", &ErrStackNotFound{name: name}
		}
		return "", fmt.Errorf("get template %s: %w", name, err)
	}
	return aws.StringValue(out.TemplateBody), nil
}

// Events returns the list of stack events in **chronological** order.
func (c *CloudFormation) Events(stackName string) ([]StackEvent, error) {
	return c.events(stackName, func(in *cloudformation.StackEvent) bool { return true })
}

func (c *CloudFormation) events(stackName string, match eventMatcher) ([]StackEvent, error) {
	var nextToken *string
	var events []StackEvent
	for {
		out, err := c.client.DescribeStackEvents(&cloudformation.DescribeStackEventsInput{
			NextToken: nextToken,
			StackName: aws.String(stackName),
		})
		if err != nil {
			return nil, fmt.Errorf("describe stack events for stack %s: %w", stackName, err)
		}
		for _, event := range out.StackEvents {
			if match(event) {
				events = append(events, StackEvent(*event))
			}
		}
		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	// Reverse the events so that they're returned in chronological order.
	// Taken from https://github.com/golang/go/wiki/SliceTricks#reversing.
	for i := len(events)/2 - 1; i >= 0; i-- {
		opp := len(events) - 1 - i
		events[i], events[opp] = events[opp], events[i]
	}
	return events, nil
}

// ListStacksWithPrefix returns all the stacks in the current AWS account.
func (c *CloudFormation) ListStacksWithPrefix(prefix string) ([]StackDescription, error) {
	return c.listStacks(prefix)
}

// ErrorEvents returns the list of events with "failed" status in **chronological order**
func (c *CloudFormation) ErrorEvents(stackName string) ([]StackEvent, error) {
	return c.events(stackName, func(in *cloudformation.StackEvent) bool {
		for _, status := range eventErrorStates {
			if aws.StringValue(in.ResourceStatus) == status {
				return true
			}
		}
		return false
	})
}

func (c *CloudFormation) create(stack *Stack) error {
	cs, err := newCreateChangeSet(c.client, stack.Name)
	if err != nil {
		return err
	}
	return cs.createAndExecute(stack.stackConfig)
}

func (c *CloudFormation) update(stack *Stack) error {
	cs, err := newUpdateChangeSet(c.client, stack.Name)
	if err != nil {
		return err
	}
	return cs.createAndExecute(stack.stackConfig)
}

func (c *CloudFormation) deleteAndWait(in *cloudformation.DeleteStackInput) error {
	_, err := c.client.DeleteStack(in)
	if err != nil {
		if !stackDoesNotExist(err) {
			return fmt.Errorf("delete stack %s: %w", aws.StringValue(in.StackName), err)
		}
		return nil // If the stack is already deleted, don't wait for it.
	}

	err = c.client.WaitUntilStackDeleteCompleteWithContext(context.Background(), &cloudformation.DescribeStacksInput{
		StackName: in.StackName,
	}, waiters...)
	if err != nil {
		return fmt.Errorf("wait until stack %s delete is complete: %w", aws.StringValue(in.StackName), err)
	}
	return nil
}

func (c *CloudFormation) listStacks(prefix string) ([]StackDescription, error) {
	var nextToken *string
	var summaries []StackDescription
	for {
		out, err := c.client.DescribeStacks(&cloudformation.DescribeStacksInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("list stacks: %w", err)
		}

		for _, summary := range out.Stacks {
			if strings.HasPrefix(*summary.StackName, prefix) {
				summaries = append(summaries, StackDescription(*summary))
			}
		}
		nextToken = out.NextToken
		if nextToken == nil {
			break
		}
	}
	return summaries, nil
}

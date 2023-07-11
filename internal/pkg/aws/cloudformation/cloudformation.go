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

type eventMatcher func(*cloudformation.StackEvent) bool

var eventErrorStates = []string{
	cloudformation.ResourceStatusCreateFailed,
	cloudformation.ResourceStatusDeleteFailed,
	cloudformation.ResourceStatusImportFailed,
	cloudformation.ResourceStatusUpdateFailed,
	cloudformation.ResourceStatusImportRollbackFailed,
}

var waiters = []request.WaiterOption{
	request.WithWaiterDelay(request.ConstantWaiterDelay(5 * time.Second)), // How long to wait in between poll cfn for updates.
	request.WithWaiterMaxAttempts(1080),                                   // Wait for at most 90 mins for any cfn action.
}

// CloudFormation represents a client to make requests to AWS CloudFormation.
type CloudFormation struct {
	client
}

// New creates a new CloudFormation client.
func New(s *session.Session) *CloudFormation {
	return &CloudFormation{
		cloudformation.New(s),
	}
}

// Create deploys a new CloudFormation stack using Change Sets.
// If the stack already exists in a failed state, deletes the stack and re-creates it.
func (c *CloudFormation) Create(stack *Stack) (changeSetID string, err error) {
	descr, err := c.Describe(stack.Name)
	if err != nil {
		var stackNotFound *ErrStackNotFound
		if !errors.As(err, &stackNotFound) {
			return "", err
		}
		// If the stack does not exist, create it.
		return c.create(stack)
	}
	status := StackStatus(aws.StringValue(descr.StackStatus))
	if status.requiresCleanup() {
		// If the stack exists, but failed to create, we'll clean it up and then re-create it.
		if err := c.DeleteAndWait(stack.Name); err != nil {
			return "", fmt.Errorf("clean up previously failed stack %s: %w", stack.Name, err)
		}
		return c.create(stack)
	}
	if status.InProgress() {
		return "", &ErrStackUpdateInProgress{
			Name: stack.Name,
		}
	}
	return "", &ErrStackAlreadyExists{
		Name:  stack.Name,
		Stack: descr,
	}
}

// CreateAndWait calls Create and then WaitForCreate.
func (c *CloudFormation) CreateAndWait(stack *Stack) error {
	if _, err := c.Create(stack); err != nil {
		return err
	}
	return c.WaitForCreate(context.Background(), stack.Name)
}

// DescribeChangeSet gathers and returns all changes for a change set.
func (c *CloudFormation) DescribeChangeSet(changeSetID, stackName string) (*ChangeSetDescription, error) {
	cs := &changeSet{name: changeSetID, stackName: stackName, client: c.client}
	out, err := cs.describe()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// WaitForCreate blocks until the stack is created or until the max attempt window expires.
func (c *CloudFormation) WaitForCreate(ctx context.Context, stackName string) error {
	err := c.client.WaitUntilStackCreateCompleteWithContext(ctx, &cloudformation.DescribeStacksInput{
		StackName: aws.String(stackName),
	}, waiters...)
	if err != nil {
		return fmt.Errorf("wait until stack %s create is complete: %w", stackName, err)
	}
	return nil
}

// Update updates an existing CloudFormation with the new configuration.
// If there are no changes for the stack, deletes the empty change set and returns ErrChangeSetEmpty.
func (c *CloudFormation) Update(stack *Stack) (changeSetID string, err error) {
	descr, err := c.Describe(stack.Name)
	if err != nil {
		return "", err
	}
	status := StackStatus(aws.StringValue(descr.StackStatus))
	if status.InProgress() {
		return "", &ErrStackUpdateInProgress{
			Name: stack.Name,
		}
	}
	return c.update(stack)
}

// UpdateAndWait calls Update and then blocks until the stack is updated or until the max attempt window expires.
func (c *CloudFormation) UpdateAndWait(stack *Stack) error {
	if _, err := c.Update(stack); err != nil {
		return err
	}
	return c.WaitForUpdate(context.Background(), stack.Name)
}

// WaitForUpdate blocks until the stack is updated or until the max attempt window expires.
func (c *CloudFormation) WaitForUpdate(ctx context.Context, stackName string) error {
	err := c.client.WaitUntilStackUpdateCompleteWithContext(ctx, &cloudformation.DescribeStacksInput{
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

// Exists returns true if the CloudFormation stack exists, false otherwise.
// If an error occurs for another reason than ErrStackNotFound, then returns the error.
func (c *CloudFormation) Exists(name string) (bool, error) {
	if _, err := c.Describe(name); err != nil {
		var notFound *ErrStackNotFound
		if errors.As(err, &notFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// MetadataOpts sets up optional parameters for Metadata function.
type MetadataOpts *cloudformation.GetTemplateSummaryInput

// MetadataWithStackName sets up the stack name for cloudformation.GetTemplateSummaryInput.
func MetadataWithStackName(name string) MetadataOpts {
	return &cloudformation.GetTemplateSummaryInput{
		StackName: aws.String(name),
	}
}

// MetadataWithStackSetName sets up the stack set name for cloudformation.GetTemplateSummaryInput.
func MetadataWithStackSetName(name string) MetadataOpts {
	return &cloudformation.GetTemplateSummaryInput{
		StackSetName: aws.String(name),
	}
}

// Metadata returns the Metadata property of the CloudFormation stack(set)'s template.
// If the stack does not exist, returns ErrStackNotFound.
func (c *CloudFormation) Metadata(opt MetadataOpts) (string, error) {
	out, err := c.GetTemplateSummary(opt)
	if err != nil {
		if stackDoesNotExist(err) {
			if aws.StringValue(opt.StackName) != "" {
				return "", &ErrStackNotFound{name: aws.StringValue(opt.StackName)}
			}
			return "", &ErrStackNotFound{name: aws.StringValue(opt.StackSetName)}
		}
		return "", fmt.Errorf("get template summary: %w", err)
	}
	return aws.StringValue(out.Metadata), nil
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

// TemplateBodyFromChangeSet returns the template body of a stack based on a change set.
// If the stack does not exist, then returns ErrStackNotFound.
func (c *CloudFormation) TemplateBodyFromChangeSet(changeSetID, stackName string) (string, error) {
	out, err := c.client.GetTemplate(&cloudformation.GetTemplateInput{
		ChangeSetName: aws.String(changeSetID),
		StackName:     aws.String(stackName),
	})
	if err != nil {
		if stackDoesNotExist(err) {
			return "", &ErrStackNotFound{name: stackName}
		}
		return "", fmt.Errorf("get template for stack %s and change set %s: %w", stackName, changeSetID, err)
	}
	return aws.StringValue(out.TemplateBody), nil
}

// Outputs returns the outputs of a stack description.
func (c *CloudFormation) Outputs(stack *Stack) (map[string]string, error) {
	stackDescription, err := c.Describe(stack.Name)
	if err != nil {
		return nil, fmt.Errorf("retrieve outputs of stack description: %w", err)
	}
	outputs := make(map[string]string)
	for _, output := range stackDescription.Outputs {
		outputs[aws.StringValue(output.OutputKey)] = aws.StringValue(output.OutputValue)
	}
	return outputs, nil
}

// Events returns the list of stack events in **chronological** order.
func (c *CloudFormation) Events(stackName string) ([]StackEvent, error) {
	return c.events(stackName, func(in *cloudformation.StackEvent) bool { return true })
}

// StackResources returns the list of resources created as part of a CloudFormation stack.
func (c *CloudFormation) StackResources(name string) ([]*StackResource, error) {
	out, err := c.DescribeStackResources(&cloudformation.DescribeStackResourcesInput{
		StackName: aws.String(name),
	})
	if err != nil {
		return nil, fmt.Errorf("describe resources for stack %s: %w", name, err)
	}
	var resources []*StackResource
	for _, r := range out.StackResources {
		if r == nil {
			continue
		}
		sr := StackResource(*r)
		resources = append(resources, &sr)
	}
	return resources, nil
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

// ListStacksWithTags returns all the stacks in the current AWS account and region with the specified matching
// tags. If a tag key is provided but the value is empty, the method will match tags with any value for the given key.
func (c *CloudFormation) ListStacksWithTags(tags map[string]string) ([]StackDescription, error) {
	match := makeTagMatcher(tags)

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
			stackTags := summary.Tags
			if match(stackTags) {
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

// CancelUpdateStack attempts to cancel the update for a CloudFormation stack specified by the stackName.
// Returns an error if failed to cancel CloudFormation stack update.
func (c *CloudFormation) CancelUpdateStack(stackName string) error {
	if _, err := c.client.CancelUpdateStack(&cloudformation.CancelUpdateStackInput{
		StackName: aws.String(stackName),
	}); err != nil {
		if !stackDoesNotExist(err) && !cancelUpdateStackNotInUpdateProgress(err) {
			return fmt.Errorf("cancel update stack: %w", err)
		}
	}
	return nil
}

func (c *CloudFormation) create(stack *Stack) (string, error) {
	cs, err := newCreateChangeSet(c.client, stack.Name)
	if err != nil {
		return "", err
	}
	if err := cs.createAndExecute(stack.stackConfig); err != nil {
		return "", err
	}
	return cs.name, nil
}

func (c *CloudFormation) update(stack *Stack) (string, error) {
	cs, err := newUpdateChangeSet(c.client, stack.Name)
	if err != nil {
		return "", err
	}
	if err := cs.createAndExecute(stack.stackConfig); err != nil {
		return "", err
	}
	return cs.name, nil
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

// makeTagMatcher takes a set of wanted tags and returns a function which matches if the given set of
// `cloudformation.Tag`s contains tags with all wanted keys and values
func makeTagMatcher(wantedTags map[string]string) func([]*cloudformation.Tag) bool {
	// Match all stacks if no desired tags are specified.
	if len(wantedTags) == 0 {
		return func([]*cloudformation.Tag) bool { return true }
	}

	return func(tags []*cloudformation.Tag) bool {
		// Define a map to determine whether each wanted tag is a match.
		tagsMatched := make(map[string]bool, len(wantedTags))

		// Populate the hash set and match map
		for k := range wantedTags {
			tagsMatched[k] = false
		}

		// Loop over all tags on the stack and decide whether they match any of the wanted tags.
		for _, tag := range tags {
			tagKey := aws.StringValue(tag.Key)
			tagValue := aws.StringValue(tag.Value)
			if wantedTags[tagKey] == tagValue || wantedTags[tagKey] == "" {
				tagsMatched[tagKey] = true
			}
		}

		// Only return true if all wanted tags are present and match in the stack's tags.
		for _, v := range tagsMatched {
			if !v {
				return false
			}
		}

		return true
	}
}

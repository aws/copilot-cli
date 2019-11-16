// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/cloudformation/cloudformationiface"
	"github.com/google/uuid"
)

// Status reasons that can occur if the change set execution status is "FAILED".
const (
	noChangesReason = "NO_CHANGES_REASON"
	noUpdatesReason = "NO_UPDATES_REASON"
)

// changeSet represents a CloudFormation Change Set.
// See https://aws.amazon.com/blogs/aws/new-change-sets-for-aws-cloudformation/
type changeSet struct {
	name            string // required
	stackID         string // required
	executionStatus string
	statusReason    string
	changes         []*cloudformation.Change

	c       cloudformationiface.CloudFormationAPI
	waiters []request.WaiterOption
}

func (set *changeSet) String() string {
	return fmt.Sprintf("name=%s, stackID=%s", set.name, set.stackID)
}

func (set *changeSet) waitForCreation() error {
	describeChangeSetInput := &cloudformation.DescribeChangeSetInput{
		ChangeSetName: aws.String(set.name),
		StackName:     aws.String(set.stackID),
	}

	if err := set.c.WaitUntilChangeSetCreateCompleteWithContext(context.Background(), describeChangeSetInput, set.waiters...); err != nil {
		return fmt.Errorf("failed to wait for changeSet creation %s: %w", set, err)
	}
	return nil
}

// describe updates the change set with its latest values.
func (set *changeSet) describe() error {
	var executionStatus, statusReason string
	var changes []*cloudformation.Change
	var nextToken *string
	for {
		out, err := set.c.DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
			ChangeSetName: aws.String(set.name),
			StackName:     aws.String(set.stackID),
			NextToken:     nextToken,
		})
		if err != nil {
			return fmt.Errorf("failed to describe changeSet %s: %w", set, err)
		}
		executionStatus = aws.StringValue(out.ExecutionStatus)
		statusReason = aws.StringValue(out.StatusReason)
		changes = append(changes, out.Changes...)
		nextToken = out.NextToken

		if nextToken == nil { // no more results left
			break
		}
	}
	set.executionStatus = executionStatus
	set.statusReason = statusReason
	set.changes = changes
	return nil
}

func (set *changeSet) execute() error {
	if err := set.describe(); err != nil {
		return err
	}
	if set.executionStatus != cloudformation.ExecutionStatusAvailable {
		// Ignore execute request if the change set does not contain any modifications.
		if set.statusReason == noChangesReason {
			return nil
		}
		if set.statusReason == noUpdatesReason {
			return nil
		}
		return &ErrNotExecutableChangeSet{
			set: set,
		}
	}
	if _, err := set.c.ExecuteChangeSet(&cloudformation.ExecuteChangeSetInput{
		ChangeSetName: aws.String(set.name),
		StackName:     aws.String(set.stackID),
	}); err != nil {
		return fmt.Errorf("failed to execute changeSet %s: %w", set, err)
	}
	return nil
}

// createChangeSetOpt is a functional option to add additional settings to a CreateChangeSetInput.
type createChangeSetOpt func(in *cloudformation.CreateChangeSetInput)

func createChangeSetInput(stackName, templateBody string, options ...createChangeSetOpt) (*cloudformation.CreateChangeSetInput, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("failed to generate random id for changeSet: %w", err)
	}

	// The change set name must match the regex [a-zA-Z][-a-zA-Z0-9]*. The generated UUID can start with a number,
	// by prefixing the uuid with a word we guarantee that we start with a letter.
	name := fmt.Sprintf("%s-%s", "ecscli", id.String())

	in := &cloudformation.CreateChangeSetInput{
		Capabilities: aws.StringSlice([]string{
			cloudformation.CapabilityCapabilityIam,
			cloudformation.CapabilityCapabilityNamedIam,
		}),
		ChangeSetName: aws.String(name),
		StackName:     aws.String(stackName),
		TemplateBody:  aws.String(templateBody),
	}
	for _, option := range options {
		option(in)
	}
	return in, nil
}

func withParameters(params []*cloudformation.Parameter) createChangeSetOpt {
	return func(in *cloudformation.CreateChangeSetInput) {
		in.Parameters = params
	}
}

func withChangeSetType(csType string) createChangeSetOpt {
	return func(in *cloudformation.CreateChangeSetInput) {
		in.ChangeSetType = aws.String(csType)
	}
}

func withTags(tags []*cloudformation.Tag) createChangeSetOpt {
	return func(in *cloudformation.CreateChangeSetInput) {
		in.Tags = tags
	}
}

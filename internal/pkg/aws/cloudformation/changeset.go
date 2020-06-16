// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/google/uuid"
)

const (
	// The change set name must match the regex [a-zA-Z][-a-zA-Z0-9]*. The generated UUID can start with a number,
	// by prefixing the uuid with a word we guarantee that we start with a letter.
	fmtChangeSetName = "ecscli-%s"

	// Status reasons that can occur if the change set execution status is "FAILED".
	noChangesReason = "NO_CHANGES_REASON"
	noUpdatesReason = "NO_UPDATES_REASON"
)

type changeSetType int

func (t changeSetType) String() string {
	switch t {
	case updateChangeSetType:
		return cloudformation.ChangeSetTypeUpdate
	default:
		return cloudformation.ChangeSetTypeCreate
	}
}

const (
	createChangeSetType changeSetType = iota
	updateChangeSetType
)

type changeSet struct {
	name      string
	stackName string
	csType    changeSetType
	client    changeSetAPI
}

type changeSetDescription struct {
	executionStatus string
	statusReason    string
	changes         []*cloudformation.Change
}

func newCreateChangeSet(cfnClient changeSetAPI, stackName string) (*changeSet, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("generate random id for Change Set: %w", err)
	}

	return &changeSet{
		name:      fmt.Sprintf(fmtChangeSetName, id.String()),
		stackName: stackName,
		csType:    createChangeSetType,

		client: cfnClient,
	}, nil
}

func newUpdateChangeSet(cfnClient changeSetAPI, stackName string) (*changeSet, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("generate random id for Change Set: %w", err)
	}

	return &changeSet{
		name:      fmt.Sprintf(fmtChangeSetName, id.String()),
		stackName: stackName,
		csType:    updateChangeSetType,

		client: cfnClient,
	}, nil
}

func (cs *changeSet) String() string {
	return fmt.Sprintf("change set %s for stack %s", cs.name, cs.stackName)
}

// create creates a Change Set and waits until it's created.
func (cs *changeSet) create(conf *stackConfig) error {
	_, err := cs.client.CreateChangeSet(&cloudformation.CreateChangeSetInput{
		ChangeSetName: aws.String(cs.name),
		StackName:     aws.String(cs.stackName),
		ChangeSetType: aws.String(cs.csType.String()),
		TemplateBody:  aws.String(conf.Template),
		Parameters:    conf.Parameters,
		Tags:          conf.Tags,
		RoleARN:       conf.RoleARN,
		Capabilities: aws.StringSlice([]string{
			cloudformation.CapabilityCapabilityIam,
			cloudformation.CapabilityCapabilityNamedIam,
			cloudformation.CapabilityCapabilityAutoExpand,
		}),
	})
	if err != nil {
		return fmt.Errorf("create %s: %w", cs, err)
	}
	err = cs.client.WaitUntilChangeSetCreateCompleteWithContext(context.Background(), &cloudformation.DescribeChangeSetInput{
		ChangeSetName: aws.String(cs.name),
		StackName:     aws.String(cs.stackName),
	}, waiters...)
	if err != nil {
		return fmt.Errorf("wait for creation of %s: %w", cs, err)
	}
	return nil
}

// describe collects all the changes and statuses that the change set will apply and returns them.
func (cs *changeSet) describe() (*changeSetDescription, error) {
	var executionStatus, statusReason string
	var changes []*cloudformation.Change
	var nextToken *string
	for {
		out, err := cs.client.DescribeChangeSet(&cloudformation.DescribeChangeSetInput{
			ChangeSetName: aws.String(cs.name),
			StackName:     aws.String(cs.stackName),
			NextToken:     nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("describe %s: %w", cs, err)
		}
		executionStatus = aws.StringValue(out.ExecutionStatus)
		statusReason = aws.StringValue(out.StatusReason)
		changes = append(changes, out.Changes...)
		nextToken = out.NextToken

		if nextToken == nil { // no more results left
			break
		}
	}
	return &changeSetDescription{
		executionStatus: executionStatus,
		statusReason:    statusReason,
		changes:         changes,
	}, nil
}

// execute executes a created change set.
func (cs *changeSet) execute() error {
	descr, err := cs.describe()
	if err != nil {
		return err
	}
	if descr.executionStatus != cloudformation.ExecutionStatusAvailable {
		// Ignore execute request if the change set does not contain any modifications.
		if descr.statusReason == noChangesReason {
			return nil
		}
		if descr.statusReason == noUpdatesReason {
			return nil
		}
		return &errChangeSetNotExecutable{
			cs:    cs,
			descr: descr,
		}
	}
	_, err = cs.client.ExecuteChangeSet(&cloudformation.ExecuteChangeSetInput{
		ChangeSetName: aws.String(cs.name),
		StackName:     aws.String(cs.stackName),
	})
	if err != nil {
		return fmt.Errorf("execute %s: %w", cs, err)
	}
	return nil
}

// createAndExecute calls create and then execute.
// If the change set is empty, returns a ErrChangeSetEmpty.
func (cs *changeSet) createAndExecute(conf *stackConfig) error {
	if err := cs.create(conf); err != nil {
		// It's possible that there are no changes between the previous and proposed stack change sets.
		// We make a call to describe the change set to see if that is indeed the case and handle it gracefully.
		descr, descrErr := cs.describe()
		if descrErr != nil {
			return fmt.Errorf("check if changeset is empty: %v: %w", err, descrErr)
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

// delete removes the change set.
func (cs *changeSet) delete() error {
	_, err := cs.client.DeleteChangeSet(&cloudformation.DeleteChangeSetInput{
		ChangeSetName: aws.String(cs.name),
		StackName:     aws.String(cs.stackName),
	})
	if err != nil {
		return fmt.Errorf("delete %s: %w", cs, err)
	}
	return nil
}

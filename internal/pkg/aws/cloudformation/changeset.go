// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/google/uuid"
)

const (
	// The change set name must match the regex [a-zA-Z][-a-zA-Z0-9]*. The generated UUID can start with a number,
	// by prefixing the uuid with a word we guarantee that we start with a letter.
	fmtChangeSetName = "copilot-%s"

	// Status reasons that can occur if the change set execution status is "FAILED".
	noChangesReason = "NO_CHANGES_REASON"
	noUpdatesReason = "NO_UPDATES_REASON"
)

// ChangeSetDescription is the output of the DescribeChangeSet action.
type ChangeSetDescription struct {
	ExecutionStatus string
	StatusReason    string
	CreationTime    time.Time
	Changes         []*cloudformation.Change
}

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

// create creates a ChangeSet, waits until it's created, and returns the ChangeSet ID on success.
func (cs *changeSet) create(conf *stackConfig) error {
	input := &cloudformation.CreateChangeSetInput{
		ChangeSetName:       aws.String(cs.name),
		StackName:           aws.String(cs.stackName),
		ChangeSetType:       aws.String(cs.csType.String()),
		Parameters:          conf.Parameters,
		Tags:                conf.Tags,
		RoleARN:             conf.RoleARN,
		IncludeNestedStacks: aws.Bool(true),
		Capabilities: aws.StringSlice([]string{
			cloudformation.CapabilityCapabilityIam,
			cloudformation.CapabilityCapabilityNamedIam,
			cloudformation.CapabilityCapabilityAutoExpand,
		}),
	}
	if conf.TemplateBody != "" {
		input.TemplateBody = aws.String(conf.TemplateBody)
	}
	if conf.TemplateURL != "" {
		input.TemplateURL = aws.String(conf.TemplateURL)
	}

	out, err := cs.client.CreateChangeSet(input)
	if err != nil {
		return fmt.Errorf("create %s: %w", cs, err)
	}
	err = cs.client.WaitUntilChangeSetCreateCompleteWithContext(context.Background(), &cloudformation.DescribeChangeSetInput{
		ChangeSetName: out.Id,
	}, waiters...)
	if err != nil {
		return fmt.Errorf("wait for creation of %s: %w", cs, err)
	}
	// Since the ChangeSet creation succeeded, use the full ARN instead of the name.
	// Using the full ID is essential in case the ChangeSet execution status is obsolete.
	// If we call DescribeChangeSet using the ChangeSet name and Stack name on an obsolete changeset, the results is empty.
	// On the other hand, if you DescribeChangeSet using the full ID then the ChangeSet summary is retrieved correctly.
	cs.name = aws.StringValue(out.Id)
	return nil
}

// describe collects all the changes and statuses that the change set will apply and returns them.
func (cs *changeSet) describe() (*ChangeSetDescription, error) {
	var executionStatus, statusReason string
	var creationTime time.Time
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
		creationTime = aws.TimeValue(out.CreationTime)
		changes = append(changes, out.Changes...)
		nextToken = out.NextToken

		if nextToken == nil { // no more results left
			break
		}
	}
	return &ChangeSetDescription{
		ExecutionStatus: executionStatus,
		StatusReason:    statusReason,
		CreationTime:    creationTime,
		Changes:         changes,
	}, nil
}

// execute executes a created change set.
func (cs *changeSet) execute() error {
	descr, err := cs.describe()
	if err != nil {
		return err
	}
	if descr.ExecutionStatus != cloudformation.ExecutionStatusAvailable {
		// Ignore execute request if the change set does not contain any modifications.
		if descr.StatusReason == noChangesReason {
			return nil
		}
		if descr.StatusReason == noUpdatesReason {
			return nil
		}
		return &ErrChangeSetNotExecutable{
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

// executeWithNoRollback executes a created change set without automatic stack rollback.
func (cs *changeSet) executeWithNoRollback() error {
	descr, err := cs.describe()
	if err != nil {
		return err
	}
	if descr.ExecutionStatus != cloudformation.ExecutionStatusAvailable {
		// Ignore execute request if the change set does not contain any modifications.
		if descr.StatusReason == noChangesReason {
			return nil
		}
		if descr.StatusReason == noUpdatesReason {
			return nil
		}
		return &ErrChangeSetNotExecutable{
			cs:    cs,
			descr: descr,
		}
	}
	_, err = cs.client.ExecuteChangeSet(&cloudformation.ExecuteChangeSetInput{
		ChangeSetName:   aws.String(cs.name),
		StackName:       aws.String(cs.stackName),
		DisableRollback: aws.Bool(true),
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
		// The change set was empty - so we clean it up. The status reason will be like
		// "The submitted information didn't contain changes. Submit different information to create a change set."
		// We try to clean up the change set because there's a limit on the number
		// of failed change sets a customer can have on a particular stack.
		// See https://cloudonaut.io/aws-cli-cloudformation-deploy-limit-exceeded/.
		if len(descr.Changes) == 0 && strings.Contains(descr.StatusReason, "didn't contain changes") {
			_ = cs.delete()
			return &ErrChangeSetEmpty{
				cs: cs,
			}
		}
		return fmt.Errorf("%w: %s", err, descr.StatusReason)
	}
	if conf.DisableRollback {
		return cs.executeWithNoRollback()
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

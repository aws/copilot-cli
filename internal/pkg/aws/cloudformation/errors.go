// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/awserr"
)

// ErrChangeSetEmpty occurs when the change set does not contain any new or updated resources.
type ErrChangeSetEmpty struct {
	cs *changeSet
}

func (e *ErrChangeSetEmpty) Error() string {
	return fmt.Sprintf("change set with name %s for stack %s has no changes", e.cs.name, e.cs.stackName)
}

// NewMockErrChangeSetEmpty creates a mock ErrChangeSetEmpty.
func NewMockErrChangeSetEmpty() *ErrChangeSetEmpty {
	return &ErrChangeSetEmpty{
		cs: &changeSet{
			name:      "mockChangeSet",
			stackName: "mockStack",
		},
	}
}

// ErrStackAlreadyExists occurs when a CloudFormation stack already exists with a given name.
type ErrStackAlreadyExists struct {
	Name  string
	Stack *StackDescription
}

func (e *ErrStackAlreadyExists) Error() string {
	return fmt.Sprintf("stack %s already exists", e.Name)
}

// ErrStackNotFound occurs when a CloudFormation stack does not exist.
type ErrStackNotFound struct {
	name string
}

func (e *ErrStackNotFound) Error() string {
	return fmt.Sprintf("stack named %s cannot be found", e.name)
}

// ErrChangeSetNotExecutable occurs when the change set cannot be executed.
type ErrChangeSetNotExecutable struct {
	cs    *changeSet
	descr *ChangeSetDescription
}

func (e *ErrChangeSetNotExecutable) Error() string {
	return fmt.Sprintf("execute change set %s for stack %s because status is %s with reason %s", e.cs.name, e.cs.stackName, e.descr.ExecutionStatus, e.descr.StatusReason)
}

// ErrStackUpdateInProgress occurs when we try to update a stack that's already being updated.
type ErrStackUpdateInProgress struct {
	Name string
}

func (e *ErrStackUpdateInProgress) Error() string {
	return fmt.Sprintf("stack %s is currently being updated and cannot be deployed to", e.Name)
}

// stackDoesNotExist returns true if the underlying error is a stack doesn't exist.
func stackDoesNotExist(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case "ValidationError":
			// A ValidationError occurs if we describe a stack which doesn't exist.
			if strings.Contains(aerr.Message(), "does not exist") {
				return true
			}
		}
	}
	return false
}

// cancelUpdateStackNotInUpdateProgress returns true if the underlying error is CancelUpdateStack
// cannot be called for a stack that is not in UPDATE_IN_PROGRESS state.
func cancelUpdateStackNotInUpdateProgress(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case "ValidationError":
			if strings.Contains(aerr.Message(), "CancelUpdateStack cannot be called from current stack status") {
				return true
			}
		}
	}
	return false
}

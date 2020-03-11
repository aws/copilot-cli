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

// ErrStackAlreadyExists occurs when a CloudFormation stack already exists with a given name.
type ErrStackAlreadyExists struct {
	name string
}

func (e *ErrStackAlreadyExists) Error() string {
	return fmt.Sprintf("stack %s already exists", e.name)
}

// errStackNotFound occurs when a particular CloudFormation stack does not exist.
type errStackNotFound struct {
	name string
}

func (e *errStackNotFound) Error() string {
	return fmt.Sprintf("stack named %s cannot be found", e.name)
}

// errChangeSetNotExecutable occurs when the change set cannot be executed.
type errChangeSetNotExecutable struct {
	cs    *changeSet
	descr *changeSetDescription
}

func (e *errChangeSetNotExecutable) Error() string {
	return fmt.Sprintf("execute change set %s for stack %s because status is %s with reason %s", e.cs.name, e.cs.stackName, e.descr.executionStatus, e.descr.statusReason)
}

// errStackUpdateInProgress occurs when we try to update a stack that's already being updated.
type errStackUpdateInProgress struct {
	name string
}

func (e *errStackUpdateInProgress) Error() string {
	return fmt.Sprintf("stack %s is currently being updated and cannot be deployed to", e.name)
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

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stackset

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// ErrStackSetOutOfDate occurs when we try to read and then update a StackSet but between reading it
// and actually updating it, someone else either started or completed an update.
type ErrStackSetOutOfDate struct {
	name      string
	parentErr error
}

func (e *ErrStackSetOutOfDate) Error() string {
	return fmt.Sprintf("stack set %q update was out of date (feel free to try again): %v", e.name, e.parentErr)
}

// ErrStackSetNotFound occurs when a stack set with the given name does not exist.
type ErrStackSetNotFound struct {
	name string
}

// Error implements the error interface.
func (e *ErrStackSetNotFound) Error() string {
	return fmt.Sprintf("stack set %q not found", e.name)
}

// IsEmpty reports whether this error is occurs on an empty cloudformation resource.
func (e *ErrStackSetNotFound) IsEmpty() bool {
	return true
}

// ErrStackSetInstancesNotFound occurs when a stack set operation should be applied to instances but they don't exist.
type ErrStackSetInstancesNotFound struct {
	name string
}

// Error implements the error interface.
func (e *ErrStackSetInstancesNotFound) Error() string {
	return fmt.Sprintf("stack set %q has no instances", e.name)
}

// IsEmpty reports whether this error is occurs on an empty cloudformation resource.
func (e *ErrStackSetInstancesNotFound) IsEmpty() bool {
	return true
}

// isAlreadyExistingStackSet returns true if the underlying error is a stack already exists error.
func isAlreadyExistingStackSet(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case cloudformation.ErrCodeNameAlreadyExistsException:
			return true
		}
	}
	return false
}

// isOutdatedStackSet returns true if the underlying error is because the operation was already performed.
func isOutdatedStackSet(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case cloudformation.ErrCodeOperationIdAlreadyExistsException, cloudformation.ErrCodeOperationInProgressException, cloudformation.ErrCodeStaleRequestException:
			return true
		}
	}
	return false
}

// isNotFoundStackSet returns true if the stack set does not exist.
func isNotFoundStackSet(err error) bool {
	if aerr, ok := err.(awserr.Error); ok {
		switch aerr.Code() {
		case cloudformation.ErrCodeStackSetNotFoundException:
			return true
		}
	}
	return false
}

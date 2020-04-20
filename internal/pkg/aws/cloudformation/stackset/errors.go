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
	stackSetName string
	parentErr    error
}

func (e *ErrStackSetOutOfDate) Error() string {
	return fmt.Sprintf("stack set %s update was out of date (feel free to try again): %v", e.stackSetName, e.parentErr)
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

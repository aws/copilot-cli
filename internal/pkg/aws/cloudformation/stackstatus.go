// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"strings"

	"github.com/aws/aws-sdk-go/service/cloudformation"
)

var (
	successStackStatuses = []string{
		cloudformation.StackStatusCreateComplete,
		cloudformation.StackStatusDeleteComplete,
		cloudformation.StackStatusUpdateComplete,
		cloudformation.StackStatusUpdateCompleteCleanupInProgress,
		cloudformation.StackStatusImportComplete,
	}

	failureStackStatuses = []string{
		cloudformation.StackStatusCreateFailed,
		cloudformation.StackStatusDeleteFailed,
		cloudformation.ResourceStatusUpdateFailed,
		cloudformation.StackStatusRollbackInProgress,
		cloudformation.StackStatusRollbackComplete,
		cloudformation.StackStatusRollbackFailed,
		cloudformation.StackStatusUpdateRollbackComplete,
		cloudformation.StackStatusUpdateRollbackCompleteCleanupInProgress,
		cloudformation.StackStatusUpdateRollbackInProgress,
		cloudformation.StackStatusUpdateRollbackFailed,
		cloudformation.ResourceStatusImportRollbackInProgress,
		cloudformation.ResourceStatusImportRollbackFailed,
	}

	// CompleteStackStatuses represents the stack was created, and can only get updated or deleted.
	CompleteStackStatuses = []string{
		cloudformation.StackStatusUpdateRollbackComplete,
		cloudformation.StackStatusCreateComplete,
		cloudformation.StackStatusUpdateComplete,
	}
)

// StackStatus represents the status of a stack.
type StackStatus string

// requiresCleanup returns true if the stack was created, but failed and should be deleted.
func (ss StackStatus) requiresCleanup() bool {
	return cloudformation.StackStatusRollbackComplete == string(ss) || cloudformation.StackStatusRollbackFailed == string(ss)
}

// InProgress returns true if the stack is currently being updated.
func (ss StackStatus) InProgress() bool {
	return strings.HasSuffix(string(ss), "IN_PROGRESS")
}

// UpsertInProgress returns true if the resource is updating or being created.
func (ss StackStatus) UpsertInProgress() bool {
	return ss == cloudformation.StackStatusCreateInProgress || ss == cloudformation.StackStatusUpdateInProgress
}

func (ss StackStatus) Success() bool {
	for _, success := range successStackStatuses {
		if string(ss) == success {
			return true
		}
	}
	return false
}

func (ss StackStatus) Failure() bool {
	for _, failure := range failureStackStatuses {
		if string(ss) == failure {
			return true
		}
	}
	return false
}

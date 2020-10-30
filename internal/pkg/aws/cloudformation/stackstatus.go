// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"strings"

	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// StackStatus represents the status of a stack.
type StackStatus string

// requiresCleanup returns true if the stack was created, but failed and should be deleted.
func (s StackStatus) requiresCleanup() bool {
	return cloudformation.StackStatusRollbackComplete == string(s) || cloudformation.StackStatusRollbackFailed == string(s)
}

// InProgress returns true if the stack is currently being updated.
func (s StackStatus) InProgress() bool {
	return strings.HasSuffix(string(s), "IN_PROGRESS")
}

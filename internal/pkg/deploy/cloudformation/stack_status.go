// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"strings"

	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// StackStatus stacks
type StackStatus string

// RequiresCleanup indicates that the stack was created, but failed.
// It should be deleted.
func (s StackStatus) RequiresCleanup() bool {
	return cloudformation.StackStatusRollbackComplete == string(s) ||
		cloudformation.StackStatusRollbackFailed == string(s)
}

// InProgress that the stack is currently being updated.
func (s StackStatus) InProgress() bool {
	return strings.HasSuffix(string(s), "IN_PROGRESS")
}

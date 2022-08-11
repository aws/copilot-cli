// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stackset

import "github.com/aws/aws-sdk-go/service/cloudformation"

const (
	opStatusSucceeded = cloudformation.StackSetOperationStatusSucceeded
	opStatusStopped   = cloudformation.StackSetOperationStatusStopped
	opStatusFailed    = cloudformation.StackSetOperationStatusFailed
)

// OpStatus represents a stack set operation status.
type OpStatus string

// IsCompleted returns true if the operation is in a final state.
func (s OpStatus) IsCompleted() bool {
	return s.IsSuccessful() || s.IsFailure()
}

// IsSuccessful returns true if the operation is completed successfully.
func (s OpStatus) IsSuccessful() bool {
	return s == opStatusSucceeded
}

// IsFailure returns true if the operation terminated in failure.
func (s OpStatus) IsFailure() bool {
	return s == opStatusStopped || s == opStatusFailed
}

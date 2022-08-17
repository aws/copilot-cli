// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stackset

import "github.com/aws/aws-sdk-go/service/cloudformation"

const (
	opStatusSucceeded = cloudformation.StackSetOperationStatusSucceeded
	opStatusStopped   = cloudformation.StackSetOperationStatusStopped
	opStatusFailed    = cloudformation.StackSetOperationStatusFailed
	opStatusRunning   = cloudformation.StackSetOperationStatusRunning
	opStatusStopping  = cloudformation.StackSetOperationStatusStopping
	opStatusQueued    = cloudformation.StackSetOperationStatusQueued
)

const (
	instanceStatusPending    = cloudformation.StackInstanceDetailedStatusPending
	instanceStatusRunning    = cloudformation.StackInstanceDetailedStatusRunning
	instanceStatusSucceeded  = cloudformation.StackInstanceDetailedStatusSucceeded
	instanceStatusFailed     = cloudformation.StackInstanceDetailedStatusFailed
	instanceStatusCancelled  = cloudformation.StackInstanceDetailedStatusCancelled
	instanceStatusInoperable = cloudformation.StackInstanceDetailedStatusInoperable
)

// OpStatus represents a stack set operation status.
type OpStatus string

// IsCompleted returns true if the operation is in a final state.
func (s OpStatus) IsCompleted() bool {
	return s.IsSuccess() || s.IsFailure()
}

// InProgress returns true if the operation has started but hasn't reached a final state yet.
func (s OpStatus) InProgress() bool {
	switch s {
	case opStatusQueued, opStatusRunning, opStatusStopping:
		return true
	default:
		return false
	}
}

// IsSuccess returns true if the operation is completed successfully.
func (s OpStatus) IsSuccess() bool {
	return s == opStatusSucceeded
}

// IsFailure returns true if the operation terminated in failure.
func (s OpStatus) IsFailure() bool {
	return s == opStatusStopped || s == opStatusFailed
}

// String implements the fmt.Stringer interface.
func (s OpStatus) String() string {
	return string(s)
}

// InstanceStatus represents a stack set's instance detailed status.
type InstanceStatus string

// IsCompleted returns true if the operation is in a final state.
func (s InstanceStatus) IsCompleted() bool {
	return s.IsSuccess() || s.IsFailure()
}

// InProgress returns true if the instance is being updated with a new template.
func (s InstanceStatus) InProgress() bool {
	return s == instanceStatusPending || s == instanceStatusRunning
}

// IsSuccess returns true if the instance is up-to-date with the stack set template.
func (s InstanceStatus) IsSuccess() bool {
	return s == instanceStatusSucceeded
}

// IsFailure returns true if the instance cannot be updated and needs to be recovered.
func (s InstanceStatus) IsFailure() bool {
	return s == instanceStatusFailed || s == instanceStatusCancelled || s == instanceStatusInoperable
}

// String implements the fmt.Stringer interface.
func (s InstanceStatus) String() string {
	return string(s)
}

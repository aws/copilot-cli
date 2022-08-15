// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stackset

import "github.com/aws/aws-sdk-go/service/cloudformation"

const (
	opStatusSucceeded = cloudformation.StackSetOperationStatusSucceeded
	opStatusStopped   = cloudformation.StackSetOperationStatusStopped
	opStatusFailed    = cloudformation.StackSetOperationStatusFailed
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

// InstanceStatus represents a stack set's instance detailed status.
type InstanceStatus string

// IsCompleted returns true if the operation is in a final state.
func (s InstanceStatus) IsCompleted() bool {
	return s.IsSuccessful() || s.IsFailure()
}

// InProgress returns true if the instance is being updated with a new template.
func (s InstanceStatus) InProgress() bool {
	for _, wanted := range ProgressInstanceStatuses() {
		if s == wanted {
			return true
		}
	}
	return false
}

// IsSuccessful returns true if the instance is up-to-date with the stack set template.
func (s InstanceStatus) IsSuccessful() bool {
	return s == instanceStatusSucceeded
}

// IsFailure returns true if the instance cannot be updated and needs to be recovered.
func (s InstanceStatus) IsFailure() bool {
	return s == instanceStatusFailed || s == instanceStatusCancelled || s == instanceStatusInoperable
}

// ProgressInstanceStatuses returns a slice of statuses that are InProgress.
func ProgressInstanceStatuses() []InstanceStatus {
	return []InstanceStatus{instanceStatusPending, instanceStatusRunning}
}

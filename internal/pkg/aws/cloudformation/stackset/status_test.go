// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stackset

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/stretchr/testify/require"
)

func TestOpStatus_IsCompleted(t *testing.T) {
	testCases := map[string]struct {
		status string
		wanted bool
	}{
		"false when queued": {
			status: cloudformation.StackSetOperationStatusQueued,
		},
		"false when running": {
			status: cloudformation.StackSetOperationStatusRunning,
		},
		"false when stopping": {
			status: cloudformation.StackSetOperationStatusStopping,
		},
		"true when succeeded": {
			status: cloudformation.StackSetOperationStatusSucceeded,
			wanted: true,
		},
		"true when stopped": {
			status: cloudformation.StackSetOperationStatusStopped,
			wanted: true,
		},
		"true when failed": {
			status: cloudformation.StackSetOperationStatusFailed,
			wanted: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, OpStatus(tc.status).IsCompleted())
		})
	}
}

func TestOpStatus_IsSuccess(t *testing.T) {
	testCases := map[string]struct {
		status string
		wanted bool
	}{
		"false when queued": {
			status: cloudformation.StackSetOperationStatusQueued,
		},
		"false when running": {
			status: cloudformation.StackSetOperationStatusRunning,
		},
		"false when stopping": {
			status: cloudformation.StackSetOperationStatusStopping,
		},
		"true when succeeded": {
			status: cloudformation.StackSetOperationStatusSucceeded,
			wanted: true,
		},
		"false when stopped": {
			status: cloudformation.StackSetOperationStatusStopped,
		},
		"false when failed": {
			status: cloudformation.StackSetOperationStatusFailed,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, OpStatus(tc.status).IsSuccess())
		})
	}
}

func TestOpStatus_InProgress(t *testing.T) {
	testCases := map[string]struct {
		status string
		wanted bool
	}{
		"true when queued": {
			status: cloudformation.StackSetOperationStatusQueued,
			wanted: true,
		},
		"true when running": {
			status: cloudformation.StackSetOperationStatusRunning,
			wanted: true,
		},
		"true when stopping": {
			status: cloudformation.StackSetOperationStatusStopping,
			wanted: true,
		},
		"false when succeeded": {
			status: cloudformation.StackSetOperationStatusSucceeded,
		},
		"false when stopped": {
			status: cloudformation.StackSetOperationStatusStopped,
		},
		"false when failed": {
			status: cloudformation.StackSetOperationStatusFailed,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, OpStatus(tc.status).InProgress())
		})
	}
}

func TestOpStatus_IsFailure(t *testing.T) {
	testCases := map[string]struct {
		status string
		wanted bool
	}{
		"false when queued": {
			status: cloudformation.StackSetOperationStatusQueued,
		},
		"false when running": {
			status: cloudformation.StackSetOperationStatusRunning,
		},
		"false when stopping": {
			status: cloudformation.StackSetOperationStatusStopping,
		},
		"false when succeeded": {
			status: cloudformation.StackSetOperationStatusSucceeded,
		},
		"true when stopped": {
			status: cloudformation.StackSetOperationStatusStopped,
			wanted: true,
		},
		"true when failed": {
			status: cloudformation.StackSetOperationStatusFailed,
			wanted: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, OpStatus(tc.status).IsFailure())
		})
	}
}

func TestOpStatus_String(t *testing.T) {
	var s OpStatus = "hello"
	require.Equal(t, "hello", s.String())
}

func TestInstanceStatus_IsCompleted(t *testing.T) {
	testCases := map[string]struct {
		status string
		wanted bool
	}{
		"false when pending": {
			status: cloudformation.StackInstanceDetailedStatusPending,
		},
		"false when running": {
			status: cloudformation.StackInstanceDetailedStatusRunning,
		},
		"true when succeeded": {
			status: cloudformation.StackInstanceDetailedStatusSucceeded,
			wanted: true,
		},
		"true when failed": {
			status: cloudformation.StackInstanceDetailedStatusFailed,
			wanted: true,
		},
		"true when cancelled": {
			status: cloudformation.StackInstanceDetailedStatusCancelled,
			wanted: true,
		},
		"true when inoperable": {
			status: cloudformation.StackInstanceDetailedStatusInoperable,
			wanted: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, InstanceStatus(tc.status).IsCompleted())
		})
	}
}

func TestInstanceStatus_InProgress(t *testing.T) {
	testCases := map[string]struct {
		status string
		wanted bool
	}{
		"true when pending": {
			status: cloudformation.StackInstanceDetailedStatusPending,
			wanted: true,
		},
		"true when running": {
			status: cloudformation.StackInstanceDetailedStatusRunning,
			wanted: true,
		},
		"false when succeeded": {
			status: cloudformation.StackInstanceDetailedStatusSucceeded,
		},
		"false when failed": {
			status: cloudformation.StackInstanceDetailedStatusFailed,
		},
		"false when cancelled": {
			status: cloudformation.StackInstanceDetailedStatusCancelled,
		},
		"false when inoperable": {
			status: cloudformation.StackInstanceDetailedStatusInoperable,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, InstanceStatus(tc.status).InProgress())
		})
	}
}

func TestInstanceStatus_IsSuccess(t *testing.T) {
	testCases := map[string]struct {
		status string
		wanted bool
	}{
		"false when pending": {
			status: cloudformation.StackInstanceDetailedStatusPending,
		},
		"false when running": {
			status: cloudformation.StackInstanceDetailedStatusRunning,
		},
		"true when succeeded": {
			status: cloudformation.StackInstanceDetailedStatusSucceeded,
			wanted: true,
		},
		"false when failed": {
			status: cloudformation.StackInstanceDetailedStatusFailed,
		},
		"false when cancelled": {
			status: cloudformation.StackInstanceDetailedStatusCancelled,
		},
		"false when inoperable": {
			status: cloudformation.StackInstanceDetailedStatusInoperable,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, InstanceStatus(tc.status).IsSuccess())
		})
	}
}

func TestInstanceStatus_IsFailure(t *testing.T) {
	testCases := map[string]struct {
		status string
		wanted bool
	}{
		"false when pending": {
			status: cloudformation.StackInstanceDetailedStatusPending,
		},
		"false when running": {
			status: cloudformation.StackInstanceDetailedStatusRunning,
		},
		"false when succeeded": {
			status: cloudformation.StackInstanceDetailedStatusSucceeded,
		},
		"true when failed": {
			status: cloudformation.StackInstanceDetailedStatusFailed,
			wanted: true,
		},
		"true when cancelled": {
			status: cloudformation.StackInstanceDetailedStatusCancelled,
			wanted: true,
		},
		"true when inoperable": {
			status: cloudformation.StackInstanceDetailedStatusInoperable,
			wanted: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, InstanceStatus(tc.status).IsFailure())
		})
	}
}

func TestInstanceStatus_String(t *testing.T) {
	var s InstanceStatus = "hello"
	require.Equal(t, "hello", s.String())
}

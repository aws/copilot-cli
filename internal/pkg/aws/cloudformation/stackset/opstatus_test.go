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

func TestOpStatus_IsSuccessful(t *testing.T) {
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
			require.Equal(t, tc.wanted, OpStatus(tc.status).IsSuccessful())
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
		"true when succeeded": {
			status: cloudformation.StackSetOperationStatusSucceeded,
		},
		"false when stopped": {
			status: cloudformation.StackSetOperationStatusStopped,
			wanted: true,
		},
		"false when failed": {
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

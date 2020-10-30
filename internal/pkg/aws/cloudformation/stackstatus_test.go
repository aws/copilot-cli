// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"testing"

	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/stretchr/testify/require"
)

func TestStackStatus_InProgress(t *testing.T) {
	testCases := map[string]struct {
		status string

		wanted bool
	}{
		"should be false if stack is created succesfully": {
			status: cloudformation.StackStatusCreateComplete,
			wanted: false,
		},
		"should be false if stack creation failed": {
			status: cloudformation.StackStatusCreateFailed,
			wanted: false,
		},
		"should be true if stack creation is in progress": {
			status: cloudformation.StackStatusCreateInProgress,
			wanted: true,
		},
		"should be true if stack update is in progress": {
			status: cloudformation.StackStatusUpdateInProgress,
			wanted: true,
		},
		"should be true if stack deletion is in progress": {
			status: cloudformation.StackStatusDeleteInProgress,
			wanted: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actual := StackStatus(tc.status).InProgress()
			require.Equal(t, tc.wanted, actual)
		})
	}
}

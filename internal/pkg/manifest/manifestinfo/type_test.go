// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifestinfo

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_IsTypeAService(t *testing.T) {
	testCases := map[string]struct {
		inType string
		wanted bool
	}{
		"return false if not a service": {
			inType: "foobar",
			wanted: false,
		},
		"return true if it is a service": {
			inType: LoadBalancedWebServiceType,
			wanted: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actual := IsTypeAService(tc.inType)
			require.Equal(t, actual, tc.wanted)
		})
	}
}

func Test_IsTypeAJob(t *testing.T) {
	testCases := map[string]struct {
		inType string
		wanted bool
	}{
		"return false if not a job": {
			inType: "foobar",
			wanted: false,
		},
		"return true if it is a job": {
			inType: ScheduledJobType,
			wanted: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actual := IsTypeAJob(tc.inType)
			require.Equal(t, actual, tc.wanted)
		})
	}
}

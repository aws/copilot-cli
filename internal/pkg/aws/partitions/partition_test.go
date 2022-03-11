// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package partitions

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegion_Partition(t *testing.T) {
	testCases := map[string]struct {
		region    string
		wantedErr error
	}{
		"error finding the partition": {
			region:    "weird region",
			wantedErr: errors.New("find the partition for region weird region"),
		},
		"success": {
			region: "us-west-2",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			_, err := Region(tc.region).Partition()
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRegion_IsAvailableInRegion(t *testing.T) {
	testCases := map[string]struct {
		sID       string
		region    string
		want      bool
		wantedErr error
	}{
		"ecs service exist in the given region": {
			region: "us-west-2",
			sID:    ECSEndpointsID,
			want:   true,
		},
		"ecs service does not exist in the given region": {
			region: "us-west-3",
			sID:    ECSEndpointsID,
			want:   false,
		},
		"apprunner service exist in the given region": {
			region: "us-west-2",
			sID:    AppRunnerEndpointsID,
			want:   true,
		},
		"apprunner service does not exist in the given region": {
			region: "us-west-3",
			sID:    AppRunnerEndpointsID,
			want:   false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := IsAvailableInRegion(tc.sID, tc.region)
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, got)
			}
		})
	}
}

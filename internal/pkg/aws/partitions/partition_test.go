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

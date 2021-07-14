// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_CloudFormationTemplate(t *testing.T) {
	testCases := map[string]struct {
		wantedContent string
		wantedError   error
	}{}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN

			// WHEN
			actualContent, err := CloudFormationTemplate(nil, "")

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedContent, actualContent)
			}
		})
	}
}

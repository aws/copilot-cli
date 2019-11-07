// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ecr

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestURIFromARN(t *testing.T) {

	testCases := map[string]struct {
		givenARN  string
		wantedURI string
		wantErr   error
	}{
		"valid arn": {
			givenARN:  "arn:aws:ecr:us-west-2:0123456789:repository/myrepo",
			wantedURI: "0123456789.dkr.ecr.us-west-2.amazonaws.com/myrepo",
		},
		"valid arn with namespace": {
			givenARN:  "arn:aws:ecr:us-west-2:0123456789:repository/myproject/myapp",
			wantedURI: "0123456789.dkr.ecr.us-west-2.amazonaws.com/myproject/myapp",
		},
		"separate region": {
			givenARN:  "arn:aws:ecr:us-east-1:0123456789:repository/myproject/myapp",
			wantedURI: "0123456789.dkr.ecr.us-east-1.amazonaws.com/myproject/myapp",
		},
		"invalid ARN": {
			givenARN: "myproject/myapp",
			wantErr:  fmt.Errorf("parsing repository ARN myproject/myapp: arn: invalid prefix"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			uri, err := URIFromARN(tc.givenARN)
			if tc.wantErr != nil {
				require.Error(t, err)
				require.EqualError(t, err, tc.wantErr.Error())
			} else {
				require.Equal(t, tc.wantedURI, uri)
			}
		})
	}
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppInformation_DNSDelegationRole(t *testing.T) {
	testCases := map[string]struct {
		in   *AppInformation
		want string
	}{
		"without tools account ARN": {
			want: "",
			in: &AppInformation{
				AccountPrincipalARN: "",
				Domain:              "ecs.aws",
			},
		},
		"without DNS": {
			want: "",
			in: &AppInformation{
				AccountPrincipalARN: "",
				Domain:              "ecs.aws",
			},
		},
		"with invalid tools principal": {
			want: "",
			in: &AppInformation{
				AccountPrincipalARN: "0000000",
				Domain:              "ecs.aws",
			},
		},
		"with dns and tools principal": {
			want: "arn:aws:iam::0000000:role/-DNSDelegationRole",

			in: &AppInformation{
				AccountPrincipalARN: "arn:aws:iam::0000000:root",
				Domain:              "ecs.aws",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.in.DNSDelegationRole())
		})
	}
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToSnakeCase(t *testing.T) {
	testCases := map[string]struct {
		in     string
		wanted string
	}{
		"camel case: starts with uppercase": {
			in:     "AdditionalResourcesPolicyArn",
			wanted: "ADDITIONAL_RESOURCES_POLICY_ARN",
		},
		"camel case: starts with lowercase": {
			in:     "additionalResourcesPolicyArn",
			wanted: "ADDITIONAL_RESOURCES_POLICY_ARN",
		},
		"all lower case": {
			in:     "myddbtable",
			wanted: "MYDDBTABLE",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, toSnakeCase(tc.in))
		})
	}
}

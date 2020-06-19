// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

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
		"has capitals in acronym": {
			in:     "myDDBTable",
			wanted: "MY_DDB_TABLE",
		},
		"has capitals and numbers": {
			in:     "my2ndDDBTable",
			wanted: "MY2ND_DDB_TABLE",
		},
		"has capitals at end": {
			in:     "myTableWithLSI",
			wanted: "MY_TABLE_WITH_LSI",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, ToSnakeCase(tc.in))
		})
	}
}

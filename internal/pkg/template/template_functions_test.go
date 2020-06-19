// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReplaceDashes(t *testing.T) {
	testCases := map[string]struct {
		in     string
		wanted string
	}{
		"no dashes": {
			in:     "mycooltable",
			wanted: "mycooltable",
		},
		"has dash": {
			in:     "my-table",
			wanted: "myDASHtable",
		},
		"has multiple dashes": {
			in:     "my--dog-table",
			wanted: "myDASHDASHdogDASHtable",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, ReplaceDashes(tc.in))
		})
	}
}

func TestDashReplacedLogicalIDToOriginal(t *testing.T) {
	testCases := map[string]struct {
		in     string
		wanted string
	}{
		"no dashes": {
			in:     "mycooltable",
			wanted: "mycooltable",
		},
		"has dash": {
			in:     "myDASHtable",
			wanted: "my-table",
		},
		"has multiple dashes": {
			in:     "myDASHDASHdogDASHtable",
			wanted: "my--dog-table",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, DashReplacedLogicalIDToOriginal(tc.in))
		})
	}
}
func TestLogicalIDSafe(t *testing.T) {
	testCases := map[string]struct {
		in     string
		wanted string
	}{
		"all alphanumeric": {
			in:     "MyCoolTable5",
			wanted: "MyCoolTable5",
		},
		"ddb-allowed special characters": {
			in:     "My_Table-Name.5",
			wanted: "MyTableName5",
		},
		"s3-allowed special characters": {
			in:     "my-bucket-5",
			wanted: "mybucket5",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, LogicalIDSafe(tc.in))
		})
	}
}

func TestEnvVarName(t *testing.T) {
	testCases := map[string]struct {
		in     string
		wanted string
	}{
		"all alphanumeric": {
			in:     "MyCoolTable5",
			wanted: "MyCoolTable5Name",
		},
		"ddb-allowed special characters": {
			in:     "My_Table-Name.5",
			wanted: "MyTableName5Name",
		},
		"s3-allowed special characters": {
			in:     "my-bucket-5",
			wanted: "mybucket5Name",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, EnvVarName(tc.in))
		})
	}
}

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

func TestInc(t *testing.T) {
	testCases := map[string]struct {
		in     int
		wanted int
	}{
		"negative": {
			in:     -1,
			wanted: 0,
		},
		"large negative": {
			in:     -32767,
			wanted: -32766,
		},
		"large positive": {
			in:     4294967296,
			wanted: 4294967297,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, Inc(tc.in))
		})
	}
}

func TestFmtSlice(t *testing.T) {
	testCases := map[string]struct {
		in     []string
		wanted string
	}{
		"simple case": {
			in:     []string{"my", "elements", "go", "here"},
			wanted: "[my, elements, go, here]",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, FmtSlice(tc.in))
		})
	}
}

func TestQuoteSlice(t *testing.T) {
	testCases := map[string]struct {
		in     []string
		wanted []string
	}{
		"simple case": {
			in:     []string{"my", "elements", "go", "here"},
			wanted: []string{"\"my\"", "\"elements\"", "\"go\"", "\"here\""},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, QuoteSlice(tc.in))
		})
	}
}

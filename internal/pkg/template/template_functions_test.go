// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

func TestURLSafeVersionFunc(t *testing.T) {
	testCases := map[string]struct {
		in     string
		wanted string
	}{
		"no plus": {
			in:     "v1.29.0",
			wanted: "v1.29.0",
		},
		"has plus": {
			in:     "v1.29.0+5-g74ef584b3",
			wanted: "v1.29.0%2B5-g74ef584b3",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, URLSafeVersion(tc.in))
		})
	}
}

func TestReplaceDashesFunc(t *testing.T) {
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
			require.Equal(t, tc.wanted, ReplaceDashesFunc(tc.in))
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
func TestStripNonAlphaNumFunc(t *testing.T) {
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
			require.Equal(t, tc.wanted, StripNonAlphaNumFunc(tc.in))
		})
	}
}

func TestEnvVarNameFunc(t *testing.T) {
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
			require.Equal(t, tc.wanted, EnvVarNameFunc(tc.in))
		})
	}
}

func TestToSnakeCaseFunc(t *testing.T) {
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
			require.Equal(t, tc.wanted, ToSnakeCaseFunc(tc.in))
		})
	}
}

func TestIncFunc(t *testing.T) {
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
			require.Equal(t, tc.wanted, IncFunc(tc.in))
		})
	}
}

func TestFmtSliceFunc(t *testing.T) {
	testCases := map[string]struct {
		in     []string
		wanted string
	}{
		"simple case": {
			in:     []string{"my", "elements", "go", "here"},
			wanted: "[my, elements, go, here]",
		},
		"no elements": {
			in:     []string{},
			wanted: "[]",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, FmtSliceFunc(tc.in))
		})
	}
}

func TestQuoteSliceFunc(t *testing.T) {
	testCases := map[string]struct {
		in     []string
		wanted []string
	}{
		"simple case": {
			in:     []string{"my", "elements", "go", "here"},
			wanted: []string{"\"my\"", "\"elements\"", "\"go\"", "\"here\""},
		},
		"no elements": {
			in:     []string{},
			wanted: []string(nil),
		},
		"nil input": {
			in:     []string(nil),
			wanted: []string(nil),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, QuoteSliceFunc(tc.in))
		})
	}
}

func TestGenerateMountPointJSON(t *testing.T) {
	require.Equal(t, `{"myEFSVolume":"/var/www"}`, generateMountPointJSON([]*MountPoint{{ContainerPath: aws.String("/var/www"), SourceVolume: aws.String("myEFSVolume")}}), "JSON should render correctly")
	require.Equal(t, "{}", generateMountPointJSON([]*MountPoint{}), "nil list of arguments should render ")
	require.Equal(t, "{}", generateMountPointJSON([]*MountPoint{{SourceVolume: aws.String("fromEFS")}}), "empty paths should not get injected")
}

func TestGenerateSNSJSON(t *testing.T) {
	testCases := map[string]struct {
		in     []*Topic
		wanted string
	}{
		"JSON should render correctly": {
			in: []*Topic{
				{
					Name:      aws.String("tests"),
					AccountID: "123456789012",
					Region:    "us-west-2",
					Partition: "aws",
					App:       "appName",
					Env:       "envName",
					Svc:       "svcName",
				},
			},
			wanted: `{"tests":"arn:aws:sns:us-west-2:123456789012:appName-envName-svcName-tests"}`,
		},
		"Topics with no names show empty": {
			in: []*Topic{
				{
					AccountID: "123456789012",
					Region:    "us-west-2",
					Partition: "aws",
					App:       "appName",
					Env:       "envName",
					Svc:       "svcName",
				},
			},
			wanted: `{}`,
		},
		"nil list of arguments should render": {
			in:     []*Topic{},
			wanted: `{}`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, generateSNSJSON(tc.in))
		})
	}
}

func TestGenerateQueueURIJSON(t *testing.T) {
	testCases := map[string]struct {
		in              []*TopicSubscription
		wanted          string
		wantedSubstring string
	}{
		"JSON should render correctly": {
			in: []*TopicSubscription{
				{
					Name:    aws.String("tests"),
					Service: aws.String("bestsvc"),
					Queue: &SQSQueue{
						Delay: aws.Int64(5),
					},
				},
			},
			wantedSubstring: `"bestsvcTestsEventsQueue":"${bestsvctestsURL}"`,
		},
		"Topics with no names show empty but main queue still populates": {
			in: []*TopicSubscription{
				{
					Service: aws.String("bestSvc"),
				},
			},
			wanted: `{}`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if tc.wanted != "" {
				require.Equal(t, generateQueueURIJSON(tc.in), tc.wanted)
			} else {
				require.Contains(t, generateQueueURIJSON(tc.in), tc.wantedSubstring)
			}
		})
	}
}

//go:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addon_test

import (
	"encoding"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAddons(t *testing.T) {
	testCases := map[string]struct {
		addonMarshaler encoding.BinaryMarshaler
		outFileName    string
	}{
		"aurora": {
			addonMarshaler: addon.WorkloadServerlessV2Template(addon.RDSProps{
				ClusterName:   "aurora",
				Engine:        "MySQL",
				InitialDBName: "main",
				Envs:          []string{"test"},
			}),
			outFileName: "aurora.yml",
		},
		"ddb": {
			addonMarshaler: addon.WorkloadDDBTemplate(&addon.DynamoDBProps{
				StorageProps: &addon.StorageProps{
					Name: "ddb",
				},
				Attributes: []addon.DDBAttribute{
					{
						Name:     aws.String("primary"),
						DataType: aws.String("S"),
					},
					{
						Name:     aws.String("sort"),
						DataType: aws.String("N"),
					},
					{
						Name:     aws.String("othersort"),
						DataType: aws.String("B"),
					},
				},
				SortKey:      aws.String("sort"),
				PartitionKey: aws.String("primary"),
				LSIs: []addon.DDBLocalSecondaryIndex{
					{
						Name:         aws.String("othersort"),
						PartitionKey: aws.String("primary"),
						SortKey:      aws.String("othersort"),
					},
				},
				HasLSI: true,
			}),
			outFileName: "ddb.yml",
		},
		"s3": {
			addonMarshaler: addon.WorkloadS3Template(&addon.S3Props{
				StorageProps: &addon.StorageProps{
					Name: "bucket",
				},
			}),
			outFileName: "bucket.yml",
		},
	}

	for name, tc := range testCases {
		testName := fmt.Sprintf("CF Template should be equal/%s", name)
		t.Run(testName, func(t *testing.T) {

			actualBytes, err := tc.addonMarshaler.MarshalBinary()
			require.NoError(t, err, "cf should render")

			cfActual := make(map[interface{}]interface{})
			require.NoError(t, yaml.Unmarshal(actualBytes, cfActual))

			expected, err := os.ReadFile(filepath.Join("testdata", "storage", tc.outFileName))
			require.NoError(t, err, "should be able to read expected bytes")
			expectedBytes := []byte(expected)
			mExpected := make(map[interface{}]interface{})
			require.NoError(t, yaml.Unmarshal(expectedBytes, mExpected))
			require.Equal(t, mExpected, cfActual)
		})
	}
}

// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOutputs(t *testing.T) {
	testCases := map[string]struct {
		testdataFileName string

		wantedOut []Output
		wantedErr error
	}{
		"parses valid CFN template": {
			testdataFileName: "template.yml",

			wantedOut: []Output{
				{
					Name:            "AdditionalResourcesPolicyArn",
					IsManagedPolicy: true,
				},
				{
					Name:     "MyRDSInstanceRotationSecretArn",
					IsSecret: true,
				},
				{
					Name: "MyDynamoDBTableName",
				},
				{
					Name: "MyDynamoDBTableArn",
				},
				{
					Name: "TestExport",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			template, err := ioutil.ReadFile(filepath.Join("testdata", "outputs", tc.testdataFileName))
			require.NoError(t, err)

			// WHEN
			out, err := Outputs(string(template))

			// THEN
			require.Equal(t, tc.wantedErr, err)
			require.ElementsMatch(t, tc.wantedOut, out)
		})
	}
}

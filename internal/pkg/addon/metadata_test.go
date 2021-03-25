// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addon

import (
    "errors"
    "io/ioutil"
    "path/filepath"
    "strings"
    "testing"

    "github.com/stretchr/testify/require"
)

func TestMetadata(t *testing.T) {
    testCases := map[string]struct {
        template         string
        testdataFileName string

        wantedMetadata []Metadata
        wantedErr error
    }{
        "returns an error if Metadata is not defined as a map": {
            template:  "Metadata: hello",
            wantedErr: errors.New(`"Metadata" field in cloudformation template is not a map`),
        },
        "returns ": {
            template: `
Metadata:
    AddonAurora: true
    AddonS3: true
`,
            wantedMetadata: []Metadata{
                {
                    Key: "AddonAurora",
                    Value: "true",
                },
                {
                    Key: "AddonS3",
                    Value: "true",
                },
            },
        },
        "returns a nil list if there are no metadata defined": {
            template: `
Resources:
  MyDBInstance:
    Type: AWS::RDS::DBInstance
`,
        },
    }

    for name, tc := range testCases {
        t.Run(name, func(t *testing.T) {
            // GIVEN
            template := tc.template
            if tc.testdataFileName != "" {
                content, err := ioutil.ReadFile(filepath.Join("testdata", "outputs", tc.testdataFileName))
                require.NoError(t, err)
                template = string(content)
            }

            // WHEN
            mt, err := Metadatas(template)

            // THEN
            if tc.wantedErr != nil {
                require.NotNil(t, err, "expected a non-nil error to be returned")
                require.True(t, strings.HasPrefix(err.Error(), tc.wantedErr.Error()), "expected the error %v to be wrapped by our prefix %v", err, tc.wantedErr)
            } else {
                require.NoError(t, err)
                require.ElementsMatch(t, tc.wantedMetadata, mt)
            }
        })
    }
}

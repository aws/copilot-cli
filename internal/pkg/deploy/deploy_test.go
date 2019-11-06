// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package deploy holds the structures to deploy applications and environments.
package deploy

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

func TestParseOwnerAndRepo(t *testing.T) {
	testCases := map[string]struct {
		src            *Source
		expectedErrMsg *string
		expectedOwner  string
		expectedRepo   string
	}{
		"unsupported source provider": {
			src: &Source{
				ProviderName: "chicken",
				Properties:   map[string]interface{}{},
			},
			expectedErrMsg: aws.String("invalid provider: chicken"),
		},
		"missing repository property": {
			src: &Source{
				ProviderName: "GitHub",
				Properties:   map[string]interface{}{},
			},
			expectedErrMsg: aws.String("unable to locate the repository from the properties"),
		},
		"invalid repository property": {
			src: &Source{
				ProviderName: "GitHub",
				Properties: map[string]interface{}{
					"repository": "invalid",
				},
			},
			expectedErrMsg: aws.String("unable to locate the repository from the properties"),
		},
		"valid repository property": {
			src: &Source{
				ProviderName: "GitHub",
				Properties: map[string]interface{}{
					"repository": "chicken/wings",
				},
			},
			expectedErrMsg: nil,
			expectedOwner:  "chicken",
			expectedRepo:   "wings",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			oAndR, err := tc.src.parseOwnerAndRepo()
			if tc.expectedErrMsg != nil {
				require.Contains(t, err.Error(), *tc.expectedErrMsg)
			} else {
				require.NoError(t, err, "expected error")
				require.Equal(t, tc.expectedOwner, oAndR.owner, "mismatched owner")
				require.Equal(t, tc.expectedRepo, oAndR.repo, "mismatched repo")
			}
		})
	}
}

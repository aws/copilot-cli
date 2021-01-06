// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

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
			expectedErrMsg: aws.String("unable to locate the repository URL from the properties"),
		},
		"valid GH repository property": {
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
		"valid full CC repository name": {
			src: &Source{
				ProviderName: "CodeCommit",
				Properties: map[string]interface{}{
					"repository": "https://us-west-2.console.aws.amazon.com/codesuite/codecommit/repositories/wings/browse",
				},
			},
			expectedErrMsg: nil,
			expectedOwner:  "",
			expectedRepo:   "wings",
		},
		"valid full GH repository name": {
			src: &Source{
				ProviderName: "GitHub",
				Properties: map[string]interface{}{
					"repository": "https://github.com/badgoose/chaOS",
				},
			},
			expectedErrMsg: nil,
			expectedOwner:  "badgoose",
			expectedRepo:   "chaOS",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			owner, repo, err := tc.src.parseOwnerAndRepo(tc.src.ProviderName)
			if tc.expectedErrMsg != nil {
				require.Contains(t, err.Error(), *tc.expectedErrMsg)
			} else {
				require.NoError(t, err, "expected error")
				require.Equal(t, tc.expectedOwner, owner, "mismatched owner")
				require.Equal(t, tc.expectedRepo, repo, "mismatched repo")
			}
		})
	}
}

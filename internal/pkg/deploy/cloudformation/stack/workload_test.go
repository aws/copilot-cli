// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestECRImage_GetLocation(t *testing.T) {
	testCases := map[string]struct {
		in ECRImage

		wanted string
	}{
		"should use the image tag over anything else": {
			in: ECRImage{
				RepoURL:  "aws_account_id.dkr.ecr.us-west-2.amazonaws.com/amazonlinux",
				ImageTag: "ab1f5575",
				Digest:   "sha256:f1d4ae3f7261a72e98c6ebefe9985cf10a0ea5bd762585a43e0700ed99863807",
			},
			wanted: "aws_account_id.dkr.ecr.us-west-2.amazonaws.com/amazonlinux:ab1f5575",
		},
		"should use the digest if no tag is provided": {
			in: ECRImage{
				RepoURL: "aws_account_id.dkr.ecr.us-west-2.amazonaws.com/amazonlinux",
				Digest:  "sha256:f1d4ae3f7261a72e98c6ebefe9985cf10a0ea5bd762585a43e0700ed99863807",
			},
			wanted: "aws_account_id.dkr.ecr.us-west-2.amazonaws.com/amazonlinux@sha256:f1d4ae3f7261a72e98c6ebefe9985cf10a0ea5bd762585a43e0700ed99863807",
		},
		"should use the latest image if nothing is provided": {
			in: ECRImage{
				RepoURL: "aws_account_id.dkr.ecr.us-west-2.amazonaws.com/amazonlinux",
			},
			wanted: "aws_account_id.dkr.ecr.us-west-2.amazonaws.com/amazonlinux:latest",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.URI())
		})
	}
}

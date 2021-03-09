// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSuffixWriter_Write(t *testing.T) {
	testCases := map[string]struct {
		inText string
		suffix []byte

		wantedText string
	}{
		"should wrap every new line with the suffix": {
			inText: `The AWS Copilot CLI is a tool for developers to build, release
and operate production ready containerized applications
on Amazon ECS and AWS Fargate.
`,
			suffix: []byte{'\t', '\t'},

			wantedText: "The AWS Copilot CLI is a tool for developers to build, release\t\t\n" +
				"and operate production ready containerized applications\t\t\n" +
				"on Amazon ECS and AWS Fargate.\t\t\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			buf := new(strings.Builder)
			sw := &suffixWriter{
				buf:    buf,
				suffix: tc.suffix,
			}

			// WHEN
			_, err := sw.Write([]byte(tc.inText))

			// THEN
			require.NoError(t, err)
			require.Equal(t, tc.wantedText, buf.String())
		})
	}
}

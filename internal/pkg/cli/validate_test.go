// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type testCase struct {
	input interface{}
	want  error
}

var basicNameTestCases = map[string]testCase{
	"string as input": {
		input: "chicken1234",
		want:  nil,
	},
	"number as input": {
		input: 1234,
		want:  errValueNotAString,
	},
	"bool as input": {
		input: false,
		want:  errValueNotAString,
	},
	"string with invalid characters": {
		input: "myProject!",
		want:  errValueBadFormat,
	},
	"empty string": {
		input: "",
		want:  errValueEmpty,
	},
	"invalid length string": {
		input: strings.Repeat("s", 256),
		want:  errValueTooLong,
	},
	"does not start with letter": {
		input: "123chicken",
		want:  errValueBadFormat,
	},
}

func TestValidateProjectName(t *testing.T) {
	// Any project-specific name validations can be added here
	testCases := map[string]testCase{
		"contains emoji": testCase{
			input: "😀",
			want:  errValueBadFormat,
		},
	}

	for name, tc := range basicNameTestCases {
		t.Run(name, func(t *testing.T) {
			got := validateProjectName(tc.input)

			require.Equal(t, tc.want, got)
		})
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateProjectName(tc.input)

			require.Equal(t, tc.want, got)
		})
	}
}

func TestValidateApplicationName(t *testing.T) {
	testCases := basicNameTestCases

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateApplicationName(tc.input)

			require.Equal(t, tc.want, got)
		})
	}
}

func TestValidateEnvironmentName(t *testing.T) {
	testCases := basicNameTestCases

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateApplicationName(tc.input)

			require.Equal(t, tc.want, got)
		})
	}
}

func TestIsCorrectFormat(t *testing.T) {
	testCases := map[string]struct {
		input string
		want  bool
	}{
		"numbers only input": {
			input: "1234",
			want:  false,
		},
		"alphabetic input only": {
			input: "abcDaZ",
			want:  true,
		},
		"alphanumeric string input": {
			input: "abC123",
			want:  true,
		},
		"contains hyphen": {
			input: "bad-goose",
			want:  true,
		},
		"non-alphanumeric string input": {
			input: "bad-goose!",
			want:  false,
		},
		"starts with non-letter": {
			input: "1bad-goose",
			want:  false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := isCorrectFormat(tc.input)

			require.Equal(t, tc.want, got)
		})
	}
}

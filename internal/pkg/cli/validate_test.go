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
	"string with non-alphanumerics": {
		input: "my-project",
		want:  errValueNotAlphanumeric,
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
		want:  errValueFirstCharNotLetter,
	},
}

func TestValidateProjectName(t *testing.T) {
	// Any project-specific name validations can be added here
	testCases := map[string]testCase{
		"contains emoji": testCase{
			input: "😀",
			want:  errValueNotAlphanumeric,
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

func TestIsAlphanumeric(t *testing.T) {
	testCases := map[string]struct {
		input string
		want  bool
	}{
		"numbers only input": {
			input: "1234",
			want:  true,
		},
		"alphabetic input only": {
			input: "abcdaz",
			want:  true,
		},
		"alphanumeric string input": {
			input: "abc123",
			want:  true,
		},
		"non-alphanumeric string input": {
			input: "my-value",
			want:  false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := isAlphanumeric(tc.input)

			require.Equal(t, tc.want, got)
		})
	}
}

func TestStartsWithLetter(t *testing.T) {
	testCases := map[string]struct {
		input string
		want  bool
	}{
		"starts with letter": {
			input: "foo1234",
			want:  true,
		},
		"starts with number": {
			input: "1234foo",
			want:  false,
		},
		"starts with special char": {
			input: "_foo",
			want:  false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := startsWithLetter(tc.input)

			require.Equal(t, tc.want, got)
		})
	}
}

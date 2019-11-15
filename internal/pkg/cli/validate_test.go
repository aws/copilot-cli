// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
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
	"contains upper-case letters": {
		input: "badGoose",
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

			require.True(t, errors.Is(got, tc.want))
		})
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateProjectName(tc.input)

			require.True(t, errors.Is(got, tc.want))
		})
	}
}

func TestValidateApplicationName(t *testing.T) {
	testCases := basicNameTestCases

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateApplicationName(tc.input)

			require.True(t, errors.Is(got, tc.want))
		})
	}
}

func TestValidateEnvironmentName(t *testing.T) {
	testCases := basicNameTestCases

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateApplicationName(tc.input)

			require.True(t, errors.Is(got, tc.want))
		})
	}
}

func TestIsCorrectFormat(t *testing.T) {
	testCases := map[string]struct {
		input string
		isLegit bool
	}{
		"numbers only input": {
			input: "1234",
			isLegit:  false,
		},
		"lower-case alphabetic input only": {
			input: "badgoose",
			isLegit:  true,
		},
		"alphanumeric string input": {
			input: "abc123",
			isLegit:  true,
		},
		"contains hyphen": {
			input: "bad-goose",
			isLegit:  true,
		},
		"non-alphanumeric string input": {
			input: "bad-goose!",
			isLegit:  false,
		},
		"starts with non-letter": {
			input: "1bad-goose",
			isLegit:  false,
		},
		"contains capital letter": {
			input: "badGoose",
			isLegit:  false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := isCorrectFormat(tc.input)

			require.Equal(t, tc.isLegit, got)
		})
	}
}

func TestValidateGitHubRepo(t *testing.T) {
	testCases := map[string]struct {
		input string
		err error
	}{
		"full url": {
			input: "https://github.com/badgoose/chaos",
			err:  nil,
		},
		"owner and repo only": {
			input: "badgoose/chaos",
			err:  nil,
		},
		"invalid repo": {
			input: "THEGOOSE",
			err: errInvalidGitHubRepo,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateGitHubRepo(tc.input)

			require.True(t, errors.Is(got, tc.err))
		})
	}
}


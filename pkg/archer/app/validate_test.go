// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateProjectName(t *testing.T) {
	testCases := map[string]struct {
		input interface{}
		want  error
	}{
		"string as input": {
			input: "1234",
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
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateProjectName(tc.input)

			require.Equal(t, tc.want, got)
		})
	}
}

func TestValidateApplicationName(t *testing.T) {
	testCases := map[string]struct {
		input interface{}
		want  error
	}{
		"string as input": {
			input: "1234",
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
			input: "my-application",
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
	}

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

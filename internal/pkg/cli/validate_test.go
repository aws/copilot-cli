// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
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
		"contains emoji": {
			input: "😀",
			want:  errValueBadFormat,
		},
	}

	for name, tc := range basicNameTestCases {
		t.Run(name, func(t *testing.T) {
			got := validateAppName(tc.input)

			require.True(t, errors.Is(got, tc.want))
		})
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateAppName(tc.input)

			require.True(t, errors.Is(got, tc.want))
		})
	}
}

func TestValidateSvcName(t *testing.T) {
	testCases := basicNameTestCases

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateSvcName(tc.input)

			require.True(t, errors.Is(got, tc.want))
		})
	}
}

func TestValidateEnvironmentName(t *testing.T) {
	testCases := basicNameTestCases

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateSvcName(tc.input)

			require.True(t, errors.Is(got, tc.want))
		})
	}
}

var s3TestCases = map[string]testCase{
	"good case": {
		input: "happy-s3-bucket",
		want:  nil,
	},
	"contains punctuation": {
		input: "sadbucket!",
		want:  errS3ValueBadFormat,
	},
	"contains spaces": {
		input: "bowie is a good dog",
		want:  errS3ValueBadFormat,
	},
	"leading whitespace": {
		input: " a-Very-GOOD-dog-indeed",
		want:  errS3ValueBadFormat,
	},
	"too long": {
		input: "sitting-in-the-morning-sun-ill-be-sitting-when-the-evening-comes-watching-the-ships-roll-in",
		want:  errS3ValueBadSize,
	},
	"too short": {
		input: "oh",
		want:  errS3ValueBadSize,
	},
}

func TestValidateS3Name(t *testing.T) {
	testCases := s3TestCases

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := s3BucketNameValidation(tc.input)

			require.True(t, errors.Is(got, tc.want))
		})
	}
}

var dynamoTestCases = map[string]testCase{
	"good case": {
		input: "dynamo_table-1",
		want:  nil,
	},
	"too short": {
		input: "p",
		want:  errDDBValueBadSize,
	},
	"too long": {
		input: "i-met-a-traveller-from-an-antique-land-who-said_two-vast-and-trunkless-legs-of-stone_stand-in-the-desert-near-them-on-the-sand_half-sunk-a-shattered-visage-lies-whose-frown_and-wrinkled-lip-and-sneer-of-cold-command_tell-that-its-sculptor-well-those-passions-read_which-yet-survive-stamped-on-these-lifeless-things",
		want:  errDDBValueBadSize,
	},
	"bad character": {
		input: "badgoose!?",
		want:  errDDBValueBadFormat,
	},
}

func TestValidateDDBName(t *testing.T) {
	testCases := dynamoTestCases
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := dynamoTableNameValidation(tc.input)

			require.True(t, errors.Is(got, tc.want))
		})
	}
}

func TestValidateStorageType(t *testing.T) {
	testCases := map[string]struct {
		input string
		want  error
	}{
		"S3 okay": {
			input: "S3 Bucket",
			want:  nil,
		},
		"DDB okay": {
			input: "Dynamo DB",
			want:  nil,
		},
		"Bad name": {
			input: "Dropbox",
			want:  fmt.Errorf(fmtErrInvalidStorageType, "Dropbox", prettify(storageTypes)),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateStorageType(tc.input)
			if tc.want == nil {
				require.Nil(t, got)
			} else {
				require.EqualError(t, tc.want, got.Error())
			}
		})
	}

}

func TestIsCorrectFormat(t *testing.T) {
	testCases := map[string]struct {
		input   string
		isLegit bool
	}{
		"numbers only input": {
			input:   "1234",
			isLegit: false,
		},
		"lower-case alphabetic input only": {
			input:   "badgoose",
			isLegit: true,
		},
		"alphanumeric string input": {
			input:   "abc123",
			isLegit: true,
		},
		"contains hyphen": {
			input:   "bad-goose",
			isLegit: true,
		},
		"non-alphanumeric string input": {
			input:   "bad-goose!",
			isLegit: false,
		},
		"starts with non-letter": {
			input:   "1bad-goose",
			isLegit: false,
		},
		"contains capital letter": {
			input:   "badGoose",
			isLegit: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := isCorrectFormat(tc.input)

			require.Equal(t, tc.isLegit, got)
		})
	}
}

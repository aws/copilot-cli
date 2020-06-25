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

func TestValidateS3Name(t *testing.T) {
	testCases := map[string]testCase{
		"good case": {
			input: "happy-s3-bucket",
			want:  nil,
		},
		"contains punctuation": {
			input: "sadbucket!",
			want:  errValueBadFormatWithPeriod,
		},
		"contains spaces": {
			input: "bowie is a good dog",
			want:  errValueBadFormatWithPeriod,
		},
		"leading whitespace": {
			input: " a-Very-GOOD-dog-indeed",
			want:  errValueBadFormatWithPeriod,
		},
		"too long": {
			input: "sitting-in-the-morning-sun-ill-be-sitting-when-the-evening-comes-watching-the-ships-roll-in",
			want:  errS3ValueBadSize,
		},
		"too short": {
			input: "oh",
			want:  errS3ValueBadSize,
		},
		"consecutive dots": {
			input: "b.u..cket",
			want:  errS3ValueBadFormat,
		},
		"trailing dash": {
			input: "bucket-",
			want:  errS3ValueTrailingDash,
		},
		"consecutive -.": {
			input: "bu.-cket",
			want:  errS3ValueBadFormat,
		},
		"ip address format": {
			input: "123.455.999.000",
			want:  errS3ValueBadFormat,
		},
		"non-ip-address numbers and dots": {
			input: "124.333.333.333.333",
			want:  nil,
		},
		"capital letters in bucket name": {
			input: "BADbucketname",
			want:  errValueBadFormatWithPeriod,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := s3BucketNameValidation(tc.input)

			require.True(t, errors.Is(got, tc.want))
		})
	}
}

func TestValidateDDBName(t *testing.T) {
	testCases := map[string]testCase{
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
			want:  errValueBadFormatWithPeriodUnderscore,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := dynamoTableNameValidation(tc.input)
			t.Logf("error: %v", got)
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
			input: "S3",
			want:  nil,
		},
		"DDB okay": {
			input: "DynamoDB",
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

func TestValidateKey(t *testing.T) {
	testCases := map[string]struct {
		input string
		want  error
	}{
		"good key": {
			input: "userID:S",
			want:  nil,
		},
		"bad key with space": {
			input: "user ID:S",
			want:  errDDBAttributeBadFormat,
		},
		"nonsense key": {
			input: "sfueir555'/",
			want:  errDDBAttributeBadFormat,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateKey(tc.input)

			require.Equal(t, tc.want, got)
		})
	}
}

func TestGetAttributeFromKey(t *testing.T) {
	testCases := map[string]struct {
		input     string
		wantName  string
		wantType  string
		wantError error
	}{
		"good case": {
			input:     "userID:S",
			wantName:  "userID",
			wantType:  "S",
			wantError: nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := getAttrFromKey(tc.input)
			if tc.wantError != nil {
				require.EqualError(t, err, tc.wantError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantName, got.name)
				require.Equal(t, tc.wantType, got.dataType)
			}
		})
	}

}
func TestValidateLSIs(t *testing.T) {
	testCases := map[string]struct {
		inputAttributes []string
		inputLsis       []string
		wantError       error
	}{
		"good case": {
			inputLsis: []string{"userID:S"},
			wantError: nil,
		},
		"bad lsi structure": {
			inputLsis: []string{"userID"},
			wantError: errDDBAttributeBadFormat,
		},
		"too many lsis": {
			inputLsis: []string{"bowie:S", "clyde:S", "keno:S", "kava:S", "meow:S", "hana:S"},
			wantError: errTooManyLsiKeys,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateLSIs(tc.inputLsis)
			if tc.wantError != nil {
				require.EqualError(t, got, tc.wantError.Error())
			} else {
				require.Nil(t, got)
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

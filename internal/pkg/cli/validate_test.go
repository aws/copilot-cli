// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/spf13/afero"

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
		want:  errBasicNameRegexNotMatched,
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
		want:  errBasicNameRegexNotMatched,
	},
	"contains upper-case letters": {
		input: "badGoose",
		want:  errBasicNameRegexNotMatched,
	},
}

func TestValidateAppName(t *testing.T) {
	// Any project-specific name validations can be added here
	testCases := map[string]testCase{
		"contains emoji": {
			input: "😀",
			want:  errBasicNameRegexNotMatched,
		},
	}

	for name, tc := range basicNameTestCases {
		t.Run(name, func(t *testing.T) {
			got := validateAppNameString(tc.input)

			require.True(t, errors.Is(got, tc.want))
		})
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateAppNameString(tc.input)

			require.True(t, errors.Is(got, tc.want))
		})
	}
}

func TestValidateSvcName(t *testing.T) {
	testCases := map[string]struct {
		val     interface{}
		svcType string

		wanted error
	}{
		"string as input": {
			val:     "hello",
			svcType: manifestinfo.LoadBalancedWebServiceType,
			wanted:  nil,
		},
		"number as input": {
			val:    1234,
			wanted: errValueNotAString,
		},
		"string with invalid characters": {
			val:    "mySvc!",
			wanted: errBasicNameRegexNotMatched,
		},
		"longer than 40 characters for app runner services": {
			val:     strings.Repeat("x", 41),
			svcType: manifestinfo.RequestDrivenWebServiceType,
			wanted:  errAppRunnerSvcNameTooLong,
		},
		"invalid length string": {
			val:     strings.Repeat("s", 256),
			svcType: manifestinfo.LoadBalancedWebServiceType,
			wanted:  errValueTooLong,
		},
		"does not start with letter": {
			val:     "123chicken",
			svcType: manifestinfo.BackendServiceType,
			wanted:  errBasicNameRegexNotMatched,
		},
		"contains upper-case letters": {
			val:     "badGoose",
			svcType: manifestinfo.LoadBalancedWebServiceType,
			wanted:  errBasicNameRegexNotMatched,
		},
		"is not a reserved name": {
			val:     "pipelines",
			svcType: manifestinfo.LoadBalancedWebServiceType,
			wanted:  errValueReserved,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateSvcName(tc.val, tc.svcType)

			require.True(t, errors.Is(got, tc.wanted), "got %v instead of %v", got, tc.wanted)
		})
	}
}

func TestValidateEnvironmentName(t *testing.T) {
	testCases := basicNameTestCases

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateEnvironmentName(tc.input)

			require.True(t, errors.Is(got, tc.want))
		})
	}
}

func TestValidatePipelineName(t *testing.T) {
	testCases := map[string]struct {
		val     interface{}
		appName string

		wanted            error
		wantedErrorSuffix string
	}{
		"string as input": {
			val:    "hello",
			wanted: nil,
		},
		"number as input": {
			val:    1234,
			wanted: errValueNotAString,
		},
		"string with invalid characters": {
			val:    "myPipe!",
			wanted: errBasicNameRegexNotMatched,
		},
		"longer than 128 characters": {
			val:               strings.Repeat("s", 129),
			wantedErrorSuffix: fmt.Sprintf(fmtErrPipelineNameTooLong, 118),
		},
		"longer than 128 characters with pipeline-[app]": {
			val:               strings.Repeat("x", 114),
			appName:           "myApp",
			wantedErrorSuffix: fmt.Sprintf(fmtErrPipelineNameTooLong, 113),
		},
		"does not start with letter": {
			val:    "123chicken",
			wanted: errBasicNameRegexNotMatched,
		},
		"starts with a dash": {
			val:    "-beta",
			wanted: errBasicNameRegexNotMatched,
		},
		"contains upper-case letters": {
			val:    "badGoose",
			wanted: errBasicNameRegexNotMatched,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validatePipelineName(tc.val, tc.appName)

			if tc.wantedErrorSuffix != "" {
				require.True(t, strings.HasSuffix(got.Error(), tc.wantedErrorSuffix), "got %v instead of %v", got, tc.wantedErrorSuffix)
				return
			}

			require.True(t, errors.Is(got, tc.wanted), "got %v instead of %v", got, tc.wanted)
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

func TestValidateRDSName(t *testing.T) {
	testCases := map[string]testCase{
		"good case": {
			input: "goodname",
			want:  nil,
		},
		"too long": {
			input: "AprilisthecruellestmonthbreedingLilacsoutofthedeadlanda",
			want:  fmt.Errorf("value must be between 1 and %d characters in length", 63-len("DBCluster")),
		},
		"bad character": {
			input: "not-good!",
			want:  errInvalidRDSNameCharacters,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := rdsNameValidation(tc.input)
			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	testCases := map[string]struct {
		input interface{}
		want  error
	}{
		"not a string": {
			input: 123,
			want:  errValueNotAString,
		},
		"empty string": {
			input: "",
			want:  errValueEmpty,
		},
		"invalid path": {
			input: "../Dockerfile",
			want:  errValueNotAValidPath,
		},
		"returns nil if valid absolute path": {
			input: "frontend/Dockerfile",
			want:  nil,
		},
		"returns nil if valid relative path": {
			input: "frontend/../backend/Dockerfile",
			want:  nil,
		},
	}
	for path, tc := range testCases {
		t.Run(path, func(t *testing.T) {

			// GIVEN
			fs := &afero.Afero{Fs: afero.NewMemMapFs()}
			fs.MkdirAll("frontend", 0755)
			fs.MkdirAll("backend", 0755)

			afero.WriteFile(fs, "frontend/Dockerfile", []byte("FROM nginx"), 0644)
			afero.WriteFile(fs, "backend/Dockerfile", []byte("FROM nginx"), 0644)

			// WHEN
			got := validatePath(fs, tc.input)

			// THEN
			if tc.want == nil {
				require.Nil(t, got)
			} else {
				require.EqualError(t, tc.want, got.Error())
			}
		})
	}
}

type mockManifestReader struct {
	out workspace.WorkloadManifest
	err error
}

func (m mockManifestReader) ReadWorkloadManifest(name string) (workspace.WorkloadManifest, error) {
	return m.out, m.err
}

func TestValidateStorageType(t *testing.T) {
	testCases := map[string]struct {
		input     string
		optionals validateStorageTypeOpts
		want      error
	}{
		"should allow S3 addons": {
			input: "S3",
			want:  nil,
		},
		"should allow DynamoDB allows": {
			input: "DynamoDB",
			want:  nil,
		},
		"should return an error if a storage type does not exist": {
			input: "Dropbox",
			want:  fmt.Errorf(fmtErrInvalidStorageType, "Dropbox", prettify(storageTypes)),
		},
		"should allow Aurora if workload name is not yet specified": {
			input: "Aurora",
			want:  nil,
		},
		"should return an error if manifest file cannot be read while initializing an Aurora storage type": {
			input: "Aurora",
			optionals: validateStorageTypeOpts{
				ws: mockManifestReader{
					err: errors.New("some error"),
				},
				workloadName: "api",
			},
			want: errors.New("invalid storage type Aurora: read manifest file for api: some error"),
		},
		"should allow Aurora if the workload type is not a RDWS": {
			input: "Aurora",
			optionals: validateStorageTypeOpts{
				ws: mockManifestReader{
					out: []byte(`
name: api
type: Load Balanced Web Service
`),
				},
				workloadName: "api",
			},
		},
		"should return an error if Aurora is selected for a RDWS while not connected to a VPC": {
			input: "Aurora",
			optionals: validateStorageTypeOpts{
				ws: mockManifestReader{
					out: []byte(`
name: api
type: Request-Driven Web Service
`),
				},
				workloadName: "api",
			},
			want: errors.New("invalid storage type Aurora: Request-Driven Web Service requires a VPC connection"),
		},
		"should succeed if Aurora is selected and RDWS is connected to a VPC": {
			input: "Aurora",
			optionals: validateStorageTypeOpts{
				ws: mockManifestReader{
					out: []byte(`
name: api
type: Request-Driven Web Service
network:
  vpc:
    placement: private
`),
				},
				workloadName: "api",
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateStorageType(tc.input, tc.optionals)
			if tc.want == nil {
				require.NoError(t, got)
			} else {
				require.EqualError(t, got, tc.want.Error())
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

func TestValidateLSIs(t *testing.T) {
	testCases := map[string]struct {
		inputAttributes []string
		inputLSIs       []string
		wantError       error
	}{
		"good case": {
			inputLSIs: []string{"userID:S"},
			wantError: nil,
		},
		"bad lsi structure": {
			inputLSIs: []string{"userID"},
			wantError: errDDBAttributeBadFormat,
		},
		"too many lsis": {
			inputLSIs: []string{"bowie:S", "clyde:S", "keno:S", "kava:S", "meow:S", "hana:S"},
			wantError: errTooManyLSIKeys,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateLSIs(tc.inputLSIs)
			if tc.wantError != nil {
				require.EqualError(t, got, tc.wantError.Error())
			} else {
				require.Nil(t, got)
			}
		})
	}
}

func TestValidateCIDR(t *testing.T) {
	testCases := map[string]struct {
		inputCIDR string
		wantError error
	}{
		"good case": {
			inputCIDR: "10.10.10.10/24",
			wantError: nil,
		},
		"bad case": {
			inputCIDR: "10.10.10.10",
			wantError: errValueNotAnIPNet,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateCIDR(tc.inputCIDR)
			if tc.wantError != nil {
				require.EqualError(t, got, tc.wantError.Error())
			} else {
				require.Nil(t, got)
			}
		})
	}
}

func Test_validatePublicSubnetsCIDR(t *testing.T) {
	testCases := map[string]struct {
		in     string
		numAZs int

		wantedErr string
	}{
		"returns nil if CIDRs are valid and match number of available AZs": {
			in:     "10.10.10.10/24,10.10.10.10/24",
			numAZs: 2,
		},
		"returns err if number of CIDRs is not equal to number of available AZs": {
			in:        "10.10.10.10/24,10.10.10.10/24",
			numAZs:    3,
			wantedErr: "number of public subnet CIDRs (2) does not match number of AZs (3)",
		},
		"returns err if input is not valid CIDR fmt": {
			in:        "10.10.10.10,10.10.10.10",
			numAZs:    2,
			wantedErr: errValueNotIPNetSlice.Error(),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actual := validatePublicSubnetsCIDR(tc.numAZs)(tc.in)
			if tc.wantedErr == "" {
				require.NoError(t, actual)
			} else {
				require.EqualError(t, actual, tc.wantedErr)
			}
		})
	}
}

func Test_validatePrivateSubnetsCIDR(t *testing.T) {
	testCases := map[string]struct {
		in     string
		numAZs int

		wantedErr string
	}{
		"returns nil if CIDRs are valid and match number of available AZs": {
			in:     "10.10.10.10/24,10.10.10.10/24",
			numAZs: 2,
		},
		"returns err if number of CIDRs is not equal to number of available AZs": {
			in:        "10.10.10.10/24,10.10.10.10/24",
			numAZs:    3,
			wantedErr: "number of private subnet CIDRs (2) does not match number of AZs (3)",
		},
		"returns err if input is not valid CIDR fmt": {
			in:        "10.10.10.10,10.10.10.10",
			numAZs:    2,
			wantedErr: errValueNotIPNetSlice.Error(),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actual := validatePrivateSubnetsCIDR(tc.numAZs)(tc.in)
			if tc.wantedErr == "" {
				require.NoError(t, actual)
			} else {
				require.EqualError(t, actual, tc.wantedErr)
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
		"contains consecutive dashes": {
			input:   "bad--goose",
			isLegit: false,
		},
		"contains trailing dash": {
			input:   "badgoose-",
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

func TestValidateCron(t *testing.T) {
	testCases := map[string]struct {
		input      string
		shouldPass bool
	}{
		"valid cron expression": {
			input:      "* * * * *",
			shouldPass: true,
		},
		"invalid cron": {
			input:      "* * * * ? *",
			shouldPass: false,
		},
		"valid schedule descriptor": {
			input:      "@every 5m",
			shouldPass: true,
		},
		"invalid schedule": {
			input:      "@every 5 minutes",
			shouldPass: false,
		},
		"bypass with rate()": {
			input:      "rate(la la la)",
			shouldPass: true,
		},
		"bypass with cron()": {
			input:      "cron(0 9 3W * ? *)",
			shouldPass: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateCron(tc.input)
			if tc.shouldPass {
				require.NoError(t, got)
			} else {
				require.NotNil(t, got)
			}
		})
	}
}

func TestValidateEngine(t *testing.T) {
	testCases := map[string]testCase{
		"mysql": {
			input: "MySQL",
			want:  nil,
		},
		"postgresql": {
			input: "PostgreSQL",
			want:  nil,
		},
		"invalid engine type": {
			input: "weird-engine",
			want:  errors.New("invalid engine type weird-engine: must be one of \"MySQL\", \"PostgreSQL\""),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateEngine(tc.input)
			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func TestValidateMySQLDBName(t *testing.T) {
	testCases := map[string]testCase{
		"good case": {
			input: "my_db_123_",
			want:  nil,
		},
		"too long": {
			input: "April-is-the-cruellest-month-breeding-Lilacs-out-of-the-dead-land-m",
			want:  errors.New("value must be between 1 and 64 characters in length"),
		},
		"bad character": {
			input: "bad_db_name:(",
			want:  fmt.Errorf(fmtErrInvalidDBNameCharacters, "bad_db_name:("),
		},
		"bad starting character": {
			input: "_not_good",
			want:  fmt.Errorf(fmtErrInvalidDBNameCharacters, "_not_good"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateMySQLDBName(tc.input)
			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func TestValidatePostgreSQLDBName(t *testing.T) {
	testCases := map[string]testCase{
		"good case": {
			input: "my_db_123_",
			want:  nil,
		},
		"too long": {
			input: "April-is-the-cruellest-month-breeding-Lilacs-out-of-the-dead-land-m",
			want:  errors.New("value must be between 1 and 63 characters in length"),
		},
		"bad character": {
			input: "bad_db_name:(",
			want:  fmt.Errorf(fmtErrInvalidDBNameCharacters, "bad_db_name:("),
		},
		"bad starting character": {
			input: "_not_good",
			want:  fmt.Errorf(fmtErrInvalidDBNameCharacters, "_not_good"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validatePostgreSQLDBName(tc.input)
			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func TestValidateSecretName(t *testing.T) {
	testCases := map[string]testCase{
		"bad character": {
			input: "bad!",
			want:  errInvalidSecretNameCharacters,
		},
		"bad character space": {
			input: "bad ",
			want:  errInvalidSecretNameCharacters,
		},
		"secret name too short": {
			input: "",
			want:  fmt.Errorf(fmtErrValueBadSize, 1, 2048-(len("/copilot/")+len("/")+len("/secrets/"))),
		},
		"secret name too long": {
			input: string(make([]rune, 2048)),
			want:  fmt.Errorf(fmtErrValueBadSize, 1, 2048-(len("/copilot/")+len("/")+len("/secrets/"))),
		},
		"valid secret name": {
			input: "secret.name",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateSecretName(tc.input)
			if tc.want != nil {
				require.EqualError(t, got, tc.want.Error())
			} else {
				require.NoError(t, got)
			}
		})
	}
}

func Test_validatePubSubTopicName(t *testing.T) {
	testCases := map[string]struct {
		inName string

		wantErr error
	}{
		"valid topic name": {
			inName: "a-Perfectly_V4l1dString",
		},
		"error when no topic name": {
			inName:  "",
			wantErr: errMissingPublishTopicField,
		},
		"error when invalid topic name": {
			inName:  "OHNO~/`...,",
			wantErr: errInvalidPubSubTopicName,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validatePubSubName(tc.inName)
			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}

func Test_validateSubscriptionKey(t *testing.T) {
	testCases := map[string]struct {
		inSub interface{}

		wantErr error
	}{
		"valid subscription": {
			inSub:   "svc:topic",
			wantErr: nil,
		},
		"error when non string": {
			inSub:   true,
			wantErr: errValueNotAString,
		},
		"error when bad format": {
			inSub:   "svctopic",
			wantErr: errSubscribeBadFormat,
		},
		"error when bad publisher name": {
			inSub:   "svc:@@@@@@@h",
			wantErr: fmt.Errorf("invalid topic subscription topic name `@@@@@@@h`: %w", errInvalidPubSubTopicName),
		},
		"error when bad svc name": {
			inSub:   "n#######:topic",
			wantErr: fmt.Errorf("invalid topic subscription service name `n#######`: %w", errBasicNameRegexNotMatched),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateSubscriptionKey(tc.inSub)
			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}

func Test_validateSubscribe(t *testing.T) {
	testCases := map[string]struct {
		inNoSubscriptions bool
		inSubscribeTags   []string

		wantErr error
	}{
		"valid subscription": {
			inNoSubscriptions: false,
			inSubscribeTags:   []string{"svc1:topic1", "svc2:topic2"},
			wantErr:           nil,
		},
		"no error when no subscriptions": {
			inNoSubscriptions: true,
			inSubscribeTags:   nil,
			wantErr:           nil,
		},
		"error when no-subscriptions and subscribe": {
			inNoSubscriptions: true,
			inSubscribeTags:   []string{"svc1:topic1", "svc2:topic2"},
			wantErr:           errors.New("validate subscribe configuration: cannot specify both --no-subscribe and --subscribe-topics"),
		},
		"error when bad subscription tag": {
			inNoSubscriptions: false,
			inSubscribeTags:   []string{"svc:topic", "svc:@@@@@@@h"},
			wantErr:           fmt.Errorf("invalid topic subscription topic name `@@@@@@@h`: %w", errInvalidPubSubTopicName),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := validateSubscribe(tc.inNoSubscriptions, tc.inSubscribeTags)
			if tc.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}
}

func TestValidateJobName(t *testing.T) {
	testCases := map[string]struct {
		val    interface{}
		wanted error
	}{
		"string as input": {
			val:    "hello",
			wanted: nil,
		},
		"number as input": {
			val:    1234,
			wanted: errValueNotAString,
		},
		"is not a reserved name": {
			val:    "pipelines",
			wanted: errValueReserved,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got := validateJobName(tc.val)
			require.True(t, errors.Is(got, tc.wanted), "got %v instead of %v", got, tc.wanted)
		})
	}
}

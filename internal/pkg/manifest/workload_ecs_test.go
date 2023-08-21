// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestExec_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		inContent []byte

		wantedStruct ExecuteCommand
		wantedError  error
	}{
		"use default with empty value": {
			inContent: []byte(`exec:
count: 1`),

			wantedStruct: ExecuteCommand{
				Enable: aws.Bool(false),
			},
		},
		"use default without any input": {
			inContent: []byte(`count: 1`),

			wantedStruct: ExecuteCommand{
				Enable: aws.Bool(false),
			},
		},
		"simple enable": {
			inContent: []byte(`exec: true`),

			wantedStruct: ExecuteCommand{
				Enable: aws.Bool(true),
			},
		},
		"with config": {
			inContent: []byte(`exec:
  enable: true`),
			wantedStruct: ExecuteCommand{
				Enable: aws.Bool(false),
				Config: ExecuteCommandConfig{
					Enable: aws.Bool(true),
				},
			},
		},
		"Error if unmarshalable": {
			inContent: []byte(`exec:
  badfield: OH NOES
  otherbadfield: DOUBLE BAD`),
			wantedError: errUnmarshalExec,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			b := TaskConfig{
				ExecuteCommand: ExecuteCommand{
					Enable: aws.Bool(false),
				},
			}
			err := yaml.Unmarshal(tc.inContent, &b)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				// check memberwise dereferenced pointer equality
				require.Equal(t, tc.wantedStruct.Enable, b.ExecuteCommand.Enable)
				require.Equal(t, tc.wantedStruct.Config, b.ExecuteCommand.Config)
			}
		})
	}
}

func TestVariable_UnmarshalYAML(t *testing.T) {
	type mockParentField struct {
		Variables map[string]Variable `yaml:"variables"`
	}
	testCases := map[string]struct {
		in          []byte
		wanted      mockParentField
		wantedError error
	}{
		"unmarshal plain string": {
			in: []byte(`
variables:
  LOG_LEVEL: DEBUG
`),
			wanted: mockParentField{
				Variables: map[string]Variable{
					"LOG_LEVEL": {
						StringOrFromCFN{
							Plain: stringP("DEBUG"),
						},
					},
				},
			},
		},
		"unmarshal import name": {
			in: []byte(`
variables:
  DB_NAME:
    from_cfn: MyUserDB
`),
			wanted: mockParentField{
				Variables: map[string]Variable{
					"DB_NAME": {
						StringOrFromCFN{
							FromCFN: fromCFN{
								Name: stringP("MyUserDB"),
							},
						},
					},
				},
			},
		},
		"nothing to unmarshal": {
			in: []byte(`other_field: yo`),
		},
		"fail to unmarshal": {
			in: []byte(`
variables:
  erroneous: 
    big_mistake: being made`),
			wantedError: errors.New(`unmarshal "variables": cannot unmarshal field to a string or into a map`),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var s mockParentField
			err := yaml.Unmarshal(tc.in, &s)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, s)
			}
		})
	}
}

func TestVariable_RequiresImport(t *testing.T) {
	testCases := map[string]struct {
		in     Variable
		wanted bool
	}{
		"requires import": {
			in: Variable{
				StringOrFromCFN{
					FromCFN: fromCFN{
						Name: stringP("prod-MyDB"),
					},
				},
			},
			wanted: true,
		},
		"does not require import if it is a plain value": {
			in: Variable{
				StringOrFromCFN{
					Plain: stringP("plain"),
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.RequiresImport())
		})
	}
}

func TestVariable_Value(t *testing.T) {
	testCases := map[string]struct {
		in     Variable
		wanted string
	}{
		"requires import": {
			in: Variable{
				StringOrFromCFN{
					FromCFN: fromCFN{
						Name: stringP("prod-MyDB"),
					},
				},
			},
			wanted: "prod-MyDB",
		},
		"does not require import if it is a plain value": {
			in: Variable{
				StringOrFromCFN{
					Plain: stringP("plain"),
				},
			},
			wanted: "plain",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.Value())
		})
	}
}

func TestSecret_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		in string

		wanted    Secret
		wantedErr error
	}{
		"should return an error if the input cannot be unmarshal to a Secret": {
			in:        "key: value",
			wantedErr: errors.New(`cannot marshal "secret" field to a string or "secretsmanager" object`),
		},
		"should be able to unmarshal a plain SSM parameter name": {
			in: "/github/token",
			wanted: Secret{
				from: StringOrFromCFN{
					Plain: aws.String("/github/token"),
				},
			},
		},
		"should be able to unmarshal an imported SSM parameter name from other cloudformation stack": {
			in: `from_cfn: "stack-SSMGHTokenName"`,
			wanted: Secret{
				from: StringOrFromCFN{
					FromCFN: fromCFN{
						Name: aws.String("stack-SSMGHTokenName"),
					},
				},
			},
		},
		"should be able to unmarshal a plain SecretsManager ARN": {
			in: "arn:aws:secretsmanager:us-west-2:111122223333:secret:aes128-1a2b3c",
			wanted: Secret{
				from: StringOrFromCFN{
					Plain: aws.String("arn:aws:secretsmanager:us-west-2:111122223333:secret:aes128-1a2b3c"),
				},
			},
		},
		"should be able to unmarshal a SecretsManager name": {
			in:     "secretsmanager: aes128-1a2b3c",
			wanted: Secret{fromSecretsManager: secretsManagerSecret{Name: aws.String("aes128-1a2b3c")}},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			actual := Secret{}

			// WHEN
			err := yaml.Unmarshal([]byte(tc.in), &actual)

			// THEN
			if tc.wantedErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wanted, actual)
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}

func TestSecret_IsSecretsManagerName(t *testing.T) {
	testCases := map[string]struct {
		in     Secret
		wanted bool
	}{
		"should return false if the secret refers to an SSM parameter": {
			in: Secret{
				from: StringOrFromCFN{
					Plain: aws.String("/github/token"),
				},
			},
		},
		"should return true if the secret refers to a SecretsManager secret name": {
			in:     Secret{fromSecretsManager: secretsManagerSecret{Name: aws.String("aes128-1a2b3c")}},
			wanted: true,
		},
		"should return false if the secret is imported": {
			in: Secret{
				from: StringOrFromCFN{
					FromCFN: fromCFN{aws.String("stack-SSMGHTokenName")},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.IsSecretsManagerName())
		})
	}
}

func TestSSMOrSecretARN_RequiresImport(t *testing.T) {
	testCases := map[string]struct {
		in     Secret
		wanted bool
	}{
		"should return false if secret is plain": {
			in: Secret{
				from: StringOrFromCFN{
					Plain: aws.String("aes128-1a2b3c"),
				},
			},
		},
		"should return true if secret is imported": {
			in: Secret{
				from: StringOrFromCFN{
					FromCFN: fromCFN{
						Name: aws.String("stack-SSMGHTokenName"),
					},
				},
			},
			wanted: true,
		},
		"should return false if secret is from secrets manager": {
			in: Secret{
				fromSecretsManager: secretsManagerSecret{
					Name: aws.String("aes128-1a2b3c"),
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.RequiresImport())
		})
	}
}

func TestSecret_Value(t *testing.T) {
	testCases := map[string]struct {
		in     Secret
		wanted string
	}{
		"should return the SSM parameter name if the secret is just a string": {
			in: Secret{
				from: StringOrFromCFN{
					Plain: aws.String("/github/token"),
				},
			},
			wanted: "/github/token",
		},
		"should return the imported name of the SSM parameter or secretARN": {
			in: Secret{
				from: StringOrFromCFN{
					FromCFN: fromCFN{
						Name: aws.String("stack-SSMGHTokenName"),
					},
				},
			},
			wanted: "stack-SSMGHTokenName",
		},
		"should return the SecretsManager secret name when the secret is from SecretsManager": {
			in:     Secret{fromSecretsManager: secretsManagerSecret{Name: aws.String("aes128-1a2b3c")}},
			wanted: "aes128-1a2b3c",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.Value())
		})
	}
}

func TestSecretsManagerSecret_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     secretsManagerSecret
		wanted bool
	}{
		"should return true on empty struct": {in: secretsManagerSecret{}, wanted: true},
		"should return false if the name is provided": {
			in: secretsManagerSecret{
				Name: aws.String("aes128-1a2b3c"),
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.IsEmpty())
		})
	}
}

func TestLogging_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     Logging
		wanted bool
	}{
		"empty logging": {
			in:     Logging{},
			wanted: true,
		},
		"non empty logging": {
			in: Logging{
				SecretOptions: map[string]Secret{
					"secret1": {
						from: StringOrFromCFN{
							Plain: aws.String("value1"),
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			got := tc.in.IsEmpty()

			// THEN
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestLogging_LogImage(t *testing.T) {
	testCases := map[string]struct {
		inputImage  *string
		wantedImage *string
	}{
		"Image specified": {
			inputImage:  aws.String("nginx:why-on-earth"),
			wantedImage: aws.String("nginx:why-on-earth"),
		},
		"no image specified": {
			inputImage:  nil,
			wantedImage: aws.String(defaultFluentbitImage),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			l := Logging{
				Image: tc.inputImage,
			}
			got := l.LogImage()

			require.Equal(t, tc.wantedImage, got)
		})
	}
}

func TestLogging_GetEnableMetadata(t *testing.T) {
	testCases := map[string]struct {
		enable *bool
		wanted *string
	}{
		"specified true": {
			enable: aws.Bool(true),
			wanted: aws.String("true"),
		},
		"specified false": {
			enable: aws.Bool(false),
			wanted: aws.String("false"),
		},
		"not specified": {
			enable: nil,
			wanted: aws.String("true"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			l := Logging{
				EnableMetadata: tc.enable,
			}
			got := l.GetEnableMetadata()

			require.Equal(t, tc.wanted, got)
		})
	}
}

func Test_ImageURI(t *testing.T) {
	testCases := map[string]struct {
		in        SidecarConfig
		wantedURI string
		wantedOk  bool
	}{
		"empty SidecarConfig": {},
		"should return URI if provided directly through `image` ": {
			in: SidecarConfig{
				Image: Union[*string, ImageLocationOrBuild]{
					Basic: aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
				},
			},
			wantedURI: "123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon",
			wantedOk:  true,
		},
		"should return the URI if provided through `image.location` field": {
			in: SidecarConfig{
				Image: Union[*string, ImageLocationOrBuild]{
					Advanced: ImageLocationOrBuild{
						Location: aws.String("123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon"),
					},
				},
			},
			wantedURI: "123456789012.dkr.ecr.us-east-2.amazonaws.com/xray-daemon",
			wantedOk:  true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			uri, ok := tc.in.ImageURI()

			// THEN
			require.Equal(t, tc.wantedURI, uri)
			require.Equal(t, tc.wantedOk, ok)
		})
	}
}

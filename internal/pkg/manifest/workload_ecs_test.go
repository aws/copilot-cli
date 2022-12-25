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
						stringOrFromCFN{
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
						stringOrFromCFN{
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
				stringOrFromCFN{
					FromCFN: fromCFN{
						Name: stringP("prod-MyDB"),
					},
				},
			},
			wanted: true,
		},
		"does not require import if it is a plain value": {
			in: Variable{
				stringOrFromCFN{
					Plain: stringP("plain"),
				},
			},
			wanted: false,
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
				stringOrFromCFN{
					FromCFN: fromCFN{
						Name: stringP("prod-MyDB"),
					},
				},
			},
			wanted: "prod-MyDB",
		},
		"does not require import if it is a plain value": {
			in: Variable{
				stringOrFromCFN{
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
		"should be able to unmarshal an plain SSM parameter name": {
			in: "/github/token",
			wanted: Secret{from: stringOrFromCFN{
				Plain: aws.String("/github/token"),
			},
			},
		},
		"should be able to unmarshal an imported SSM parameter name from other cloudformation stack": {
			in: `from_cfn: "/github/token"`,
			wanted: Secret{from: stringOrFromCFN{
				FromCFN: fromCFN{
					Name: aws.String("/github/token"),
				},
			},
			},
		},
		"should be able to unmarshal a plain SecretsManager ARN": {
			in: "arn:aws:secretsmanager:us-west-2:111122223333:secret:aes128-1a2b3c",
			wanted: Secret{from: stringOrFromCFN{
				Plain: aws.String("arn:aws:secretsmanager:us-west-2:111122223333:secret:aes128-1a2b3c"),
			},
			},
		},
		"should be able to unmarshal a plain SecretsManager name": {
			in: "secretsmanager: aes128-1a2b3c",
			wanted: Secret{fromSecretsManager: secretsManagerSecret{
				SecretName: stringOrFromCFN{
					Plain: aws.String("aes128-1a2b3c"),
				},
			},
			},
		},
		// "should be able to unmarshal a imported SecretsManager name from othercloudformation stack":{

		// }
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
			in: Secret{from: stringOrFromCFN{
				Plain: aws.String("/github/token"),
			},
			},
		},
		"should return true if the secret refers to a SecretsManager secret name": {
			in: Secret{fromSecretsManager: secretsManagerSecret{
				stringOrFromCFN{
					Plain: aws.String("aes128-1a2b3c"),
				},
			},
			},
			wanted: true,
		},
		"should return true if the secret is imported": {
			in: Secret{fromSecretsManager: secretsManagerSecret{
				stringOrFromCFN{
					FromCFN: fromCFN{
						Name: aws.String("aes128-1a2b3c"),
					},
				},
			},
			},
			wanted: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.IsSecretsManagerName())
		})
	}
}

func TestSSMRequiresImport(t *testing.T) {
	testCases := map[string]struct {
		in     Secret
		wanted bool
	}{
		"should return false if secret is plain": {
			in: Secret{from: stringOrFromCFN{
				Plain: aws.String("aes128-1a2b3c"),
			},
			},
			wanted: false,
		},
		"should return true if secret is imported": {
			in: Secret{from: stringOrFromCFN{
				FromCFN: fromCFN{
					Name: aws.String("mygithubtoken"),
				},
			},
			},
			wanted: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.SSMRequiresImport())
		})
	}
}

func TestSecretsManagerRequiresImport(t *testing.T) {
	testCases := map[string]struct {
		in     Secret
		wanted bool
	}{
		"should return false if secret is plain": {
			in: Secret{fromSecretsManager: secretsManagerSecret{
				SecretName: stringOrFromCFN{
					Plain: aws.String("aes128-1a2b3c"),
				},
			},
			},
			wanted: false,
		},
		"should return true if secret is imported": {
			in: Secret{fromSecretsManager: secretsManagerSecret{
				SecretName: stringOrFromCFN{
					FromCFN: fromCFN{
						Name: aws.String("mygithubtoken"),
					},
				},
			},
			},
			wanted: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.SecretsManagerRequiresImport())
		})
	}
}

func TestSecret_Value(t *testing.T) {
	testCases := map[string]struct {
		in     Secret
		wanted string
	}{
		"should return the SSM parameter name if the secret is just a string": {
			in: Secret{from: stringOrFromCFN{
				Plain: aws.String("/github/token"),
			}},
			wanted: "/github/token",
		},
		"should return the imported name of the SSM parameter or secretARN": {
			in: Secret{from: stringOrFromCFN{
				FromCFN: fromCFN{
					Name: aws.String("mygithubtoken"),
				},
			},
			},
			wanted: "mygithubtoken",
		},
		"should return the SecretsManager secret name when the secret is from SecretsManager": {
			in: Secret{
				fromSecretsManager: secretsManagerSecret{
					SecretName: stringOrFromCFN{
						Plain: aws.String("aes128-1a2b3c"),
					},
				},
			},
			wanted: "aes128-1a2b3c",
		},
		"should return the imported name from ClodFormation when the secret is from SecretsManager": {
			in: Secret{
				fromSecretsManager: secretsManagerSecret{
					SecretName: stringOrFromCFN{
						FromCFN: fromCFN{
							Name: aws.String("mydbusername"),
						},
					},
				},
			},
			wanted: "mydbusername",
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
				SecretName: stringOrFromCFN{
					Plain: aws.String("aes128-1a2b3c"),
				},
			},
		},
		"should return false if the name is an imported secretmanager": {
			in: secretsManagerSecret{
				SecretName: stringOrFromCFN{
					FromCFN: fromCFN{
						Name: aws.String("mygithubtoken"),
					},
				},
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
					"secret1": {from: stringOrFromCFN{
						Plain: aws.String("value1"),
					}},
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

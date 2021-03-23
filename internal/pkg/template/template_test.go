// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/gobuffalo/packd"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestTemplate_Read(t *testing.T) {
	testCases := map[string]struct {
		inPath           string
		mockDependencies func(t *Template)

		wantedContent string
		wantedErr     error
	}{
		"template does not exist": {
			inPath: "/fake/manifest.yml",
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				t.box = mockBox
			},

			wantedErr: errors.New("read template /fake/manifest.yml"),
		},
		"returns content": {
			inPath: "/fake/manifest.yml",
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				mockBox.AddString("/fake/manifest.yml", "hello")
				t.box = mockBox
			},

			wantedContent: "hello",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{}
			tc.mockDependencies(tpl)

			// WHEN
			c, err := tpl.Read(tc.inPath)

			if tc.wantedErr != nil {
				require.Contains(t, err.Error(), tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedContent, c.String())
			}
		})
	}
}

func TestTemplate_UploadEnvironmentCustomResources(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(t *Template)

		wantedErr error
	}{
		"success": {
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				for _, file := range envCustomResourceFiles {
					mockBox.AddString(fmt.Sprintf("custom-resources/%s.js", file), "hello")
				}
				t.box = mockBox
			},
		},
		"errors if env custom resource file doesn't exist": {
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				mockBox.AddString("badFile", "hello")
				t.box = mockBox
			},
			wantedErr: fmt.Errorf("read template custom-resources/dns-cert-validator.js: file does not exist"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{}
			tc.mockDependencies(tpl)
			mockUploader := s3.CompressAndUploadFunc(func(key string, files ...s3.NamedBinary) (string, error) {
				require.Contains(t, key, "scripts")
				require.Contains(t, key, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824")
				return "mockURL", nil
			})

			// WHEN
			gotCustomResources, err := tpl.UploadEnvironmentCustomResources(mockUploader)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, len(envCustomResourceFiles), len(gotCustomResources))
			}
		})
	}
}

func TestTemplate_Parse(t *testing.T) {
	testCases := map[string]struct {
		inPath           string
		inData           interface{}
		mockDependencies func(t *Template)

		wantedContent string
		wantedErr     error
	}{
		"template does not exist": {
			inPath: "/fake/manifest.yml",
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				t.box = mockBox
			},

			wantedErr: errors.New("read template /fake/manifest.yml"),
		},
		"template cannot be parsed": {
			inPath: "/fake/manifest.yml",
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				mockBox.AddString("/fake/manifest.yml", `{{}}`)
				t.box = mockBox
			},

			wantedErr: errors.New("parse template /fake/manifest.yml"),
		},
		"template cannot be executed": {
			inPath: "/fake/manifest.yml",
			inData: struct{}{},
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				mockBox.AddString("/fake/manifest.yml", `{{.Name}}`)
				t.box = mockBox
			},

			wantedErr: fmt.Errorf("execute template %s with data %v", "/fake/manifest.yml", struct{}{}),
		},
		"valid template": {
			inPath: "/fake/manifest.yml",
			inData: struct {
				Name string
			}{
				Name: "webhook",
			},
			mockDependencies: func(t *Template) {
				mockBox := packd.NewMemoryBox()
				mockBox.AddString("/fake/manifest.yml", `{{.Name}}`)
				t.box = mockBox
			},

			wantedContent: "webhook",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := &Template{}
			tc.mockDependencies(tpl)

			// WHEN
			c, err := tpl.Parse(tc.inPath, tc.inData)

			if tc.wantedErr != nil {
				require.Contains(t, err.Error(), tc.wantedErr.Error())
			} else {
				require.Equal(t, tc.wantedContent, c.String())
			}
		})
	}
}

func TestTemplate_ParseNetwork(t *testing.T) {
	type cfn struct {
		Resources struct {
			Service struct {
				Properties struct {
					NetworkConfiguration map[interface{}]interface{} `yaml:"NetworkConfiguration"`
				} `yaml:"Properties"`
			} `yaml:"Service"`
		} `yaml:"Resources"`
	}

	testCases := map[string]struct {
		input *NetworkOpts

		wantedNetworkConfig string
	}{
		"should render AWS VPC configuration for public subnets by default": {
			input: nil,
			wantedNetworkConfig: `
  AwsvpcConfiguration:
    AssignPublicIp: ENABLED
    Subnets:
    - Fn::Select:
      - 0
      - Fn::Split:
        - ','
        - Fn::ImportValue: !Sub '${AppName}-${EnvName}-PublicSubnets'
    - Fn::Select:
      - 1
      - Fn::Split:
        - ','
        - Fn::ImportValue: !Sub '${AppName}-${EnvName}-PublicSubnets'
    SecurityGroups:
      - Fn::ImportValue: !Sub '${AppName}-${EnvName}-EnvironmentSecurityGroup'
`,
		},
		"should render AWS VPC configuration for private subnets": {
			input: &NetworkOpts{
				AssignPublicIP: "DISABLED",
				SubnetsType:    "PrivateSubnets",
			},
			wantedNetworkConfig: `
  AwsvpcConfiguration:
    AssignPublicIp: DISABLED
    Subnets:
    - Fn::Select:
      - 0
      - Fn::Split:
        - ','
        - Fn::ImportValue: !Sub '${AppName}-${EnvName}-PrivateSubnets'
    - Fn::Select:
      - 1
      - Fn::Split:
        - ','
        - Fn::ImportValue: !Sub '${AppName}-${EnvName}-PrivateSubnets'
    SecurityGroups:
      - Fn::ImportValue: !Sub '${AppName}-${EnvName}-EnvironmentSecurityGroup'
`,
		},
		"should render AWS VPC configuration for private subnets with security groups": {
			input: &NetworkOpts{
				AssignPublicIP: "DISABLED",
				SubnetsType:    "PrivateSubnets",
				SecurityGroups: []string{
					"sg-1bcf1d5b",
					"sg-asdasdas",
				},
			},
			wantedNetworkConfig: `
  AwsvpcConfiguration:
    AssignPublicIp: DISABLED
    Subnets:
    - Fn::Select:
      - 0
      - Fn::Split:
        - ','
        - Fn::ImportValue: !Sub '${AppName}-${EnvName}-PrivateSubnets'
    - Fn::Select:
      - 1
      - Fn::Split:
        - ','
        - Fn::ImportValue: !Sub '${AppName}-${EnvName}-PrivateSubnets'
    SecurityGroups:
      - Fn::ImportValue: !Sub '${AppName}-${EnvName}-EnvironmentSecurityGroup'
      - "sg-1bcf1d5b"
      - "sg-asdasdas"
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tpl := New()
			wanted := make(map[interface{}]interface{})
			err := yaml.Unmarshal([]byte(tc.wantedNetworkConfig), &wanted)
			require.NoError(t, err, "unmarshal wanted config")

			// WHEN
			content, err := tpl.ParseLoadBalancedWebService(WorkloadOpts{
				Network: tc.input,
			})

			// THEN
			require.NoError(t, err, "parse load balanced web service")
			var actual cfn
			err = yaml.Unmarshal(content.Bytes(), &actual)
			require.NoError(t, err, "unmarshal actual config")
			require.Equal(t, wanted, actual.Resources.Service.Properties.NetworkConfiguration)
		})
	}
}

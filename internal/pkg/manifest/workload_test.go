// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestBuildArgs_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		inContent []byte

		wantedStruct BuildArgsOrString
		wantedError  error
	}{
		"legacy case: simple build string": {
			inContent: []byte(`build: ./Dockerfile`),

			wantedStruct: BuildArgsOrString{
				BuildString: aws.String("./Dockerfile"),
			},
		},
		"Dockerfile specified in build opts": {
			inContent: []byte(`build:
  dockerfile: path/to/Dockerfile
`),
			wantedStruct: BuildArgsOrString{
				BuildArgs: DockerBuildArgs{
					Dockerfile: aws.String("path/to/Dockerfile"),
				},
				BuildString: nil,
			},
		},
		"Dockerfile context, and args specified in build opts": {
			inContent: []byte(`build:
  dockerfile: path/to/Dockerfile
  args:
    arg1: value1
    bestdog: bowie
  context: path/to/source`),
			wantedStruct: BuildArgsOrString{
				BuildArgs: DockerBuildArgs{
					Dockerfile: aws.String("path/to/Dockerfile"),
					Context:    aws.String("path/to/source"),
					Args: map[string]string{
						"arg1":    "value1",
						"bestdog": "bowie",
					},
				},
				BuildString: nil,
			},
		},
		"Dockerfile with cache from and target build opts": {
			inContent: []byte(`build:
  cache_from:
    - foo/bar:latest
    - foo/bar/baz:1.2.3
  target: foobar`),
			wantedStruct: BuildArgsOrString{
				BuildArgs: DockerBuildArgs{
					Target: aws.String("foobar"),
					CacheFrom: []string{
						"foo/bar:latest",
						"foo/bar/baz:1.2.3",
					},
				},
				BuildString: nil,
			},
		},
		"Error if unmarshalable": {
			inContent: []byte(`build:
  badfield: OH NOES
  otherbadfield: DOUBLE BAD`),
			wantedError: errUnmarshalBuildOpts,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			b := Image{
				Build: BuildArgsOrString{
					BuildString: aws.String("./default"),
				},
			}
			err := yaml.Unmarshal(tc.inContent, &b)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				// check memberwise dereferenced pointer equality
				require.Equal(t, tc.wantedStruct.BuildString, b.Build.BuildString)
				require.Equal(t, tc.wantedStruct.BuildArgs.Context, b.Build.BuildArgs.Context)
				require.Equal(t, tc.wantedStruct.BuildArgs.Dockerfile, b.Build.BuildArgs.Dockerfile)
				require.Equal(t, tc.wantedStruct.BuildArgs.Args, b.Build.BuildArgs.Args)
				require.Equal(t, tc.wantedStruct.BuildArgs.Target, b.Build.BuildArgs.Target)
				require.Equal(t, tc.wantedStruct.BuildArgs.CacheFrom, b.Build.BuildArgs.CacheFrom)
			}
		})
	}
}

func TestBuildConfig(t *testing.T) {
	mockWsRoot := "/root/dir"
	testCases := map[string]struct {
		inBuild     BuildArgsOrString
		wantedBuild DockerBuildArgs
	}{
		"simple case: BuildString path to dockerfile": {
			inBuild: BuildArgsOrString{
				BuildString: aws.String("my/Dockerfile"),
			},
			wantedBuild: DockerBuildArgs{
				Dockerfile: aws.String(filepath.Join(mockWsRoot, "my/Dockerfile")),
				Context:    aws.String(filepath.Join(mockWsRoot, "my")),
			},
		},
		"Different context than dockerfile": {
			inBuild: BuildArgsOrString{
				BuildArgs: DockerBuildArgs{
					Dockerfile: aws.String("build/dockerfile"),
					Context:    aws.String("cmd/main"),
				},
			},
			wantedBuild: DockerBuildArgs{
				Dockerfile: aws.String(filepath.Join(mockWsRoot, "build/dockerfile")),
				Context:    aws.String(filepath.Join(mockWsRoot, "cmd/main")),
			},
		},
		"no dockerfile specified": {
			inBuild: BuildArgsOrString{
				BuildArgs: DockerBuildArgs{
					Context: aws.String("cmd/main"),
				},
			},
			wantedBuild: DockerBuildArgs{
				Dockerfile: aws.String(filepath.Join(mockWsRoot, "cmd", "main", "Dockerfile")),
				Context:    aws.String(filepath.Join(mockWsRoot, "cmd", "main")),
			},
		},
		"no dockerfile or context specified": {
			inBuild: BuildArgsOrString{
				BuildArgs: DockerBuildArgs{
					Args: map[string]string{
						"goodDog": "bowie",
					},
				},
			},
			wantedBuild: DockerBuildArgs{
				Dockerfile: aws.String(filepath.Join(mockWsRoot, "Dockerfile")),
				Context:    aws.String(mockWsRoot),
				Args: map[string]string{
					"goodDog": "bowie",
				},
			},
		},
		"including args": {
			inBuild: BuildArgsOrString{
				BuildArgs: DockerBuildArgs{
					Dockerfile: aws.String("my/Dockerfile"),
					Args: map[string]string{
						"goodDog":  "bowie",
						"badGoose": "HONK",
					},
				},
			},
			wantedBuild: DockerBuildArgs{
				Dockerfile: aws.String(filepath.Join(mockWsRoot, "my/Dockerfile")),
				Context:    aws.String(filepath.Join(mockWsRoot, "my")),
				Args: map[string]string{
					"goodDog":  "bowie",
					"badGoose": "HONK",
				},
			},
		},
		"including build options": {
			inBuild: BuildArgsOrString{
				BuildArgs: DockerBuildArgs{
					Target: aws.String("foobar"),
					CacheFrom: []string{
						"foo/bar:latest",
						"foo/bar/baz:1.2.3",
					},
				},
			},
			wantedBuild: DockerBuildArgs{
				Dockerfile: aws.String(filepath.Join(mockWsRoot, "Dockerfile")),
				Context:    aws.String(mockWsRoot),
				Target:     aws.String("foobar"),
				CacheFrom: []string{
					"foo/bar:latest",
					"foo/bar/baz:1.2.3",
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			s := Image{
				Build: tc.inBuild,
			}
			got := s.BuildConfig(mockWsRoot)

			require.Equal(t, tc.wantedBuild, *got)
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

func TestNetworkConfig_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		data string

		wantedConfig NetworkConfig
		wantedErr    error
	}{
		"defaults to public placement if vpc is empty": {
			data: `
network:
  vpc:
`,
			wantedConfig: NetworkConfig{
				VPC: vpcConfig{
					Placement: stringP(PublicSubnetPlacement),
				},
			},
		},
		"returns error if placement option is invalid": {
			data: `
network:
  vpc:
    placement: 'tartarus'
`,
			wantedErr: errors.New(`field 'network.vpc.placement' is 'tartarus' must be one of []string{"public", "private"}`),
		},
		"unmarshals successfully for public placement with security groups": {
			data: `
network:
  vpc:
    placement: 'public'
    security_groups:
    - 'sg-1234'
    - 'sg-4567'
`,
			wantedConfig: NetworkConfig{
				VPC: vpcConfig{
					Placement:      stringP(PublicSubnetPlacement),
					SecurityGroups: []string{"sg-1234", "sg-4567"},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			type manifest struct {
				Network NetworkConfig `yaml:"network"`
			}
			var m manifest

			// WHEN
			err := yaml.Unmarshal([]byte(tc.data), &m)

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedConfig, m.Network)
			}
		})
	}
}

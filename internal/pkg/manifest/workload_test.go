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

func TestEntryPointOverride_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		inContent []byte

		wantedStruct EntryPointOverride
		wantedError  error
	}{
		"Entrypoint specified in string": {
			inContent: []byte(`entrypoint: echo hello`),
			wantedStruct: EntryPointOverride{
				String:      aws.String("echo hello"),
				StringSlice: nil,
			},
		},
		"Entrypoint specified in slice of strings": {
			inContent: []byte(`entrypoint: ["/bin/sh", "-c"]`),
			wantedStruct: EntryPointOverride{
				String:      nil,
				StringSlice: []string{"/bin/sh", "-c"},
			},
		},
		"Error if unmarshalable": {
			inContent: []byte(`entrypoint: {"/bin/sh", "-c"}`),
			wantedStruct: EntryPointOverride{
				String:      nil,
				StringSlice: nil,
			},
			wantedError: errUnmarshalEntryPoint,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			e := ImageOverride{
				EntryPoint: &EntryPointOverride{
					String: aws.String("wrong"),
				},
			}

			err := yaml.Unmarshal(tc.inContent, &e)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				// check memberwise dereferenced pointer equality
				require.Equal(t, tc.wantedStruct.StringSlice, e.EntryPoint.StringSlice)
				require.Equal(t, tc.wantedStruct.String, e.EntryPoint.String)
			}
		})
	}
}

func TestEntryPointOverride_ToStringSlice(t *testing.T) {
	testCases := map[string]struct {
		inEntryPointOverride EntryPointOverride

		wantedSlice []string
		wantedError error
	}{
		"Both fields are empty": {
			inEntryPointOverride: EntryPointOverride{
				String:      nil,
				StringSlice: nil,
			},
			wantedSlice: nil,
		},
		"Given a string": {
			inEntryPointOverride: EntryPointOverride{
				String:      aws.String(`read "some command"`),
				StringSlice: nil,
			},
			wantedSlice: []string{"read", "some command"},
		},
		"Given a string slice": {
			inEntryPointOverride: EntryPointOverride{
				String:      nil,
				StringSlice: []string{"/bin/sh", "-c"},
			},
			wantedSlice: []string{"/bin/sh", "-c"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			out, err := tc.inEntryPointOverride.ToStringSlice()
			require.NoError(t, err)
			require.Equal(t, tc.wantedSlice, out)
		})
	}
}

func TestCommandOverride_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		inContent []byte

		wantedStruct CommandOverride
		wantedError  error
	}{
		"Entrypoint specified in string": {
			inContent: []byte(`command: echo hello`),
			wantedStruct: CommandOverride{
				String:      aws.String("echo hello"),
				StringSlice: nil,
			},
		},
		"Entrypoint specified in slice of strings": {
			inContent: []byte(`command: ["--version"]`),
			wantedStruct: CommandOverride{
				String:      nil,
				StringSlice: []string{"--version"},
			},
		},
		"Error if unmarshalable": {
			inContent: []byte(`command: {-c}`),
			wantedStruct: CommandOverride{
				String:      nil,
				StringSlice: nil,
			},
			wantedError: errUnmarshalCommand,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			e := ImageOverride{
				Command: &CommandOverride{
					String: aws.String("wrong"),
				},
			}

			err := yaml.Unmarshal(tc.inContent, &e)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				// check memberwise dereferenced pointer equality
				require.Equal(t, tc.wantedStruct.StringSlice, e.Command.StringSlice)
				require.Equal(t, tc.wantedStruct.String, e.Command.String)
			}
		})
	}
}

func TestCommandOverride_ToStringSlice(t *testing.T) {
	testCases := map[string]struct {
		inCommandOverrides CommandOverride

		wantedSlice []string
	}{
		"Both fields are empty": {
			inCommandOverrides: CommandOverride{
				String:      nil,
				StringSlice: nil,
			},
			wantedSlice: nil,
		},
		"Given a string": {
			inCommandOverrides: CommandOverride{
				String:      aws.String(`-c read "some command"`),
				StringSlice: nil,
			},
			wantedSlice: []string{"-c", "read", "some command"},
		},
		"Given a string slice": {
			inCommandOverrides: CommandOverride{
				String:      nil,
				StringSlice: []string{"-c", "read", "some", "command"},
			},
			wantedSlice: []string{"-c", "read", "some", "command"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			out, err := tc.inCommandOverrides.ToStringSlice()
			require.NoError(t, err)
			require.Equal(t, tc.wantedSlice, out)
		})
	}
}

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

func TestPlatformArgsOrString_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		inContent []byte

		wantedStruct PlatformArgsOrString
		wantedError  error
	}{
		"returns error if both string and args specified": {
			inContent: []byte(`platform: linux/amd64
  osfamily: linux
  architecture: amd64`),

			wantedError: errors.New("yaml: line 2: mapping values are not allowed in this context"),
		},
		"returns error if platform string invalid": {
			inContent: []byte(`platform: linus/mad64`),

			wantedError: errors.New("validate platform: platform linus/mad64 is invalid; valid platforms are: linux/amd64, linux/x86_64, windows/amd64 and windows/x86_64"),
		},
		"returns error if only args.os specified": {
			inContent: []byte(`platform:
  osfamily: linux`),
			wantedError: errors.New("fields 'osfamily' and 'architecture' must either both be specified or both be empty"),
		},
		"returns error if only args.arch specified": {
			inContent: []byte(`platform:
  architecture: amd64`),
			wantedError: errors.New("fields 'osfamily' and 'architecture' must either both be specified or both be empty"),
		},
		"returns error if args.os invalid": {
			inContent: []byte(`platform:
  osfamily: OSFamilia
  architecture: amd64`),
			wantedError: errors.New("platform pair ('OSFamilia', 'amd64') is invalid: fields ('osfamily', 'architecture') must be one of" +
				" ('linux', 'x86_64'), ('linux', 'amd64')," +
				" ('windows_server_2019_core', 'x86_64'), ('windows_server_2019_core', 'amd64')," +
				" ('windows_server_2019_full', 'x86_64'), ('windows_server_2019_full', 'amd64')"),
		},
		"returns error if args.arch invalid": {
			inContent: []byte(`platform:
  osfamily: linux
  architecture: abc123`),
			wantedError: errors.New("platform pair ('linux', 'abc123') is invalid: fields ('osfamily', 'architecture') must be one of" +
				" ('linux', 'x86_64'), ('linux', 'amd64')," +
				" ('windows_server_2019_core', 'x86_64'), ('windows_server_2019_core', 'amd64')," +
				" ('windows_server_2019_full', 'x86_64'), ('windows_server_2019_full', 'amd64')"),
		},
		"platform string": {
			inContent: []byte(`platform: linux/amd64`),

			wantedStruct: PlatformArgsOrString{
				PlatformString: aws.String("linux/amd64"),
			},
		},
		"both os/arch specified with valid values": {
			inContent: []byte(`platform:
  osfamily: linux
  architecture: amd64`),
			wantedStruct: PlatformArgsOrString{
				PlatformString: nil,
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("linux"),
					Arch:     aws.String("amd64"),
				},
			},
		},
		"error if unmarshalable": {
			inContent: []byte(`platform:
  ohess: linus
  archie: leg64`),
			wantedError: errUnmarshalPlatformOpts,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			p := TaskConfig{
				Platform: &PlatformArgsOrString{
					PlatformString: nil,
					PlatformArgs: PlatformArgs{
						OSFamily: nil,
						Arch:     nil,
					},
				},
			}
			err := yaml.Unmarshal(tc.inContent, &p)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedStruct.PlatformString, p.Platform.PlatformString)
				require.Equal(t, tc.wantedStruct.PlatformArgs.OSFamily, p.Platform.PlatformArgs.OSFamily)
				require.Equal(t, tc.wantedStruct.PlatformArgs.Arch, p.Platform.PlatformArgs.Arch)
			}
		})
	}
}

func TestPlatformArgsOrString_OS(t *testing.T) {
	testCases := map[string]struct {
		in     *PlatformArgsOrString
		wanted string
	}{
		"should return os when platform is of string format 'os/arch'": {
			in: &PlatformArgsOrString{
				PlatformString: aws.String("linux/amd64"),
			},
			wanted: "linux",
		},
		"should return OS when platform is a map": {
			in: &PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("windows_server_core"),
					Arch:     aws.String("x86_64"),
				},
			},
			wanted: "windows_server_core",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.OS())
		})
	}
}

func TestPlatformArgsOrString_Arch(t *testing.T) {
	testCases := map[string]struct {
		in     *PlatformArgsOrString
		wanted string
	}{
		"should return arch when platform is of string format 'os/arch'": {
			in: &PlatformArgsOrString{
				PlatformString: aws.String("linux/arm"),
			},
			wanted: "arm",
		},
		"should return arch when platform is a map": {
			in: &PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("windows_server_core"),
					Arch:     aws.String("x86_64"),
				},
			},
			wanted: "x86_64",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.in.Arch())
		})
	}
}

func TestRedirectPlatform(t *testing.T) {
	testCases := map[string]struct {
		inOS           string
		inArch         string
		inWorkloadType string

		wantedPlatform string
		wantedError    error
	}{
		"returns nil if default platform": {
			inOS:           "linux",
			inArch:         "amd64",
			inWorkloadType: LoadBalancedWebServiceType,

			wantedPlatform: "",
			wantedError:    nil,
		},
		"returns error if App Runner + Windows": {
			inOS:           "windows",
			inArch:         "amd64",
			inWorkloadType: RequestDrivenWebServiceType,

			wantedPlatform: "",
			wantedError:    errors.New("Windows is not supported for App Runner services"),
		},
		"targets amd64 if ARM architecture passed in": {
			inOS:   "linux",
			inArch: "arm64",

			wantedPlatform: "linux/amd64",
			wantedError:    nil,
		},
		"returns non-default os as is": {
			inOS:   "windows",
			inArch: "amd64",

			wantedPlatform: "windows/amd64",
			wantedError:    nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			platform, err := RedirectPlatform(tc.inOS, tc.inArch, tc.inWorkloadType)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedPlatform, platform)
			}
		})
	}
}

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

		wantedConfig *NetworkConfig
		wantedErr    error
	}{
		"defaults to public placement if vpc is empty": {
			data: `
network:
  vpc:
`,
			wantedConfig: &NetworkConfig{
				VPC: &vpcConfig{
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
			wantedConfig: &NetworkConfig{
				VPC: &vpcConfig{
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
				Network *NetworkConfig `yaml:"network"`
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

func TestDependency_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		inContent []byte

		wantedStruct Image
		wantedError  error
	}{
		"Unspecified optional dependencies don't appear in image": {
			inContent:    []byte(``),
			wantedStruct: Image{},
		},
		"Empty dependencies don't appear in image": {
			inContent:    []byte(`depends_on:`),
			wantedStruct: Image{},
		},
		"Error when unmarshallable": {
			inContent: []byte(`depends_on:
    frontend: coolwebsite
  sidecar2: wheels`),
			wantedStruct: Image{
				DependsOn: map[string]string{
					"frontend": "coolwebsite",
					"sidecar2": "wheels",
				},
			},
			wantedError: errors.New("yaml: line 2: did not find expected key"),
		},
		"Valid yaml specified": {
			inContent: []byte(`depends_on:
  frontend: coolwebsite
  sidecar2: wheels`),
			wantedStruct: Image{
				DependsOn: map[string]string{
					"frontend": "coolwebsite",
					"sidecar2": "wheels",
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			i := Image{}

			err := yaml.Unmarshal(tc.inContent, &i)

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedStruct.DependsOn, i.DependsOn)
			}
		})
	}
}

func TestUnmarshalPublish(t *testing.T) {
	testCases := map[string]struct {
		inContent     string
		wantedPublish PublishConfig
		wantedErr     error
	}{
		"Valid publish yaml": {
			inContent: `topics:
  - name: tests
    allowed_workers:
      - hello
`,
			wantedPublish: PublishConfig{
				Topics: []Topic{
					{
						Name:           aws.String("tests"),
						AllowedWorkers: []string{"hello"},
					},
				},
			},
		},
		"Empty workers don't appear in topic": {
			inContent: `topics:
  - name: tests
`,
			wantedPublish: PublishConfig{
				Topics: []Topic{
					{
						Name: aws.String("tests"),
					},
				},
			},
		},
		"Error when unmarshalable": {
			inContent: `topics:
   - name: tests
    allowed_workers:
      - hello
  - name: orders
`,
			wantedErr: errors.New("yaml: line 1: did not find expected '-' indicator"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			p := PublishConfig{}

			err := yaml.Unmarshal([]byte(tc.inContent), &p)

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedPublish, p)
			}
		})
	}
}

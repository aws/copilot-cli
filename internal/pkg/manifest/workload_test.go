// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestImage_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		inContent []byte

		wantedError error
	}{
		"error if both build and location are set": {
			inContent: []byte(`build: mockBuild
location: mockLocation`),
			wantedError: fmt.Errorf(`must specify one of "build" and "location"`),
		},
		"success": {
			inContent: []byte(`location: mockLocation`),
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
				require.Equal(t, "mockLocation", aws.StringValue(i.Location))
			}
		})
	}
}

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
				EntryPoint: EntryPointOverride{
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
				Command: CommandOverride{
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
				ImageLocationOrBuild: ImageLocationOrBuild{
					Build: BuildArgsOrString{
						BuildString: aws.String("./default"),
					},
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
		"error if unmarshalable": {
			inContent: []byte(`platform:
  ohess: linus
  archie: leg64`),
			wantedError: errUnmarshalPlatformOpts,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			p := TaskConfig{}
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

func TestServiceConnectBoolOrArgs_ServiceConnectEnabled(t *testing.T) {
	testCases := map[string]struct {
		mft *ServiceConnectBoolOrArgs

		wanted bool
	}{
		"disabled by default": {
			mft:    &ServiceConnectBoolOrArgs{},
			wanted: false,
		},
		"set by bool": {
			mft: &ServiceConnectBoolOrArgs{
				EnableServiceConnect: aws.Bool(true),
			},
			wanted: true,
		},
		"set by args": {
			mft: &ServiceConnectBoolOrArgs{
				ServiceConnectArgs: ServiceConnectArgs{
					Alias: aws.String("api"),
				},
			},
			wanted: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			enabled := tc.mft.Enabled()

			// THEN
			require.Equal(t, tc.wanted, enabled)
		})
	}
}

func TestServiceConnect_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		inContent []byte

		wantedStruct ServiceConnectBoolOrArgs
		wantedError  error
	}{
		"returns error if both bool and args specified": {
			inContent: []byte(`connect: true
  alias: api`),

			wantedError: errors.New("yaml: line 2: mapping values are not allowed in this context"),
		},
		"error if unmarshalable": {
			inContent: []byte(`connect:
  ohess: linus
  archie: leg64`),
			wantedError: errUnmarshalServiceConnectOpts,
		},
		"success": {
			inContent: []byte(`connect:
  alias: api`),
			wantedStruct: ServiceConnectBoolOrArgs{
				ServiceConnectArgs: ServiceConnectArgs{
					Alias: aws.String("api"),
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			v := NetworkConfig{}
			err := yaml.Unmarshal(tc.inContent, &v)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedStruct, v.Connect)
			}
		})
	}
}

func TestPlacementArgOrString_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		inContent []byte

		wantedStruct PlacementArgOrString
		wantedError  error
	}{
		"returns error if both string and args specified": {
			inContent: []byte(`placement: private
  subnets: ["id1", "id2"]`),

			wantedError: errors.New("yaml: line 2: mapping values are not allowed in this context"),
		},
		"error if unmarshalable": {
			inContent: []byte(`placement:
  ohess: linus
  archie: leg64`),
			wantedError: errUnmarshalPlacementOpts,
		},
		"success": {
			inContent: []byte(`placement:
  subnets: ["id1", "id2"]`),
			wantedStruct: PlacementArgOrString{
				PlacementArgs: PlacementArgs{
					Subnets: SubnetListOrArgs{
						IDs: []string{"id1", "id2"},
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			v := vpcConfig{}
			err := yaml.Unmarshal(tc.inContent, &v)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedStruct.PlacementString, v.Placement.PlacementString)
				require.Equal(t, tc.wantedStruct.PlacementArgs.Subnets, v.Placement.PlacementArgs.Subnets)
			}
		})
	}
}

func TestSubnetListOrArgs_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		inContent []byte

		wantedStruct SubnetListOrArgs
		wantedError  error
	}{
		"returns error if both string slice and args specified": {
			inContent: []byte(`subnets: ["id1", "id2"]
  from_tags:
    - foo: bar`),

			wantedError: errors.New("yaml: line 1: did not find expected key"),
		},
		"error if unmarshalable": {
			inContent: []byte(`subnets:
  ohess: linus
  archie: leg64`),
			wantedError: errUnmarshalSubnetsOpts,
		},
		"success with string slice": {
			inContent: []byte(`subnets: ["id1", "id2"]`),
			wantedStruct: SubnetListOrArgs{
				IDs: []string{"id1", "id2"},
			},
		},
		"success with args": {
			inContent: []byte(`subnets:
  from_tags:
    foo: bar`),
			wantedStruct: SubnetListOrArgs{
				SubnetArgs: SubnetArgs{
					FromTags: map[string]StringSliceOrString{
						"foo": {String: aws.String("bar")},
					},
				},
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			v := PlacementArgs{}
			err := yaml.Unmarshal(tc.inContent, &v)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedStruct.IDs, v.Subnets.IDs)
				require.Equal(t, tc.wantedStruct.FromTags, v.Subnets.FromTags)
			}
		})
	}
}

func TestPlatformArgsOrString_OS(t *testing.T) {
	linux := PlatformString("linux/amd64")
	testCases := map[string]struct {
		in     *PlatformArgsOrString
		wanted string
	}{
		"should return os when platform is of string format 'os/arch'": {
			in: &PlatformArgsOrString{
				PlatformString: &linux,
			},
			wanted: "linux",
		},
		"should return OS when platform is a map 2019 core": {
			in: &PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("windows_server_2019_core"),
					Arch:     aws.String("x86_64"),
				},
			},
			wanted: "windows_server_2019_core",
		},
		"should return lowercase OS 2019 core": {
			in: &PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("wINdows_sERver_2019_cORe"),
					Arch:     aws.String("x86_64"),
				},
			},
			wanted: "windows_server_2019_core",
		},
		"should return OS when platform is a map 2019 full": {
			in: &PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("windows_server_2019_full"),
					Arch:     aws.String("x86_64"),
				},
			},
			wanted: "windows_server_2019_full",
		},
		"should return lowercase OS 2019 full": {
			in: &PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("wINdows_sERver_2019_fUll"),
					Arch:     aws.String("x86_64"),
				},
			},
			wanted: "windows_server_2019_full",
		},
		"should return OS when platform is a map 2022 core": {
			in: &PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("windows_server_2022_core"),
					Arch:     aws.String("x86_64"),
				},
			},
			wanted: "windows_server_2022_core",
		},
		"should return lowercase OS 2022 core": {
			in: &PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("wINdows_sERver_2022_cORe"),
					Arch:     aws.String("x86_64"),
				},
			},
			wanted: "windows_server_2022_core",
		},
		"should return OS when platform is a map 2022 full": {
			in: &PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("windows_server_2022_full"),
					Arch:     aws.String("x86_64"),
				},
			},
			wanted: "windows_server_2022_full",
		},
		"should return lowercase OS 2022 full": {
			in: &PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("wINdows_sERver_2022_fUll"),
					Arch:     aws.String("x86_64"),
				},
			},
			wanted: "windows_server_2022_full",
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
				PlatformString: (*PlatformString)(aws.String("windows/arm")),
			},
			wanted: "arm",
		},
		"should return arch when platform is a map 2019 core": {
			in: &PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("windows_server_2019_core"),
					Arch:     aws.String("x86_64"),
				},
			},
			wanted: "x86_64",
		},
		"should return arch when platform is a map 2019 full": {
			in: &PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("windows_server_2019_full"),
					Arch:     aws.String("x86_64"),
				},
			},
			wanted: "x86_64",
		},
		"should return arch when platform is a map 2022 core": {
			in: &PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("windows_server_2022_core"),
					Arch:     aws.String("x86_64"),
				},
			},
			wanted: "x86_64",
		},
		"should return arch when platform is a map 2022 full": {
			in: &PlatformArgsOrString{
				PlatformArgs: PlatformArgs{
					OSFamily: aws.String("windows_server_2022_full"),
					Arch:     aws.String("x86_64"),
				},
			},
			wanted: "x86_64",
		},
		"should return lowercase arch": {
			in: &PlatformArgsOrString{
				PlatformString: (*PlatformString)(aws.String("windows/aMd64")),
			},
			wanted: "amd64",
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
			inWorkloadType: manifestinfo.LoadBalancedWebServiceType,

			wantedPlatform: "",
			wantedError:    nil,
		},
		"returns error if App Runner + Windows": {
			inOS:           "windows",
			inArch:         "amd64",
			inWorkloadType: manifestinfo.RequestDrivenWebServiceType,

			wantedPlatform: "",
			wantedError:    errors.New("Windows is not supported for App Runner services"),
		},
		"targets x86_64 if ARM architecture passed in": {
			inOS:   "linux",
			inArch: "arm64",

			wantedPlatform: "linux/x86_64",
			wantedError:    nil,
		},
		"returns non-default os as is": {
			inOS:   "windows",
			inArch: "amd64",

			wantedPlatform: "windows/x86_64",
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
				ImageLocationOrBuild: ImageLocationOrBuild{
					Build: tc.inBuild,
				},
			}
			got := s.BuildConfig(mockWsRoot)

			require.Equal(t, tc.wantedBuild, *got)
		})
	}
}

func TestNetworkConfig_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     NetworkConfig
		wanted bool
	}{
		"empty network config": {
			in:     NetworkConfig{},
			wanted: true,
		},
		"non empty network config": {
			in: NetworkConfig{
				VPC: vpcConfig{
					SecurityGroups: SecurityGroupsIDsOrConfig{
						IDs: []StringOrFromCFN{
							{
								Plain: aws.String("group"),
							},
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

func TestSecurityGroupsConfig_GetIDs(t *testing.T) {
	testCases := map[string]struct {
		in     SecurityGroupsIDsOrConfig
		wanted []StringOrFromCFN
	}{
		"nil returned when no security groups are specified": {
			in:     SecurityGroupsIDsOrConfig{},
			wanted: nil,
		},
		"security groups in map are returned": {
			in: SecurityGroupsIDsOrConfig{
				AdvancedConfig: SecurityGroupsConfig{
					SecurityGroups: []StringOrFromCFN{
						{
							Plain: aws.String("group"),
						},
						{
							Plain: aws.String("group1"),
						},
						{
							FromCFN: fromCFN{
								Name: aws.String("sg-001"),
							},
						},
					},
				},
			},
			wanted: []StringOrFromCFN{
				{
					Plain: aws.String("group"),
				},
				{
					Plain: aws.String("group1"),
				},
				{
					FromCFN: fromCFN{
						Name: aws.String("sg-001"),
					},
				},
			},
		},
		"nil returned when security groups in map are empty": {
			in: SecurityGroupsIDsOrConfig{
				AdvancedConfig: SecurityGroupsConfig{
					SecurityGroups: []StringOrFromCFN{},
				},
			},
			wanted: nil,
		},
		"security groups in array are returned": {
			in: SecurityGroupsIDsOrConfig{
				IDs: []StringOrFromCFN{
					{
						Plain: aws.String("123"),
					},
					{
						Plain: aws.String("45"),
					},
					{
						FromCFN: fromCFN{
							Name: aws.String("sg-001"),
						},
					},
				},
			},
			wanted: []StringOrFromCFN{
				{Plain: aws.String("123")},
				{Plain: aws.String("45")},
				{FromCFN: fromCFN{
					Name: aws.String("sg-001"),
				}},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			sgs := tc.in.GetIDs()

			// THEN
			require.Equal(t, tc.wanted, sgs)
		})
	}
}

func TestSecurityGroupsConfig_IsDefaultSecurityGroupDenied(t *testing.T) {
	testCases := map[string]struct {
		in     SecurityGroupsIDsOrConfig
		wanted bool
	}{
		"default security group is applied when no vpc security config is present": {
			wanted: false,
		},
		"default security group is applied when deny_default is not specified in SG config": {
			in: SecurityGroupsIDsOrConfig{
				AdvancedConfig: SecurityGroupsConfig{
					SecurityGroups: []StringOrFromCFN{
						{
							Plain: aws.String("1"),
						},
					},
				},
			},
			wanted: false,
		},
		"default security group is applied when deny_default is false in SG config": {
			in: SecurityGroupsIDsOrConfig{
				AdvancedConfig: SecurityGroupsConfig{
					SecurityGroups: []StringOrFromCFN{
						{
							Plain: aws.String("1"),
						},
					},
					DenyDefault: aws.Bool(false),
				},
			},
			wanted: false,
		},
		"default security group is applied when security group array is specified": {
			in: SecurityGroupsIDsOrConfig{
				IDs: []StringOrFromCFN{
					{
						Plain: aws.String("1"),
					},
				},
			},
			wanted: false,
		},
		"default security group is not applied when default_deny is true": {
			in: SecurityGroupsIDsOrConfig{
				AdvancedConfig: SecurityGroupsConfig{
					DenyDefault: aws.Bool(true),
				},
			},
			wanted: true,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			actualIsDefaultSGDenied := tc.in.IsDefaultSecurityGroupDenied()

			// THEN
			require.Equal(t, tc.wanted, actualIsDefaultSGDenied)
		})
	}
}

func TestNetworkConfig_UnmarshalYAML(t *testing.T) {
	var (
		trueValue = true
	)
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
				VPC: vpcConfig{},
			},
		},
		"unmarshals successfully for public placement with security groups": {
			data: `
network:
  vpc:
    placement: 'public'
    security_groups:
      - 'sg-1234'
      - 'sg-4567'
      - from_cfn: 'dbsg-001'
`,
			wantedConfig: &NetworkConfig{
				VPC: vpcConfig{
					Placement: PlacementArgOrString{
						PlacementString: placementStringP(PublicSubnetPlacement),
					},
					SecurityGroups: SecurityGroupsIDsOrConfig{
						IDs: []StringOrFromCFN{
							{
								Plain: aws.String("sg-1234"),
							},
							{
								Plain: aws.String("sg-4567"),
							},
							{
								FromCFN: fromCFN{
									Name: aws.String("dbsg-001"),
								},
							},
						},
						AdvancedConfig: SecurityGroupsConfig{},
					},
				},
			},
		},
		"unmarshal is successful for security groups specified in config": {
			data: `
network:
  vpc:
    security_groups:
      groups: 
        - 'sg-1234'
        - 'sg-4567'
        - from_cfn: 'dbsg-001'
      deny_default: true
`,
			wantedConfig: &NetworkConfig{
				VPC: vpcConfig{
					Placement: PlacementArgOrString{},
					SecurityGroups: SecurityGroupsIDsOrConfig{
						IDs: nil,
						AdvancedConfig: SecurityGroupsConfig{
							SecurityGroups: []StringOrFromCFN{
								{
									Plain: aws.String("sg-1234"),
								},
								{
									Plain: aws.String("sg-4567"),
								},
								{
									FromCFN: fromCFN{
										Name: aws.String("dbsg-001"),
									},
								},
							},
							DenyDefault: &trueValue,
						},
					},
				},
			},
		},
		"unmarshal is successful for security groups specified in config without default deny": {
			data: `
network:
  vpc:
    security_groups:
      groups: ['sg-1234', 'sg-4567']
`,
			wantedConfig: &NetworkConfig{
				VPC: vpcConfig{
					Placement: PlacementArgOrString{},
					SecurityGroups: SecurityGroupsIDsOrConfig{
						IDs: nil,
						AdvancedConfig: SecurityGroupsConfig{
							SecurityGroups: []StringOrFromCFN{
								{
									Plain: aws.String("sg-1234"),
								},
								{
									Plain: aws.String("sg-4567"),
								},
							},
						},
					},
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
			inContent: `
topics:
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
		"Valid publish yaml with fifo topic enabled": {
			inContent: `
topics:
  - name: tests
    fifo: true
`,
			wantedPublish: PublishConfig{
				Topics: []Topic{
					{
						Name: aws.String("tests"),
						FIFO: FIFOTopicAdvanceConfigOrBool{
							Enable: aws.Bool(true),
						},
					},
				},
			},
		},
		"Valid publish yaml with advanced fifo topic": {
			inContent: `
topics:
  - name: tests
    fifo:
      content_based_deduplication: true
`,
			wantedPublish: PublishConfig{
				Topics: []Topic{
					{
						Name: aws.String("tests"),
						FIFO: FIFOTopicAdvanceConfigOrBool{
							Advanced: FIFOTopicAdvanceConfig{
								ContentBasedDeduplication: aws.Bool(true),
							},
						},
					},
				},
			},
		},
		"Invalid publish yaml with advanced fifo topic": {
			inContent: `
topics:
  - name: tests
    fifo: apple
`,
			wantedErr: errors.New(`unable to unmarshal "fifo" field into boolean or compose-style map`),
		},

		"Error when unmarshalable": {
			inContent: `
topics: abc
`,
			wantedErr: errors.New("yaml: unmarshal errors:\n  line 2: cannot unmarshal !!str `abc` into []manifest.Topic"),
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

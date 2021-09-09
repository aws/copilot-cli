// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/imdario/mergo"
	"github.com/stretchr/testify/require"
)

func TestBasicTransformer_Transformer(t *testing.T) {
	type testBasicTransformerStruct struct {
		PBool   *bool
		PInt    *int
		PInt64  *int64
		PUint16 *uint16
		PUint32 *uint32
		PString *string
		PSlice  *[]string
		Slice   []string
	}

	testCases := map[string]struct {
		original func(s *testBasicTransformerStruct)
		override func(s *testBasicTransformerStruct)
		wanted   func(s *testBasicTransformerStruct)
	}{
		"overridden": {
			original: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(false)
				s.PInt = aws.Int(24)
				s.PInt64 = aws.Int64(24)
				s.PUint16 = aws.Uint16(24)
				s.PUint32 = aws.Uint32(24)
				s.PString = aws.String("horse")
				s.Slice = []string{"horses", "run"}

				mockSlice := []string{"horses", "run"}
				s.PSlice = &mockSlice
			},
			override: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(true)
				s.PInt = aws.Int(42)
				s.PInt64 = aws.Int64(42)
				s.PUint16 = aws.Uint16(42)
				s.PUint32 = aws.Uint32(42)
				s.PString = aws.String("pony")
				s.Slice = []string{"pony", "run"}

				mockSlice := []string{"pony", "run"}
				s.PSlice = &mockSlice
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(true)
				s.PInt = aws.Int(42)
				s.PInt64 = aws.Int64(42)
				s.PUint16 = aws.Uint16(42)
				s.PUint32 = aws.Uint32(42)
				s.PString = aws.String("pony")
				s.Slice = []string{"pony", "run"}

				mockSlice := []string{"pony", "run"}
				s.PSlice = &mockSlice
			},
		},
		"explicitly overridden by zero value": {
			original: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(true)
				s.PInt = aws.Int(24)
				s.PInt64 = aws.Int64(24)
				s.PUint16 = aws.Uint16(24)
				s.PUint32 = aws.Uint32(24)
				s.PString = aws.String("horse")
				s.Slice = []string{"horses", "run"}

				mockSlice := []string{"horses", "run"}
				s.PSlice = &mockSlice
			},
			override: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(false)
				s.PInt = aws.Int(0)
				s.PInt64 = aws.Int64(0)
				s.PUint16 = aws.Uint16(0)
				s.PUint32 = aws.Uint32(0)
				s.PString = aws.String("")
				s.Slice = []string{}

				var mockSlice []string
				s.PSlice = &mockSlice
			},
			wanted: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(false)
				s.PInt = aws.Int(0)
				s.PInt64 = aws.Int64(0)
				s.PUint16 = aws.Uint16(0)
				s.PUint32 = aws.Uint32(0)
				s.PString = aws.String("")
				s.Slice = []string{}

				var mockSlice []string
				s.PSlice = &mockSlice
			},
		},
		"not overridden by nil": {
			original: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(true)
				s.PInt = aws.Int(24)
				s.PInt64 = aws.Int64(24)
				s.PUint16 = aws.Uint16(24)
				s.PUint32 = aws.Uint32(24)
				s.PString = aws.String("horse")
				s.Slice = []string{"horses", "run"}

				mockSlice := []string{"horses", "run"}
				s.PSlice = &mockSlice
			},
			override: func(s *testBasicTransformerStruct) {},
			wanted: func(s *testBasicTransformerStruct) {
				s.PBool = aws.Bool(true)
				s.PInt = aws.Int(24)
				s.PInt64 = aws.Int64(24)
				s.PUint16 = aws.Uint16(24)
				s.PUint32 = aws.Uint32(24)
				s.PString = aws.String("horse")
				s.Slice = []string{"horses", "run"}

				mockSlice := []string{"horses", "run"}
				s.PSlice = &mockSlice
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted testBasicTransformerStruct

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			err := mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(basicTransformer{}))

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestImageTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(i *Image)
		override func(i *Image)
		wanted   func(i *Image)
	}{
		"build set to empty if location is not nil": {
			original: func(i *Image) {
				i.Build = BuildArgsOrString{
					BuildString: aws.String("mockBuild"),
				}
			},
			override: func(i *Image) {
				i.Location = aws.String("mockLocation")
			},
			wanted: func(i *Image) {
				i.Location = aws.String("mockLocation")
				i.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs:   DockerBuildArgs{},
				}
			},
		},
		"location set to empty if build is not nil": {
			original: func(i *Image) {
				i.Location = aws.String("mockLocation")
			},
			override: func(i *Image) {
				i.Build = BuildArgsOrString{
					BuildArgs: DockerBuildArgs{
						Dockerfile: aws.String("mockDockerfile"),
					},
				}
			},
			wanted: func(i *Image) {
				i.Location = nil
				i.Build = BuildArgsOrString{
					BuildString: nil,
					BuildArgs: DockerBuildArgs{
						Dockerfile: aws.String("mockDockerfile"),
					},
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted Image

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use imageTransformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(imageTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestBuildArgsOrStringTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(b *BuildArgsOrString)
		override func(b *BuildArgsOrString)
		wanted   func(b *BuildArgsOrString)
	}{
		"build string set to empty if build args is not nil": {
			original: func(b *BuildArgsOrString) {
				b.BuildString = aws.String("mockBuild")
			},
			override: func(b *BuildArgsOrString) {
				b.BuildArgs = DockerBuildArgs{
					Context:    aws.String("mockContext"),
					Dockerfile: aws.String("mockDockerfile"),
				}
			},
			wanted: func(b *BuildArgsOrString) {
				b.BuildString = nil
				b.BuildArgs = DockerBuildArgs{
					Context:    aws.String("mockContext"),
					Dockerfile: aws.String("mockDockerfile"),
				}
			},
		},
		"build args set to empty if build string is not nil": {
			original: func(b *BuildArgsOrString) {
				b.BuildArgs = DockerBuildArgs{
					Context:    aws.String("mockContext"),
					Dockerfile: aws.String("mockDockerfile"),
				}
			},
			override: func(b *BuildArgsOrString) {
				b.BuildString = aws.String("mockBuild")
			},
			wanted: func(b *BuildArgsOrString) {
				b.BuildString = aws.String("mockBuild")
				b.BuildArgs = DockerBuildArgs{}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted BuildArgsOrString

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use imageTransformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(buildArgsOrStringTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestStringSliceOrStringTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(s *stringSliceOrString)
		override func(s *stringSliceOrString)
		wanted   func(s *stringSliceOrString)
	}{
		"string set to empty if string slice is not nil": {
			original: func(s *stringSliceOrString) {
				s.String = aws.String("mockString")
			},
			override: func(s *stringSliceOrString) {
				s.StringSlice = []string{"mock", "string", "slice"}
			},
			wanted: func(s *stringSliceOrString) {
				s.StringSlice = []string{"mock", "string", "slice"}
			},
		},
		"string slice set to empty if string is not nil": {
			original: func(s *stringSliceOrString) {
				s.StringSlice = []string{"mock", "string", "slice"}
			},
			override: func(s *stringSliceOrString) {
				s.String = aws.String("mockString")
			},
			wanted: func(s *stringSliceOrString) {
				s.String = aws.String("mockString")
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted stringSliceOrString

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use imageTransformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(stringSliceOrStringTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestPlatformArgsOrStringTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(p *PlatformArgsOrString)
		override func(p *PlatformArgsOrString)
		wanted   func(p *PlatformArgsOrString)
	}{
		"string set to empty if args is not nil": {
			original: func(p *PlatformArgsOrString) {
				p.PlatformString = aws.String("mockString")
			},
			override: func(p *PlatformArgsOrString) {
				p.PlatformArgs = PlatformArgs{
					OSFamily: aws.String("mock"),
					Arch:     aws.String("platformTest"),
				}
			},
			wanted: func(p *PlatformArgsOrString) {
				p.PlatformArgs = PlatformArgs{
					OSFamily: aws.String("mock"),
					Arch:     aws.String("platformTest"),
				}
			},
		},
		"args set to empty if string is not nil": {
			original: func(p *PlatformArgsOrString) {
				p.PlatformArgs = PlatformArgs{
					OSFamily: aws.String("mock"),
					Arch:     aws.String("platformTest"),
				}
			},
			override: func(p *PlatformArgsOrString) {
				p.PlatformString = aws.String("mockString")
			},
			wanted: func(p *PlatformArgsOrString) {
				p.PlatformString = aws.String("mockString")
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted PlatformArgsOrString

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use imageTransformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(platformArgsOrStringTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestHealthCheckArgsOrStringTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(h *HealthCheckArgsOrString)
		override func(h *HealthCheckArgsOrString)
		wanted   func(h *HealthCheckArgsOrString)
	}{
		"string set to empty if args is not nil": {
			original: func(h *HealthCheckArgsOrString) {
				h.HealthCheckPath = aws.String("mockPath")
			},
			override: func(h *HealthCheckArgsOrString) {
				h.HealthCheckArgs = HTTPHealthCheckArgs{
					Path:         aws.String("mockPathArgs"),
					SuccessCodes: aws.String("200"),
				}
			},
			wanted: func(h *HealthCheckArgsOrString) {
				h.HealthCheckArgs = HTTPHealthCheckArgs{
					Path:         aws.String("mockPathArgs"),
					SuccessCodes: aws.String("200"),
				}
			},
		},
		"args set to empty if string is not nil": {
			original: func(h *HealthCheckArgsOrString) {
				h.HealthCheckArgs = HTTPHealthCheckArgs{
					Path:         aws.String("mockPathArgs"),
					SuccessCodes: aws.String("200"),
				}
			},
			override: func(h *HealthCheckArgsOrString) {
				h.HealthCheckPath = aws.String("mockPath")
			},
			wanted: func(h *HealthCheckArgsOrString) {
				h.HealthCheckPath = aws.String("mockPath")
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted HealthCheckArgsOrString

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use imageTransformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(healthCheckArgsOrStringTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestCountTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(c *Count)
		override func(c *Count)
		wanted   func(c *Count)
	}{
		"value set to empty if advanced count is not nil": {
			original: func(c *Count) {
				c.Value = aws.Int(24)
			},
			override: func(c *Count) {
				c.AdvancedCount = AdvancedCount{
					Spot: aws.Int(42),
				}
			},
			wanted: func(c *Count) {
				c.AdvancedCount = AdvancedCount{
					Spot: aws.Int(42),
				}
			},
		},
		"advanced count set to empty if value is not nil": {
			original: func(c *Count) {
				c.AdvancedCount = AdvancedCount{
					Spot: aws.Int(42),
				}
			},
			override: func(c *Count) {
				c.Value = aws.Int(24)
			},
			wanted: func(c *Count) {
				c.Value = aws.Int(24)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted Count

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use imageTransformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(countTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestAdvancedCountTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(a *AdvancedCount)
		override func(a *AdvancedCount)
		wanted   func(a *AdvancedCount)
	}{
		"spot set to empty if auto scaling is not empty": {
			original: func(a *AdvancedCount) {
				a.Spot = aws.Int(24)
			},
			override: func(a *AdvancedCount) {
				a.Range = &Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				}
				a.CPU = aws.Int(1024)
				a.Requests = aws.Int(42)
			},
			wanted: func(a *AdvancedCount) {
				a.Range = &Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				}
				a.CPU = aws.Int(1024)
				a.Requests = aws.Int(42)
			},
		},
		"auto scaling set to empty if spot is not nil": {
			original: func(a *AdvancedCount) {
				a.Range = &Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				}
				a.CPU = aws.Int(1024)
				a.Requests = aws.Int(42)
			},
			override: func(a *AdvancedCount) {
				a.Spot = aws.Int(24)
			},
			wanted: func(a *AdvancedCount) {
				a.Spot = aws.Int(24)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted AdvancedCount

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use imageTransformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(advancedCountTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestRangeTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(r *Range)
		override func(r *Range)
		wanted   func(r *Range)
	}{
		"value set to empty if range config is not nil": {
			original: func(r *Range) {
				r.Value = (*IntRangeBand)(aws.String("24-42"))
			},
			override: func(r *Range) {
				r.RangeConfig = RangeConfig{
					Min:      aws.Int(5),
					Max:      aws.Int(42),
					SpotFrom: aws.Int(13),
				}
			},
			wanted: func(r *Range) {
				r.RangeConfig = RangeConfig{
					Min:      aws.Int(5),
					Max:      aws.Int(42),
					SpotFrom: aws.Int(13),
				}
			},
		},
		"range config set to empty if value is not nil": {
			original: func(r *Range) {
				r.RangeConfig = RangeConfig{
					Min:      aws.Int(5),
					Max:      aws.Int(42),
					SpotFrom: aws.Int(13),
				}
			},
			override: func(r *Range) {
				r.Value = (*IntRangeBand)(aws.String("24-42"))
			},
			wanted: func(r *Range) {
				r.Value = (*IntRangeBand)(aws.String("24-42"))
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted Range

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use imageTransformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(rangeTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestEfsConfigOrBoolTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(e *EFSConfigOrBool)
		override func(e *EFSConfigOrBool)
		wanted   func(e *EFSConfigOrBool)
	}{
		"bool set to empty if config is not nil": {
			original: func(e *EFSConfigOrBool) {
				e.Enabled = aws.Bool(true)
			},
			override: func(e *EFSConfigOrBool) {
				e.Advanced = EFSVolumeConfiguration{
					UID: aws.Uint32(31415926),
					GID: aws.Uint32(53589793),
				}
			},
			wanted: func(e *EFSConfigOrBool) {
				e.Advanced = EFSVolumeConfiguration{
					UID: aws.Uint32(31415926),
					GID: aws.Uint32(53589793),
				}
			},
		},
		"config set to empty if bool is not nil": {
			original: func(e *EFSConfigOrBool) {
				e.Advanced = EFSVolumeConfiguration{
					UID: aws.Uint32(31415926),
					GID: aws.Uint32(53589793),
				}
			},
			override: func(e *EFSConfigOrBool) {
				e.Enabled = aws.Bool(true)
			},
			wanted: func(e *EFSConfigOrBool) {
				e.Enabled = aws.Bool(true)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted EFSConfigOrBool

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use imageTransformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(efsConfigOrBoolTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestEfsVolumeConfigurationTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(e *EFSVolumeConfiguration)
		override func(e *EFSVolumeConfiguration)
		wanted   func(e *EFSVolumeConfiguration)
	}{
		"UID config set to empty if BYO config is not empty": {
			original: func(e *EFSVolumeConfiguration) {
				e.UID = aws.Uint32(31415926)
				e.GID = aws.Uint32(53589793)
			},
			override: func(e *EFSVolumeConfiguration) {
				e.FileSystemID = aws.String("mockFileSystem")
				e.RootDirectory = aws.String("mockRootDir")
			},
			wanted: func(e *EFSVolumeConfiguration) {
				e.FileSystemID = aws.String("mockFileSystem")
				e.RootDirectory = aws.String("mockRootDir")
			},
		},
		"BYO config set to empty if UID config is not empty": {
			original: func(e *EFSVolumeConfiguration) {
				e.FileSystemID = aws.String("mockFileSystem")
				e.RootDirectory = aws.String("mockRootDir")
			},
			override: func(e *EFSVolumeConfiguration) {
				e.UID = aws.Uint32(31415926)
				e.GID = aws.Uint32(53589793)
			},
			wanted: func(e *EFSVolumeConfiguration) {
				e.UID = aws.Uint32(31415926)
				e.GID = aws.Uint32(53589793)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted EFSVolumeConfiguration

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use imageTransformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(efsVolumeConfigurationTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

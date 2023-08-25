// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"
	"time"

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

			// Use custom transformer.
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

			// Use custom transformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(buildArgsOrStringTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestAliasTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(*Alias)
		override func(*Alias)
		wanted   func(*Alias)
	}{
		"advanced alias set to empty if string slice is not nil": {
			original: func(a *Alias) {
				a.AdvancedAliases = []AdvancedAlias{
					{
						Alias: aws.String("mockAlias"),
					},
				}
			},
			override: func(a *Alias) {
				a.StringSliceOrString = StringSliceOrString{
					StringSlice: []string{"mock", "string", "slice"},
				}
			},
			wanted: func(a *Alias) {
				a.StringSliceOrString.StringSlice = []string{"mock", "string", "slice"}
			},
		},
		"StringSliceOrString set to empty if advanced alias is not nil": {
			original: func(a *Alias) {
				a.StringSliceOrString = StringSliceOrString{
					StringSlice: []string{"mock", "string", "slice"},
				}
			},
			override: func(a *Alias) {
				a.AdvancedAliases = []AdvancedAlias{
					{
						Alias: aws.String("mockAlias"),
					},
				}
			},
			wanted: func(a *Alias) {
				a.AdvancedAliases = []AdvancedAlias{
					{
						Alias: aws.String("mockAlias"),
					},
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted Alias

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use custom transformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(aliasTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestStringSliceOrStringTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(s *StringSliceOrString)
		override func(s *StringSliceOrString)
		wanted   func(s *StringSliceOrString)
	}{
		"string set to empty if string slice is not nil": {
			original: func(s *StringSliceOrString) {
				s.String = aws.String("mockString")
			},
			override: func(s *StringSliceOrString) {
				s.StringSlice = []string{"mock", "string", "slice"}
			},
			wanted: func(s *StringSliceOrString) {
				s.StringSlice = []string{"mock", "string", "slice"}
			},
		},
		"string slice set to empty if string is not nil": {
			original: func(s *StringSliceOrString) {
				s.StringSlice = []string{"mock", "string", "slice"}
			},
			override: func(s *StringSliceOrString) {
				s.String = aws.String("mockString")
			},
			wanted: func(s *StringSliceOrString) {
				s.String = aws.String("mockString")
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted StringSliceOrString

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use custom transformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(stringSliceOrStringTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestPlatformArgsOrStringTransformer_Transformer(t *testing.T) {
	mockPlatformStr := PlatformString("mockString")
	testCases := map[string]struct {
		original func(p *PlatformArgsOrString)
		override func(p *PlatformArgsOrString)
		wanted   func(p *PlatformArgsOrString)
	}{
		"string set to empty if args is not nil": {
			original: func(p *PlatformArgsOrString) {
				p.PlatformString = &mockPlatformStr
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
				p.PlatformString = &mockPlatformStr
			},
			wanted: func(p *PlatformArgsOrString) {
				p.PlatformString = &mockPlatformStr
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

			// Use custom transformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(platformArgsOrStringTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestPlacementArgsOrStringTransformer_Transformer(t *testing.T) {
	mockPlacementStr := PlacementString("mockString")
	testCases := map[string]struct {
		original func(p *PlacementArgOrString)
		override func(p *PlacementArgOrString)
		wanted   func(p *PlacementArgOrString)
	}{
		"string set to empty if args is not nil": {
			original: func(p *PlacementArgOrString) {
				p.PlacementString = &mockPlacementStr
			},
			override: func(p *PlacementArgOrString) {
				p.PlacementArgs = PlacementArgs{
					Subnets: SubnetListOrArgs{
						IDs: []string{"id1"},
					},
				}
			},
			wanted: func(p *PlacementArgOrString) {
				p.PlacementArgs = PlacementArgs{
					Subnets: SubnetListOrArgs{
						IDs: []string{"id1"},
					},
				}
			},
		},
		"args set to empty if string is not nil": {
			original: func(p *PlacementArgOrString) {
				p.PlacementArgs = PlacementArgs{
					Subnets: SubnetListOrArgs{
						IDs: []string{"id1"},
					},
				}
			},
			override: func(p *PlacementArgOrString) {
				p.PlacementString = &mockPlacementStr
			},
			wanted: func(p *PlacementArgOrString) {
				p.PlacementString = &mockPlacementStr
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted PlacementArgOrString

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use custom transformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(placementArgOrStringTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestSubnetListOrArgsTransformer_Transformer(t *testing.T) {
	mockSubnetIDs := []string{"id1", "id2"}
	mockSubnetFromTags := map[string]StringSliceOrString{
		"foo": {
			String: aws.String("bar"),
		},
	}
	testCases := map[string]struct {
		original func(p *SubnetListOrArgs)
		override func(p *SubnetListOrArgs)
		wanted   func(p *SubnetListOrArgs)
	}{
		"string slice set to empty if args is not nil": {
			original: func(s *SubnetListOrArgs) {
				s.IDs = mockSubnetIDs
			},
			override: func(s *SubnetListOrArgs) {
				s.SubnetArgs = SubnetArgs{
					FromTags: mockSubnetFromTags,
				}
			},
			wanted: func(s *SubnetListOrArgs) {
				s.SubnetArgs = SubnetArgs{
					FromTags: mockSubnetFromTags,
				}
			},
		},
		"args set to empty if string is not nil": {
			original: func(s *SubnetListOrArgs) {
				s.SubnetArgs = SubnetArgs{
					FromTags: mockSubnetFromTags,
				}
			},
			override: func(s *SubnetListOrArgs) {
				s.IDs = mockSubnetIDs
			},
			wanted: func(s *SubnetListOrArgs) {
				s.IDs = mockSubnetIDs
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted SubnetListOrArgs

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use custom transformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(subnetListOrArgsTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestServiceConnectTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(p *ServiceConnectBoolOrArgs)
		override func(p *ServiceConnectBoolOrArgs)
		wanted   func(p *ServiceConnectBoolOrArgs)
	}{
		"bool set to empty if args is not nil": {
			original: func(s *ServiceConnectBoolOrArgs) {
				s.EnableServiceConnect = aws.Bool(false)
			},
			override: func(s *ServiceConnectBoolOrArgs) {
				s.ServiceConnectArgs = ServiceConnectArgs{
					Alias: aws.String("api"),
				}
			},
			wanted: func(s *ServiceConnectBoolOrArgs) {
				s.ServiceConnectArgs = ServiceConnectArgs{
					Alias: aws.String("api"),
				}
			},
		},
		"args set to empty if bool is not nil": {
			original: func(s *ServiceConnectBoolOrArgs) {
				s.ServiceConnectArgs = ServiceConnectArgs{
					Alias: aws.String("api"),
				}
			},
			override: func(s *ServiceConnectBoolOrArgs) {
				s.EnableServiceConnect = aws.Bool(true)
			},
			wanted: func(s *ServiceConnectBoolOrArgs) {
				s.EnableServiceConnect = aws.Bool(true)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted ServiceConnectBoolOrArgs

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use custom transformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(serviceConnectTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

type unionTransformerTest[Basic, Advanced any] struct {
	original Union[Basic, Advanced]
	override Union[Basic, Advanced]
	expected Union[Basic, Advanced]
}

func TestTransformer_Generic(t *testing.T) {
	runUnionTransformerTests(t, map[string]unionTransformerTest[any, any]{
		"switches to Simple from Advanced if overridden": {
			original: AdvancedToUnion[any, any](nil),
			override: BasicToUnion[any, any](nil),
			expected: BasicToUnion[any, any](nil),
		},
		"switches to Advanced from Simple if overridden": {
			original: BasicToUnion[any, any](nil),
			override: AdvancedToUnion[any, any](nil),
			expected: AdvancedToUnion[any, any](nil),
		},
		"switches to Simple if original unset": {
			original: Union[any, any]{},
			override: BasicToUnion[any, any](nil),
			expected: BasicToUnion[any, any](nil),
		},
		"switches to Advanced if original unset": {
			original: Union[any, any]{},
			override: AdvancedToUnion[any, any](nil),
			expected: AdvancedToUnion[any, any](nil),
		},
	})
}

func TestTransformer_StringOrHealthCheckArgs(t *testing.T) {
	runUnionTransformerTests(t, map[string]unionTransformerTest[string, HTTPHealthCheckArgs]{
		"string unset if args set": {
			original: BasicToUnion[string, HTTPHealthCheckArgs]("mockPath"),
			override: AdvancedToUnion[string](HTTPHealthCheckArgs{
				Path:         aws.String("mockPathArgs"),
				SuccessCodes: aws.String("200"),
			}),
			expected: AdvancedToUnion[string](HTTPHealthCheckArgs{
				Path:         aws.String("mockPathArgs"),
				SuccessCodes: aws.String("200"),
			}),
		},
		"args unset if string set": {
			original: AdvancedToUnion[string](HTTPHealthCheckArgs{
				Path:         aws.String("mockPathArgs"),
				SuccessCodes: aws.String("200"),
			}),
			override: BasicToUnion[string, HTTPHealthCheckArgs]("mockPath"),
			expected: BasicToUnion[string, HTTPHealthCheckArgs]("mockPath"),
		},
		"string merges correctly": {
			original: BasicToUnion[string, HTTPHealthCheckArgs]("path"),
			override: BasicToUnion[string, HTTPHealthCheckArgs]("newPath"),
			expected: BasicToUnion[string, HTTPHealthCheckArgs]("newPath"),
		},
		"args merge correctly": {
			original: AdvancedToUnion[string](HTTPHealthCheckArgs{
				Path:             aws.String("mockPathArgs"),
				SuccessCodes:     aws.String("200"),
				HealthyThreshold: aws.Int64(10),
			}),
			override: AdvancedToUnion[string](HTTPHealthCheckArgs{
				SuccessCodes:       aws.String("420"),
				UnhealthyThreshold: aws.Int64(20),
			}),
			expected: AdvancedToUnion[string](HTTPHealthCheckArgs{
				Path:               aws.String("mockPathArgs"), // merged unchanged
				SuccessCodes:       aws.String("420"),          // updated
				HealthyThreshold:   aws.Int64(10),              // comes from original
				UnhealthyThreshold: aws.Int64(20),              // comes from override
			}),
		},
	})
}

func runUnionTransformerTests[Basic, Advanced any](t *testing.T, tests map[string]unionTransformerTest[Basic, Advanced]) {
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Perform default merge.
			err := mergo.Merge(&tc.original, tc.override, mergo.WithOverride)
			require.NoError(t, err)

			// Use custom transformer.
			err = mergo.Merge(&tc.original, tc.override, mergo.WithOverride, mergo.WithTransformers(unionTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, tc.expected, tc.original)
		})
	}
}

func TestUnionPanicRecover(t *testing.T) {
	// trick the transformer logic into thinking
	// this is the real manifest.Union type
	type Union[T any] struct{}
	err := mergo.Merge(&Union[any]{}, &Union[any]{}, mergo.WithTransformers(unionTransformer{}))
	require.EqualError(t, err, "override union: reflect: call of reflect.Value.Call on zero Value")
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

			// Use custom transformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(countTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestAdvancedCountTransformer_Transformer(t *testing.T) {
	perc := Percentage(80)
	mockConfig := ScalingConfigOrT[Percentage]{
		Value: &perc,
	}
	mockReq := ScalingConfigOrT[int]{
		Value: aws.Int(42),
	}
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
				a.Range = Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				}
				a.CPU = mockConfig
				a.Requests = mockReq
			},
			wanted: func(a *AdvancedCount) {
				a.Range = Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				}
				a.CPU = mockConfig
				a.Requests = mockReq
			},
		},
		"auto scaling set to empty if spot is not nil": {
			original: func(a *AdvancedCount) {
				a.Range = Range{
					Value: (*IntRangeBand)(aws.String("1-10")),
				}
				a.CPU = mockConfig
				a.Requests = mockReq
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

			// Use custom transformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(advancedCountTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestScalingConfigOrT_Transformer(t *testing.T) {
	perc := Percentage(80)
	mockConfig := AdvancedScalingConfig[Percentage]{
		Value: &perc,
	}
	testCases := map[string]struct {
		original func(s *ScalingConfigOrT[Percentage])
		override func(s *ScalingConfigOrT[Percentage])
		wanted   func(s *ScalingConfigOrT[Percentage])
	}{
		"advanced config value set to nil if percentage is not nil": {
			original: func(s *ScalingConfigOrT[Percentage]) {
				s.ScalingConfig = mockConfig
			},
			override: func(s *ScalingConfigOrT[Percentage]) {
				s.Value = &perc
			},
			wanted: func(s *ScalingConfigOrT[Percentage]) {
				s.Value = &perc
			},
		},
		"percentage set to nil if advanced config value is not nil": {
			original: func(s *ScalingConfigOrT[Percentage]) {
				s.Value = &perc
			},
			override: func(s *ScalingConfigOrT[Percentage]) {
				s.ScalingConfig = mockConfig
			},
			wanted: func(s *ScalingConfigOrT[Percentage]) {
				s.ScalingConfig = mockConfig
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted ScalingConfigOrT[Percentage]

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use custom transformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(scalingConfigOrTTransformer[Percentage]{}))
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

			// Use custom transformer.
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

			// Use custom transformer.
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
				e.FileSystemID = StringOrFromCFN{Plain: aws.String("mockFileSystem")}
				e.RootDirectory = aws.String("mockRootDir")
			},
			wanted: func(e *EFSVolumeConfiguration) {
				e.FileSystemID = StringOrFromCFN{Plain: aws.String("mockFileSystem")}
				e.RootDirectory = aws.String("mockRootDir")
			},
		},
		"BYO config set to empty if UID config is not empty": {
			original: func(e *EFSVolumeConfiguration) {
				e.FileSystemID = StringOrFromCFN{FromCFN: fromCFN{Name: aws.String("mockFileSystem")}}
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

			// Use custom transformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(efsVolumeConfigurationTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestSQSQueueOrBoolTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(e *SQSQueueOrBool)
		override func(e *SQSQueueOrBool)
		wanted   func(e *SQSQueueOrBool)
	}{
		"bool set to empty if config is not nil": {
			original: func(e *SQSQueueOrBool) {
				e.Enabled = aws.Bool(true)
			},
			override: func(e *SQSQueueOrBool) {
				e.Advanced = SQSQueue{
					Retention: durationp(5 * time.Second),
				}
			},
			wanted: func(e *SQSQueueOrBool) {
				e.Advanced = SQSQueue{
					Retention: durationp(5 * time.Second),
				}
			},
		},
		"config set to empty if bool is not nil": {
			original: func(e *SQSQueueOrBool) {
				e.Advanced = SQSQueue{
					Retention: durationp(5 * time.Second),
				}
			},
			override: func(e *SQSQueueOrBool) {
				e.Enabled = aws.Bool(true)
			},
			wanted: func(e *SQSQueueOrBool) {
				e.Enabled = aws.Bool(true)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted SQSQueueOrBool

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use custom transformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(sqsQueueOrBoolTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestHTTPOrBoolTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(r *HTTPOrBool)
		override func(r *HTTPOrBool)
		wanted   func(r *HTTPOrBool)
	}{
		"bool set to empty if config is not nil": {
			original: func(r *HTTPOrBool) {
				r.Enabled = aws.Bool(true)
			},
			override: func(r *HTTPOrBool) {
				r.HTTP = HTTP{
					Main: RoutingRule{
						Path: aws.String("mockPath"),
					},
				}
			},
			wanted: func(r *HTTPOrBool) {
				r.HTTP = HTTP{
					Main: RoutingRule{
						Path: aws.String("mockPath"),
					},
				}
			},
		},
		"config set to empty if bool is not nil": {
			original: func(r *HTTPOrBool) {
				r.HTTP = HTTP{
					Main: RoutingRule{
						Path: aws.String("mockPath"),
					},
				}
			},
			override: func(r *HTTPOrBool) {
				r.Enabled = aws.Bool(false)
			},
			wanted: func(r *HTTPOrBool) {
				r.Enabled = aws.Bool(false)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted HTTPOrBool

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use custom transformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(httpOrBoolTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestSecretTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(s *Secret)
		override func(s *Secret)
		wanted   func(s *Secret)
	}{
		`"from" set to empty when overriding with "secretsmanager"`: {
			original: func(s *Secret) {
				s.from = StringOrFromCFN{
					Plain: aws.String("/github/token"),
				}
			},
			override: func(s *Secret) {
				s.fromSecretsManager = secretsManagerSecret{
					Name: aws.String("aes128-1a2b3c"),
				}
			},
			wanted: func(s *Secret) {
				s.fromSecretsManager = secretsManagerSecret{
					Name: aws.String("aes128-1a2b3c"),
				}
			},
		},
		`"secretsmanager" set to empty when overriding with "from"`: {
			original: func(s *Secret) {
				s.fromSecretsManager = secretsManagerSecret{
					Name: aws.String("aes128-1a2b3c"),
				}
			},
			override: func(s *Secret) {
				s.from = StringOrFromCFN{
					Plain: aws.String("/github/token"),
				}
			},
			wanted: func(s *Secret) {
				s.from = StringOrFromCFN{
					Plain: aws.String("/github/token"),
				}
			},
		},
		`"secretsmanager" set to empty when overriding with imported "from"`: {
			original: func(s *Secret) {
				s.fromSecretsManager = secretsManagerSecret{
					Name: aws.String("aes128-1a2b3c"),
				}
			},
			override: func(s *Secret) {
				s.from = StringOrFromCFN{
					FromCFN: fromCFN{
						Name: aws.String("stack-SSMGHTokenName"),
					},
				}
			},
			wanted: func(s *Secret) {
				s.from = StringOrFromCFN{
					FromCFN: fromCFN{
						Name: aws.String("stack-SSMGHTokenName"),
					},
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted Secret

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use custom transformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(secretTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

func TestEnvironmentCDNConfigTransformer_Transformer(t *testing.T) {
	testCases := map[string]struct {
		original func(cfg *EnvironmentCDNConfig)
		override func(cfg *EnvironmentCDNConfig)
		wanted   func(cfg *EnvironmentCDNConfig)
	}{
		"cdnconfig set to empty if enabled is not nil": {
			original: func(cfg *EnvironmentCDNConfig) {
				cfg.Config = AdvancedCDNConfig{
					Certificate: aws.String("arn:aws:acm:us-east-1:1111111:certificate/look-like-a-good-arn"),
				}
			},
			override: func(cfg *EnvironmentCDNConfig) {
				cfg.Enabled = aws.Bool(true)
			},
			wanted: func(cfg *EnvironmentCDNConfig) {
				cfg.Enabled = aws.Bool(true)
			},
		},
		"enabled set to nil if cdnconfig is not empty": {
			original: func(cfg *EnvironmentCDNConfig) {
				cfg.Enabled = aws.Bool(true)
			},
			override: func(cfg *EnvironmentCDNConfig) {
				cfg.Config = AdvancedCDNConfig{
					Certificate: aws.String("arn:aws:acm:us-east-1:1111111:certificate/look-like-a-good-arn"),
				}
			},
			wanted: func(cfg *EnvironmentCDNConfig) {
				cfg.Config = AdvancedCDNConfig{
					Certificate: aws.String("arn:aws:acm:us-east-1:1111111:certificate/look-like-a-good-arn"),
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			var dst, override, wanted EnvironmentCDNConfig

			tc.original(&dst)
			tc.override(&override)
			tc.wanted(&wanted)

			// Perform default merge.
			err := mergo.Merge(&dst, override, mergo.WithOverride)
			require.NoError(t, err)

			// Use custom transformer.
			err = mergo.Merge(&dst, override, mergo.WithOverride, mergo.WithTransformers(environmentCDNConfigTransformer{}))
			require.NoError(t, err)

			require.NoError(t, err)
			require.Equal(t, wanted, dst)
		})
	}
}

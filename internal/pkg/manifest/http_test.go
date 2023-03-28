// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestHTTPOrBool_Disabled(t *testing.T) {
	testCases := map[string]struct {
		in     HTTPOrBool
		wanted bool
	}{
		"disabled": {
			in: HTTPOrBool{
				Enabled: aws.Bool(false),
			},
			wanted: true,
		},
		"enabled implicitly": {
			in: HTTPOrBool{},
		},
		"enabled explicitly": {
			in: HTTPOrBool{
				Enabled: aws.Bool(true),
			},
		},
		"enabled explicitly by advanced configuration": {
			in: HTTPOrBool{
				HTTP: HTTP{
					Main: RoutingRule{
						Path: aws.String("mockPath"),
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			got := tc.in.Disabled()

			// THEN
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestAlias_HostedZones(t *testing.T) {
	testCases := map[string]struct {
		in     Alias
		wanted []string
	}{
		"no hosted zone": {
			in: Alias{
				AdvancedAliases: []AdvancedAlias{},
			},
			wanted: []string{},
		},
		"with hosted zones": {
			in: Alias{
				AdvancedAliases: []AdvancedAlias{
					{
						HostedZone: aws.String("mockHostedZone1"),
					},
					{
						HostedZone: aws.String("mockHostedZone2"),
					},
				},
			},
			wanted: []string{"mockHostedZone1", "mockHostedZone2"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			got := tc.in.HostedZones()
			// THEN
			require.ElementsMatch(t, tc.wanted, got)
		})
	}
}

func TestAlias_UnmarshalYAML(t *testing.T) {
	testCases := map[string]struct {
		inContent []byte

		wantedStruct Alias
		wantedError  error
	}{
		"Alias specified in string": {
			inContent: []byte(`alias: foobar.com`),
			wantedStruct: Alias{
				StringSliceOrString: StringSliceOrString{
					String: aws.String("foobar.com"),
				},
			},
		},
		"Alias specified in slice of strings": {
			inContent: []byte(`alias:
  - example.com
  - v1.example.com`),
			wantedStruct: Alias{
				StringSliceOrString: StringSliceOrString{
					StringSlice: []string{"example.com", "v1.example.com"},
				},
				AdvancedAliases: []AdvancedAlias{},
			},
		},
		"Alias specified in advanced alias slice": {
			inContent: []byte(`alias:
  - name: example.com
    hosted_zone: Z0873220N255IR3MTNR4
  - name: foobar.com`),
			wantedStruct: Alias{
				AdvancedAliases: []AdvancedAlias{
					{
						Alias:      aws.String("example.com"),
						HostedZone: aws.String("Z0873220N255IR3MTNR4"),
					},
					{
						Alias: aws.String("foobar.com"),
					},
				},
			},
		},
		"Error if unmarshalable": {
			inContent: []byte(`alias:
  foo: bar`),
			wantedStruct: Alias{
				StringSliceOrString: StringSliceOrString{},
				AdvancedAliases:     []AdvancedAlias{},
			},
			wantedError: errUnmarshalAlias,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			r := HTTP{
				Main: RoutingRule{
					Alias: Alias{
						StringSliceOrString: StringSliceOrString{
							String: aws.String("wrong"),
						},
					},
				},
			}

			err := yaml.Unmarshal(tc.inContent, &r)
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				// check memberwise dereferenced pointer equality
				require.Equal(t, tc.wantedStruct.StringSliceOrString, r.Main.Alias.StringSliceOrString)
				require.Equal(t, tc.wantedStruct.AdvancedAliases, r.Main.Alias.AdvancedAliases)
			}
		})
	}
}

func TestAlias_IsEmpty(t *testing.T) {
	testCases := map[string]struct {
		in     Alias
		wanted bool
	}{
		"empty alias": {
			in:     Alias{},
			wanted: true,
		},
		"non empty alias": {
			in: Alias{
				StringSliceOrString: StringSliceOrString{
					String: aws.String("alias test"),
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

func TestAlias_ToString(t *testing.T) {
	testCases := map[string]struct {
		inAlias Alias
		wanted  string
	}{
		"alias using string": {
			inAlias: Alias{
				StringSliceOrString: StringSliceOrString{
					String: stringP("example.com"),
				},
			},
			wanted: "example.com",
		},
		"alias using string slice": {
			inAlias: Alias{
				StringSliceOrString: StringSliceOrString{
					StringSlice: []string{"example.com", "v1.example.com"},
				},
			},
			wanted: "example.com,v1.example.com",
		},
		"alias using advanced alias slice": {
			inAlias: Alias{
				AdvancedAliases: []AdvancedAlias{
					{
						Alias: aws.String("example.com"),
					},
					{
						Alias: aws.String("v1.example.com"),
					},
				},
			},
			wanted: "example.com,v1.example.com",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			got := tc.inAlias.ToString()

			// THEN
			require.Equal(t, tc.wanted, got)
		})
	}
}

func TestAlias_ToStringSlice(t *testing.T) {
	testCases := map[string]struct {
		inAlias Alias
		wanted  []string
	}{
		"alias using string": {
			inAlias: Alias{
				StringSliceOrString: StringSliceOrString{
					String: stringP("example.com"),
				},
			},
			wanted: []string{"example.com"},
		},
		"alias using string slice": {
			inAlias: Alias{
				StringSliceOrString: StringSliceOrString{
					StringSlice: []string{"example.com", "v1.example.com"},
				},
			},
			wanted: []string{"example.com", "v1.example.com"},
		},
		"alias using advanced alias slice": {
			inAlias: Alias{
				AdvancedAliases: []AdvancedAlias{
					{
						Alias: aws.String("example.com"),
					},
					{
						Alias: aws.String("v1.example.com"),
					},
				},
			},
			wanted: []string{"example.com", "v1.example.com"},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			got, _ := tc.inAlias.ToStringSlice()

			// THEN
			require.Equal(t, tc.wanted, got)
		})
	}
}

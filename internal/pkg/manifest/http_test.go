// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/stretchr/testify/require"
)

func TestRoutingRuleConfigOrBool_Disabled(t *testing.T) {
	testCases := map[string]struct {
		in     RoutingRuleConfigOrBool
		wanted bool
	}{
		"disabled": {
			in: RoutingRuleConfigOrBool{
				Enabled: aws.Bool(false),
			},
			wanted: true,
		},
		"enabled implicitly": {
			in: RoutingRuleConfigOrBool{},
		},
		"enabled explicitly": {
			in: RoutingRuleConfigOrBool{
				Enabled: aws.Bool(true),
			},
		},
		"enabled explicitly by advanced configuration": {
			in: RoutingRuleConfigOrBool{
				RoutingRuleConfiguration: RoutingRuleConfiguration{
					Path: aws.String("mockPath"),
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

func TestRoutingRuleConfigOrBool_EmptyOrDisabled(t *testing.T) {
	testCases := map[string]struct {
		in     RoutingRuleConfigOrBool
		wanted bool
	}{
		"disabled": {
			in: RoutingRuleConfigOrBool{
				Enabled: aws.Bool(false),
			},
			wanted: true,
		},
		"empty": {
			in:     RoutingRuleConfigOrBool{},
			wanted: true,
		},
		"enabled explicitly": {
			in: RoutingRuleConfigOrBool{
				Enabled: aws.Bool(true),
			},
		},
		"enabled explicitly by advanced configuration": {
			in: RoutingRuleConfigOrBool{
				RoutingRuleConfiguration: RoutingRuleConfiguration{
					Path: aws.String("mockPath"),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			got := tc.in.EmptyOrDisabled()

			// THEN
			require.Equal(t, tc.wanted, got)
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
				String: aws.String("alias test"),
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
				String: stringP("example.com"),
			},
			wanted: "example.com",
		},
		"alias using string slice": {
			inAlias: Alias{
				StringSlice: []string{"example.com", "v1.example.com"},
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

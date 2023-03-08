// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_longestCommonSubsequence(t *testing.T) {
	testCases := []struct {
		inA    []string
		inB    []string
		wanted []string
	}{
		{
			inA:    []string{"a", "b", "c"},
			inB:    []string{"a", "b", "c"},
			wanted: []string{"a", "b", "c"},
		},
		{
			inA:    []string{"a", "b", "c"},
			inB:    []string{"c"},
			wanted: []string{"c"},
		},
		{
			inA:    []string{"a", "b", "c"},
			inB:    []string{"a", "B", "c"},
			wanted: []string{"a", "c"},
		},
		{
			inA:    []string{"a", "b", "c"},
			inB:    []string{"a", "B", "C"},
			wanted: []string{"a"},
		},
		{
			inA:    []string{"a", "c", "b", "b", "d"},
			inB:    []string{"a", "B", "b", "c", "c", "d"},
			wanted: []string{"a", "b", "d"}, // NOTE: a, c, d is also correct
		},
		{
			inA:    []string{"a", "b", "B", "B", "c", "d", "D", "d", "e", "f"},
			inB:    []string{"a", "B", "C", "d", "d", "e", "f"},
			wanted: []string{"a", "B", "d", "d", "e", "f"},
		},
		{
			inB: []string{},
		},
		{
			inA:    []string{"a"},
			wanted: nil,
		},
		{
			inA:    []string{"a"},
			inB:    []string{"a"},
			wanted: []string{"a"},
		},
		{
			inA:    []string{"a"},
			inB:    []string{"b"},
			wanted: nil,
		},
		{
			inA:    []string{"a", "b", "c", "c"},
			inB:    []string{"c"},
			wanted: []string{"c"},
		},
	}
	for idx, tc := range testCases {
		t.Run(fmt.Sprintf("casess %v", idx), func(t *testing.T) {
			require.Equal(t, tc.wanted, longestCommonSubsequence(tc.inA, tc.inB))
		})
	}
}

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
		inA     []string
		inB     []string
		wantedA []int
		wantedB []int
	}{
		{
			inA:     []string{"a", "b", "c"},
			inB:     []string{"a", "b", "c"},
			wantedA: []int{0, 1, 2},
			wantedB: []int{0, 1, 2},
		},
		{
			inA:     []string{"a", "b", "c"},
			inB:     []string{"c"},
			wantedA: []int{2},
			wantedB: []int{0},
		},
		{
			inA:     []string{"a", "b", "c"},
			inB:     []string{"a", "B", "c"},
			wantedA: []int{0, 2},
			wantedB: []int{0, 2},
		},
		{
			inA:     []string{"a", "b", "c"},
			inB:     []string{"a", "B", "C"},
			wantedA: []int{0},
			wantedB: []int{0},
		},
		{
			inA: []string{"a", "c", "b", "b", "d"},
			inB: []string{"a", "B", "b", "c", "c", "d"},
			// NOTE: the wanted sequence here is a,b,d; however, a, c, d is also correct.
			wantedA: []int{0, 3, 4}, // NOTE: 0, 2, 4 is also correct.
			wantedB: []int{0, 2, 5},
		},
		{
			inA:     []string{"a", "b", "B", "B", "c", "d", "D", "d", "e", "f"},
			inB:     []string{"a", "B", "C", "d", "d", "e", "f"},
			wantedA: []int{0, 2, 5, 7, 8, 9},
			wantedB: []int{0, 1, 3, 4, 5, 6},
		},
		{
			inB: []string{},
		},
		{
			inA: []string{"a"},
		},
		{
			inA:     []string{"a"},
			inB:     []string{"a"},
			wantedA: []int{0},
			wantedB: []int{0},
		},
		{
			inA: []string{"a"},
			inB: []string{"b"},
		},
		{
			inA:     []string{"a", "b", "c", "c"},
			inB:     []string{"c"},
			wantedA: []int{2},
			wantedB: []int{0},
		},
	}
	for idx, tc := range testCases {
		t.Run(fmt.Sprintf("case %v", idx), func(t *testing.T) {
			gotA, gotB := longestCommonSubsequence(tc.inA, tc.inB)
			require.Equal(t, tc.wantedA, gotA)
			require.Equal(t, tc.wantedB, gotB)
		})
	}
}

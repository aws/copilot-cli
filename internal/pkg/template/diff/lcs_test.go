// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func Test_longestCommonSubsequence_string(t *testing.T) {
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
		t.Run(fmt.Sprintf("string case %v", idx), func(t *testing.T) {
			got := longestCommonSubsequence(tc.inA, tc.inB, func(inA, inB int) bool { return tc.inA[inA] == tc.inB[inB] })
			var wanted []lcsIndex
			for idx := range tc.wantedA {
				wanted = append(wanted, lcsIndex{
					inA: tc.wantedA[idx],
					inB: tc.wantedB[idx],
				})
			}
			require.Equal(t, wanted, got)
		})
	}

}

func Test_longestCommonSubsequence_yamlNode(t *testing.T) {
	testCases := map[string]struct {
		inA string
		inB string

		mockEq eqFunc

		wantedA []int
		wantedB []int
	}{
		"map/reorder map field": { // The order of map fields should not matter.
			inA: `- Sid: power
  Action: '*'
  Resources: '*'`,
			inB: `- Sid: power
  Resources: '*'
  Action: '*'`,
			mockEq: func(inA, inB int) bool {
				return true
			},
			wantedA: []int{0},
			wantedB: []int{0},
		},
		"map/change node scalar style": { // The style should not matter.
			inA: `- Sid: 'power'`,
			inB: `- Sid: "power"`,
			mockEq: func(inA, inB int) bool {
				return true
			},
			wantedA: []int{0},
			wantedB: []int{0},
		},
		"map/not equal": {
			inA: `- Sid: power
  Action: '*'`,
			inB: `- Sid: no power
  Resources: '*'`,
			mockEq: func(inA, inB int) bool {
				return false
			},
		},
		"map/reorder": {
			inA: `- Sid: power
- Sid: less power`,
			inB: `- Sid: less power
- Sid: power`,
			mockEq: func(inA, inB int) bool {
				return (inA == 0 && inB == 1) || (inA == 1 && inB == 0)
			},
			wantedA: []int{1}, // Note: wantedA, wantedB = [0], [1] is also correct.
			wantedB: []int{0},
		},
		"scalar/change style": { // The style should not matter.
			inA: `['a','b']`,
			inB: `["a","b"]`,
			mockEq: func(inA, inB int) bool {
				return inA == inB
			},
			wantedA: []int{0, 1},
			wantedB: []int{0, 1},
		},
		"scalar/mixed style": { // The style should not matter.
			inA: `['a',"b"]`,
			inB: `["a",'b']`,
			mockEq: func(inA, inB int) bool {
				return inA == inB
			},
			wantedA: []int{0, 1},
			wantedB: []int{0, 1},
		},
		"scalar/not equal": { // The style should not matter.
			inA: `[a,b,c,d]`,
			inB: `[a,d]`,
			mockEq: func(inA, inB int) bool {
				return inA == 0 && inB == 0 || inA == 3 && inB == 1
			},
			wantedA: []int{0, 3},
			wantedB: []int{0, 1},
		},
		"change list style": {
			inA: `- a
- b
- c`,
			inB: `[a,b,c]`,
			mockEq: func(inA, inB int) bool {
				return inA == inB
			},
			wantedA: []int{0, 1, 2}, // Note: wantedA, wantedB = [0], [1] is also correct.
			wantedB: []int{0, 1, 2},
		},
		"change item kind": {
			inA: `- a
- b
- c`,
			inB: `- Sid: hey
- a
`,
			mockEq: func(inA, inB int) bool {
				return inA == 0 && inB == 1
			},
			wantedA: []int{0}, // Note: wantedA, wantedB = [0], [1] is also correct.
			wantedB: []int{1},
		},
	}
	for idx, tc := range testCases {
		t.Run(idx, func(t *testing.T) {
			var inANode, inBNode []yaml.Node
			require.NoError(t, yaml.Unmarshal([]byte(tc.inA), &inANode))
			require.NoError(t, yaml.Unmarshal([]byte(tc.inB), &inBNode))
			got := longestCommonSubsequence(inANode, inBNode, tc.mockEq)
			var wanted []lcsIndex
			for idx := range tc.wantedA {
				wanted = append(wanted, lcsIndex{
					inA: tc.wantedA[idx],
					inB: tc.wantedB[idx],
				})
			}
			require.Equal(t, wanted, got)
		})
	}
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConstructDiffTree(t *testing.T) {
	testCases := map[string]struct {
		curr        string
		old         string
		wanted      *Node
		wantedError error
	}{
		"add a map":                {},
		"remove a map":             {},
		"add an item to a list":    {},
		"remove an item to a list": {},
		"change a keyed value":     {},
		"change a list item value": {},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			got, err := Parse([]byte(tc.curr), []byte(tc.old))
			if tc.wantedError != nil {
				require.Equal(t, tc.wanted, err)
			}
			require.True(t, equalTree(got, &Node{}, t))
		})
	}
}

func equalLeaves(a, b *Node, t *testing.T) bool {
	aNew, err := yaml.Marshal(a.newValue)
	require.NoError(t, err)
	bNew, err := yaml.Marshal(b.newValue)
	require.NoError(t, err)
	aOld, err := yaml.Marshal(a.oldValue)
	require.NoError(t, err)
	bOld, err := yaml.Marshal(b.oldValue)
	require.NoError(t, err)
	return string(aNew) == string(bNew) && string(aOld) == string(bOld)
}

func equalTree(a, b *Node, t *testing.T) bool {
	if a.key != b.key || len(a.children) != len(b.children) {
		return false
	}
	if len(a.children) == 0 {
		return equalLeaves(a, b, t)
	}
	for k := range a.children {
		if equal := equalTree(a.children[k], b.children[k], t); !equal {
			return false
		}
	}
	return true
}

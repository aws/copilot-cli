// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGraph_Add(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// GIVEN
		graph := New()

		// WHEN
		graph.Add("A", "B")
		graph.Add("B", "A")
		graph.Add("A", "C")

		// THEN
		require.ElementsMatch(t, graph.Nodes["A"], []string{"B", "C"})
		require.ElementsMatch(t, graph.Nodes["B"], []string{"A"})
	})
}

func TestGraph_IsAcyclic(t *testing.T) {
	testCases := map[string]struct {
		graph Graph

		isAcyclic bool
		cycle     []string
	}{
		"small non acyclic graph": {
			graph: Graph{
				Nodes: map[string][]string{
					"A": {"B", "C"},
					"B": {"A"},
				},
			},

			isAcyclic: false,
			cycle:     []string{"A", "B"},
		},
		"non acyclic": {
			graph: Graph{
				Nodes: map[string][]string{
					"K": {"F"},
					"A": {"B", "C"},
					"B": {"D", "E"},
					"E": {"G"},
					"F": {"G"},
					"G": {"A"},
				},
			},

			isAcyclic: false,
			cycle:     []string{"A", "G", "E", "B"},
		},
		"acyclic": {
			graph: Graph{
				Nodes: map[string][]string{
					"A": {"B", "C"},
					"B": {"D"},
					"E": {"G"},
					"F": {"G"},
				},
			},

			isAcyclic: true,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			gotCycle, gotAcyclic := tc.graph.IsAcyclic()

			// THEN
			require.Equal(t, tc.isAcyclic, gotAcyclic)
			require.ElementsMatch(t, tc.cycle, gotCycle)
		})
	}
}

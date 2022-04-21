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
		graph := New[string]()

		// WHEN
		graph.Add(Edge[string]{
			From: "A",
			To:   "B",
		})
		graph.Add(Edge[string]{
			From: "B",
			To:   "A",
		})
		graph.Add(Edge[string]{
			From: "A",
			To:   "C",
		})

		// THEN
		require.Equal(t, graph.nodes["A"], neighbors[string]{"B": true, "C": true})
		require.Equal(t, graph.nodes["B"], neighbors[string]{"A": true})
	})
}

func TestGraph_IsAcyclic(t *testing.T) {
	testCases := map[string]struct {
		graph Graph[string]

		isAcyclic bool
		cycle     []string
	}{
		"small non acyclic graph": {
			graph: Graph[string]{
				nodes: map[string]neighbors[string]{
					"A": {"B": true, "C": true},
					"B": {"A": true},
				},
			},

			isAcyclic: false,
			cycle:     []string{"A", "B"},
		},
		"non acyclic": {
			graph: Graph[string]{
				nodes: map[string]neighbors[string]{
					"K": {"F": true},
					"A": {"B": true, "C": true},
					"B": {"D": true, "E": true},
					"E": {"G": true},
					"F": {"G": true},
					"G": {"A": true},
				},
			},

			isAcyclic: false,
			cycle:     []string{"A", "G", "E", "B"},
		},
		"acyclic": {
			graph: Graph[string]{
				nodes: map[string]neighbors[string]{
					"A": {"B": true, "C": true},
					"B": {"D": true},
					"E": {"G": true},
					"F": {"G": true},
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

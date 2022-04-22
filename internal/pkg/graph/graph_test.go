// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"strings"
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
		require.Equal(t, graph.vertices["A"], neighbors[string]{"B": true, "C": true})
		require.Equal(t, graph.vertices["B"], neighbors[string]{"A": true})
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
				vertices: map[string]neighbors[string]{
					"A": {"B": true, "C": true},
					"B": {"A": true},
				},
			},

			isAcyclic: false,
			cycle:     []string{"A", "B"},
		},
		"non acyclic": {
			graph: Graph[string]{
				vertices: map[string]neighbors[string]{
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
				vertices: map[string]neighbors[string]{
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

func TestGraph_Roots(t *testing.T) {
	testCases := map[string]struct {
		graph *Graph[int]

		wantedRoots []int
	}{
		"should return nil if the graph is empty": {
			graph: New[int](),
		},
		"should return all the vertices if there are no edges in the graph": {
			graph:       New[int](1, 2, 3, 4, 5),
			wantedRoots: []int{1, 2, 3, 4, 5},
		},
		"should return only vertices with no in degrees": {
			graph: func() *Graph[int] {
				g := New[int]()
				g.Add(Edge[int]{
					From: 1,
					To:   3,
				})
				g.Add(Edge[int]{
					From: 2,
					To:   3,
				})
				g.Add(Edge[int]{
					From: 3,
					To:   4,
				})
				return g
			}(),

			wantedRoots: []int{1, 2},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.ElementsMatch(t, tc.wantedRoots, tc.graph.Roots())
		})
	}
}

func TestLevelOrderTraversal(t *testing.T) {
	testCases := map[string]struct {
		graph *Graph[string]

		wantedRanks     map[string]int
		wantedErrPrefix string
	}{
		"should return an error when a cycle is detected": {
			graph: &Graph[string]{
				vertices: map[string]neighbors[string]{
					"frontend": map[string]bool{
						"backend": true,
					},
					"backend": map[string]bool{
						"frontend": true,
					},
				},
			},
			wantedErrPrefix: "graph contains a cycle: ", // the cycle can appear in any order as map traversals are not deterministic, so only check the prefix.
		},
		"should return the ranks for a graph that looks like a bus": {
			graph: &Graph[string]{
				vertices: map[string]neighbors[string]{
					"api": nil,
					"lb": map[string]bool{
						"api": true,
					},
					"vpc": map[string]bool{
						"lb": true,
					},
				},
			},

			wantedRanks: map[string]int{
				"api": 2,
				"lb":  1,
				"vpc": 0,
			},
		},
		"should return the ranks for a graph that looks like a tree": {
			graph: &Graph[string]{
				vertices: map[string]neighbors[string]{
					"api":      nil,
					"frontend": nil,
					"backend":  nil,
					"s3": map[string]bool{
						"api":      true,
						"frontend": true,
					},
					"rds": map[string]bool{
						"backend": true,
					},
					"vpc": map[string]bool{
						"rds": true,
						"s3":  true,
					},
				},
			},

			wantedRanks: map[string]int{
				"api":      2,
				"frontend": 2,
				"backend":  2,
				"s3":       1,
				"rds":      1,
				"vpc":      0,
			},
		},
		"should return the ranks for a graph with multiple root nodes": {
			graph: &Graph[string]{
				vertices: map[string]neighbors[string]{
					"frontend": nil,
					"orders": map[string]bool{
						"frontend": true,
					},
					"warehouse": map[string]bool{
						"orders": true,
					},
					"payments": map[string]bool{
						"frontend": true,
					},
				},
			},

			wantedRanks: map[string]int{
				"frontend":  2,
				"orders":    1,
				"warehouse": 0,
				"payments":  0,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			topo, err := LevelOrderTraversal(tc.graph)

			if tc.wantedErrPrefix != "" {
				require.Error(t, err)
				require.True(t, strings.HasPrefix(err.Error(), tc.wantedErrPrefix))
			} else {
				require.NoError(t, err)

				for vtx, wantedRank := range tc.wantedRanks {
					rank, _ := topo.Rank(vtx)
					require.Equal(t, wantedRank, rank, "expected rank for vertex %s does not match", vtx)
				}
			}
		})
	}
}

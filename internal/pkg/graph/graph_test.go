// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGraph_Add(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// GIVEN
		graph := New[string]()

		// WHEN
		// A <-> B
		//    -> C
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
		require.ElementsMatch(t, []string{"B", "C"}, graph.Neighbors("A"))
		require.ElementsMatch(t, []string{"A"}, graph.Neighbors("B"))
	})
}

func TestGraph_InDegree(t *testing.T) {
	testCases := map[string]struct {
		graph *Graph[rune]

		wanted map[rune]int
	}{
		"should return 0 for nodes that don't exist in the graph": {
			graph: New[rune](),

			wanted: map[rune]int{
				'a': 0,
			},
		},
		"should return number of incoming edges for complex graph": {
			graph: func() *Graph[rune] {
				g := New[rune]()
				g.Add(Edge[rune]{'a', 'b'})
				g.Add(Edge[rune]{'b', 'a'})
				g.Add(Edge[rune]{'a', 'c'})
				g.Add(Edge[rune]{'b', 'c'})
				g.Add(Edge[rune]{'d', 'e'})
				return g
			}(),
			wanted: map[rune]int{
				'a': 1,
				'b': 1,
				'c': 2,
				'd': 0,
				'e': 1,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			for vtx, wanted := range tc.wanted {
				require.Equal(t, wanted, tc.graph.InDegree(vtx), "indegree for vertex %v does not match", vtx)
			}
		})
	}
}

func TestGraph_Remove(t *testing.T) {
	testCases := map[string]struct {
		graph *Graph[rune]

		wantedNeighbors map[rune][]rune
		wantedIndegrees map[rune]int
	}{
		"edge deletion should be idempotent": {
			graph: func() *Graph[rune] {
				g := New[rune]()
				g.Add(Edge[rune]{'a', 'b'})
				g.Add(Edge[rune]{'z', 'b'})
				g.Remove(Edge[rune]{'a', 'b'})
				g.Remove(Edge[rune]{'a', 'b'}) // Remove a second time.
				return g
			}(),

			wantedNeighbors: map[rune][]rune{
				'a': nil,
				'b': nil,
				'z': {'b'},
			},
			wantedIndegrees: map[rune]int{
				'a': 0,
				'z': 0,
				'b': 1,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			for vtx, wanted := range tc.wantedNeighbors {
				require.ElementsMatch(t, wanted, tc.graph.Neighbors(vtx), "neighbors for vertex %v do not match", vtx)
			}
			for vtx, wanted := range tc.wantedIndegrees {
				require.Equal(t, wanted, tc.graph.InDegree(vtx), "indegree for vertex %v does not match")
			}
		})
	}
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

func TestTopologicalOrder(t *testing.T) {
	testCases := map[string]struct {
		graph *Graph[string]

		wantedRanks     map[string]int
		wantedErrPrefix string
	}{
		"should return an error when a cycle is detected": {
			// frontend <-> backend
			graph: func() *Graph[string] {
				g := New("frontend", "backend")
				g.Add(Edge[string]{
					From: "frontend",
					To:   "backend",
				})
				g.Add(Edge[string]{
					From: "backend",
					To:   "frontend",
				})
				return g
			}(),
			wantedErrPrefix: "graph contains a cycle: ", // the cycle can appear in any order as map traversals are not deterministic, so only check the prefix.
		},
		"should return the ranks for a graph that looks like a bus": {
			// vpc -> lb -> api
			graph: func() *Graph[string] {
				g := New[string]()
				g.Add(Edge[string]{
					From: "vpc",
					To:   "lb",
				})
				g.Add(Edge[string]{
					From: "lb",
					To:   "api",
				})
				return g
			}(),

			wantedRanks: map[string]int{
				"api": 2,
				"lb":  1,
				"vpc": 0,
			},
		},
		"should return the ranks for a graph that looks like a tree": {
			graph: func() *Graph[string] {
				// vpc -> rds -> backend
				//     -> s3  -> api
				//            -> frontend
				g := New[string]()
				g.Add(Edge[string]{
					From: "vpc",
					To:   "rds",
				})
				g.Add(Edge[string]{
					From: "vpc",
					To:   "s3",
				})
				g.Add(Edge[string]{
					From: "rds",
					To:   "backend",
				})
				g.Add(Edge[string]{
					From: "s3",
					To:   "api",
				})
				g.Add(Edge[string]{
					From: "s3",
					To:   "frontend",
				})
				return g
			}(),

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
			graph: func() *Graph[string] {
				// warehouse -> orders   -> frontend
				//              payments ->
				g := New[string]()
				g.Add(Edge[string]{
					From: "payments",
					To:   "frontend",
				})
				g.Add(Edge[string]{
					From: "warehouse",
					To:   "orders",
				})
				g.Add(Edge[string]{
					From: "orders",
					To:   "frontend",
				})
				return g
			}(),

			wantedRanks: map[string]int{
				"frontend":  2,
				"orders":    1,
				"warehouse": 0,
				"payments":  0,
			},
		},
		"should find the longest path to a node": {
			graph: func() *Graph[string] {
				// a -> b -> c -> d -> f
				// a           -> e -> f
				g := New[string]()
				for _, edge := range []Edge[string]{{"a", "b"}, {"b", "c"}, {"c", "d"}, {"d", "f"}, {"a", "e"}, {"e", "f"}} {
					g.Add(edge)
				}
				return g
			}(),
			wantedRanks: map[string]int{
				"a": 0,
				"b": 1,
				"e": 1,
				"c": 2,
				"d": 3,
				"f": 4,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			topo, err := TopologicalOrder(tc.graph)

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

func buildGraphWithSingleParent() *LabeledGraph[string] {
	vertices := []string{"A", "B", "C", "D"}
	graph := NewLabeledGraph[string](vertices)
	graph.Add(Edge[string]{From: "D", To: "C"}) // D -> C
	graph.Add(Edge[string]{From: "C", To: "B"}) // C -> B
	graph.Add(Edge[string]{From: "B", To: "A"}) // B -> A
	return graph
}

func TestTraverseInDependencyOrder(t *testing.T) {
	t.Run("graph with single root vertex", func(t *testing.T) {
		graph := buildGraphWithSingleParent()
		var visited []string
		processFn := func(ctx context.Context, v string) error {
			visited = append(visited, v)
			return nil
		}
		err := graph.UpwardTraversal(context.Background(), processFn)
		require.NoError(t, err)
		expected := []string{"A", "B", "C", "D"}
		require.Equal(t, expected, visited)
	})
	t.Run("graph with multiple parents and boundary nodes", func(t *testing.T) {
		vertices := []string{"A", "B", "C", "D"}
		graph := NewLabeledGraph[string](vertices)
		graph.Add(Edge[string]{From: "A", To: "C"})
		graph.Add(Edge[string]{From: "A", To: "D"})
		graph.Add(Edge[string]{From: "B", To: "D"})
		vtxChan := make(chan string, 4)
		seen := make(map[string]int)
		done := make(chan struct{})
		go func() {
			for _, vtx := range vertices {
				seen[vtx]++
			}
			close(done)
		}()

		err := graph.DownwardTraversal(context.Background(), func(ctx context.Context, vtx string) error {
			vtxChan <- vtx
			return nil
		})
		require.NoError(t, err, "Error during iteration")
		close(vtxChan)
		<-done

		require.Len(t, seen, 4)
		for vtx, count := range seen {
			require.Equal(t, 1, count, "%s", vtx)
		}
	})
}

func TestTraverseInReverseDependencyOrder(t *testing.T) {
	t.Run("Graph with single root vertex", func(t *testing.T) {
		graph := buildGraphWithSingleParent()
		var visited []string
		processFn := func(ctx context.Context, v string) error {
			visited = append(visited, v)
			return nil
		}
		err := graph.DownwardTraversal(context.Background(), processFn)
		require.NoError(t, err)
		expected := []string{"D", "C", "B", "A"}
		require.Equal(t, expected, visited)
	})
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package graph provides functionality for directed graphs.
package graph

// vertexStatus denotes the visiting status of a vertex when running DFS in a graph.
type vertexStatus int

const (
	unvisited vertexStatus = iota + 1
	visiting
	visited
)

// Graph represents a directed graph.
type Graph[V comparable] struct {
	vertices map[V]neighbors[V]
}

// Edge represents one edge of a directed graph.
type Edge[V comparable] struct {
	From V
	To   V
}

type neighbors[V comparable] map[V]bool

// New initiates a new Graph.
func New[V comparable](vertices ...V) *Graph[V] {
	m := make(map[V]neighbors[V])
	for _, vertex := range vertices {
		m[vertex] = make(neighbors[V])
	}
	return &Graph[V]{
		vertices: m,
	}
}

// Add adds a connection between two vertices.
func (g *Graph[V]) Add(edge Edge[V]) {
	from, to := edge.From, edge.To
	if _, ok := g.vertices[from]; !ok {
		g.vertices[from] = make(neighbors[V])
	}
	if _, ok := g.vertices[to]; !ok {
		g.vertices[to] = make(neighbors[V])
	}
	g.vertices[from][to] = true
}

type findCycleTempVars[V comparable] struct {
	status     map[V]vertexStatus
	parents    map[V]V
	cycleStart V
	cycleEnd   V
}

// IsAcyclic checks if the graph is acyclic. If not, return the first detected cycle.
func (g *Graph[V]) IsAcyclic() ([]V, bool) {
	var cycle []V
	status := make(map[V]vertexStatus)
	for vertex := range g.vertices {
		status[vertex] = unvisited
	}
	temp := findCycleTempVars[V]{
		status:  status,
		parents: make(map[V]V),
	}
	// We will run a series of DFS in the graph. Initially all vertices are marked unvisited.
	// From each unvisited vertex, start the DFS, mark it visiting while entering and mark it visited on exit.
	// If DFS moves to a visiting vertex, then we have found a cycle. The cycle itself can be reconstructed using parent map.
	// See https://cp-algorithms.com/graph/finding-cycle.html
	for vertex := range g.vertices {
		if status[vertex] == unvisited && g.hasCycles(&temp, vertex) {
			for n := temp.cycleStart; n != temp.cycleEnd; n = temp.parents[n] {
				cycle = append(cycle, n)
			}
			cycle = append(cycle, temp.cycleEnd)
			return cycle, false
		}
	}
	return nil, true
}

// Roots return a slice of vertices with no incoming edges.
func (g *Graph[V]) Roots() []V {
	hasIncomingEdge := make(map[V]bool)
	for vtx := range g.vertices {
		hasIncomingEdge[vtx] = false
	}

	for from := range g.vertices {
		for to := range g.vertices[from] {
			hasIncomingEdge[to] = true
		}
	}

	var roots []V
	for vtx, yes := range hasIncomingEdge {
		if !yes {
			roots = append(roots, vtx)
		}
	}
	return roots
}

func (g *Graph[V]) hasCycles(temp *findCycleTempVars[V], currVertex V) bool {
	temp.status[currVertex] = visiting
	for vertex := range g.vertices[currVertex] {
		if temp.status[vertex] == unvisited {
			temp.parents[vertex] = currVertex
			if g.hasCycles(temp, vertex) {
				return true
			}
		} else if temp.status[vertex] == visiting {
			temp.cycleStart = currVertex
			temp.cycleEnd = vertex
			return true
		}
	}
	temp.status[currVertex] = visited
	return false
}

// LevelOrder ranks vertices in a breadth-first search manner.
type LevelOrder[V comparable] struct {
	marked map[V]bool
	ranks  map[V]int
}

// Rank returns the order of the vertex. The smallest order starts at 0.
// The second boolean return value is used to indicate whether the vertex exists in the graph.
func (bfs *LevelOrder[V]) Rank(vtx V) (int, bool) {
	r, ok := bfs.ranks[vtx]
	return r, ok
}

func (bfs *LevelOrder[V]) traverse(g *Graph[V], root V) {
	queue := []V{root}
	bfs.marked[root] = true
	bfs.ranks[root] = 0
	for {
		if len(queue) == 0 {
			return
		}
		var vtx V
		vtx, queue = queue[0], queue[1:]
		for neighbor := range g.vertices[vtx] {
			if bfs.marked[neighbor] {
				continue
			}
			bfs.marked[neighbor] = true
			if rank := bfs.ranks[vtx] + 1; rank > bfs.ranks[neighbor] { // Is the new rank higher than a previous traversal?
				bfs.ranks[neighbor] = rank
			}
			queue = append(queue, neighbor)
		}
	}
}

// LevelOrderTraversal determines whether the directed graph is acyclic, and if so then
// finds a level-order, or breadth-first search, ranking of the vertices.
// If the digraph contains a cycle, then an error is returned.
//
// An example graph and their ranks is shown below to illustrate:
// .
//├── a          rank: 0
//│   ├── c      rank: 1
//│   │   └── f  rank: 2
//│   └── d      rank: 1
//└── b          rank: 0
//    └── e      rank: 1
func LevelOrderTraversal[V comparable](digraph *Graph[V]) (*LevelOrder[V], error) {
	if vertices, isAcyclic := digraph.IsAcyclic(); !isAcyclic {
		return nil, &errCycle[V]{
			vertices,
		}
	}

	bfs := &LevelOrder[V]{
		ranks: make(map[V]int),
	}
	for _, root := range digraph.Roots() {
		bfs.marked = make(map[V]bool) // Reset all markings before each run.
		bfs.traverse(digraph, root)
	}
	return bfs, nil
}

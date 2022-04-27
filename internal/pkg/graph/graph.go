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
	vertices  map[V]neighbors[V] // Adjacency list for each vertex.
	inDegrees map[V]int          // Number of incoming edges for each vertex.
}

// Edge represents one edge of a directed graph.
type Edge[V comparable] struct {
	From V
	To   V
}

type neighbors[V comparable] map[V]bool

// New initiates a new Graph.
func New[V comparable](vertices ...V) *Graph[V] {
	adj := make(map[V]neighbors[V])
	inDegrees := make(map[V]int)
	for _, vertex := range vertices {
		adj[vertex] = make(neighbors[V])
		inDegrees[vertex] = 0
	}
	return &Graph[V]{
		vertices:  adj,
		inDegrees: inDegrees,
	}
}

// Neighbors returns the list of connected vertices from vtx.
func (g *Graph[V]) Neighbors(vtx V) []V {
	neighbors, ok := g.vertices[vtx]
	if !ok {
		return nil
	}
	arr := make([]V, len(neighbors))
	i := 0
	for neighbor := range neighbors {
		arr[i] = neighbor
		i += 1
	}
	return arr
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
	if _, ok := g.inDegrees[from]; !ok {
		g.inDegrees[from] = 0
	}
	if _, ok := g.inDegrees[to]; !ok {
		g.inDegrees[to] = 0
	}

	g.vertices[from][to] = true
	g.inDegrees[to] += 1
}

// InDegree returns the number of incoming edges to vtx.
func (g *Graph[V]) InDegree(vtx V) int {
	return g.inDegrees[vtx]
}

// Remove deletes a connection between two vertices.
func (g *Graph[V]) Remove(edge Edge[V]) {
	if _, ok := g.vertices[edge.From][edge.To]; !ok {
		return
	}
	delete(g.vertices[edge.From], edge.To)
	g.inDegrees[edge.To] -= 1
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

// Roots returns a slice of vertices with no incoming edges.
func (g *Graph[V]) Roots() []V {
	var roots []V
	for vtx, degree := range g.inDegrees {
		if degree == 0 {
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

// TopologicalSorter ranks vertices using Kahn's algorithm: https://en.wikipedia.org/wiki/Topological_sorting#Kahn's_algorithm
// However, if two vertices can be scheduled in parallel then the same rank is returned.
type TopologicalSorter[V comparable] struct {
	ranks map[V]int
}

// Rank returns the order of the vertex. The smallest order starts at 0.
// The second boolean return value is used to indicate whether the vertex exists in the graph.
func (alg *TopologicalSorter[V]) Rank(vtx V) (int, bool) {
	r, ok := alg.ranks[vtx]
	return r, ok
}

func (alg *TopologicalSorter[V]) traverse(g *Graph[V]) {
	roots := g.Roots()
	for _, root := range roots {
		alg.ranks[root] = 0 // Explicitly set to 0 so that `_, ok := alg.ranks[vtx]` returns true instead of false.
	}
	for len(roots) > 0 {
		var vtx V
		vtx, roots = roots[0], roots[1:]
		for _, neighbor := range g.Neighbors(vtx) {
			if new, old := alg.ranks[vtx]+1, alg.ranks[neighbor]; new > old {
				alg.ranks[neighbor] = new
			}
			g.Remove(Edge[V]{vtx, neighbor})
			if g.InDegree(neighbor) == 0 {
				roots = append(roots, neighbor)
			}
		}
	}
}

// TopologicalOrder determines whether the directed graph is acyclic, and if so then
// finds a topological-order, or a linear order, of the vertices.
// Note that this function will modify the original graph.
//
// If there is an edge from vertex V to U, then V must happen before U and results in rank of V < rank of U.
// When there are ties (two vertices can be scheduled in parallel), the vertices are given the same rank.
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
func TopologicalOrder[V comparable](digraph *Graph[V]) (*TopologicalSorter[V], error) {
	if vertices, isAcyclic := digraph.IsAcyclic(); !isAcyclic {
		return nil, &errCycle[V]{
			vertices,
		}
	}

	topo := &TopologicalSorter[V]{
		ranks: make(map[V]int),
	}
	topo.traverse(digraph)
	return topo, nil
}

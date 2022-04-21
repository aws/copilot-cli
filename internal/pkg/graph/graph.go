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
	// Add origin vertex if doesn't exist.
	if _, ok := g.vertices[from]; !ok {
		g.vertices[from] = make(neighbors[V])
	}
	// Add edge.
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

// TopologicalSorter ranks vertices in a graph using topological sort.
type TopologicalSorter[V comparable] struct {
	ranks map[V]int
}

// Rank returns the order of the vertex. The smallest order starts at 0.
// If the vertex does not exist in the graph, then returns an error.
func (s *TopologicalSorter[V]) Rank(vtx V) (int, error) {
	// TODO(efekarakus): Implement me.
	return 0, nil
}

// TopologicalSort determines whether the directed graph is acyclic, and if so then finds a topological order.
// If the digraph contains a cycle, then an error is returned.
func TopologicalSort[V comparable](digraph *Graph[V]) (*TopologicalSorter[V], error) {
	// TODO(efekarakus): Implement me.
	return nil, nil
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package graph provides functionality for directed graphs.
package graph

import (
	"context"
	"sync"

	"golang.org/x/sync/errgroup"
)

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
	status    map[V]string       // status of each vertex.
	lock      sync.Mutex         // lock used to mutate graph data.
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

func NewWithStatus[V comparable](status string, vertices ...V) *Graph[V] {
	g := New[V](vertices...)
	g.status = make(map[V]string)
	for vertex := range g.vertices {
		g.status[vertex] = status
	}
	return g
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

// updateStatus updates the status of a vertex.
func (g *Graph[V]) updateStatus(vertex V, status string) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.status[vertex] = status
}

// getStatus gets the status of a vertex.
func (g *Graph[V]) getStatus(vertex V) string {
	g.lock.Lock()
	defer g.lock.Unlock()
	return g.status[vertex]
}

// getLeaves returns the leaves of a given vertex.
func (g *Graph[V]) leaves() []V {
	g.lock.Lock()
	defer g.lock.Unlock()
	var leaves []V
	for vtx := range g.vertices {
		if len(g.vertices[vtx]) == 0 {
			leaves = append(leaves, vtx)
		}
	}
	return leaves
}

// getParents returns the parent vertices (incoming edges) of vertex.
func (g *Graph[V]) parents(vtx V) []V {
	g.lock.Lock()
	defer g.lock.Unlock()
	var parents []V
	for v, neighbors := range g.vertices {
		if neighbors[vtx] {
			parents = append(parents, v)
		}
	}
	return parents
}

// getChildren returns the child vertices (outgoing edges) of vertex.
func (g *Graph[V]) children(vtx V) []V {
	g.lock.Lock()
	defer g.lock.Unlock()
	return g.Neighbors(vtx)
}

// filterParents filters parents based on the vertex status.
func (g *Graph[V]) filterParents(vtx V, status string) []V {
	parents := g.parents(vtx)
	var filtered []V
	for _, parent := range parents {
		if g.getStatus(parent) == status {
			filtered = append(filtered, parent)
		}
	}
	return filtered
}

// filterChildren filters children based on the vertex status.
func (g *Graph[V]) filterChildren(vtx V, status string) []V {
	children := g.children(vtx)
	var filtered []V
	for _, child := range children {
		if g.getStatus(child) == status {
			filtered = append(filtered, child)
		}
	}
	return filtered
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
// ├── a          rank: 0
// │   ├── c      rank: 1
// │   │   └── f  rank: 2
// │   └── d      rank: 1
// └── b          rank: 0
//
//	└── e      rank: 1
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

/*
UpwardTraversal performs an upward traversal on the graph starting from leaves (nodes with no children)
and moving towards root nodes (nodes with children).
It applies the specified process function to each vertex in the graph, skipping vertices with the
"adjacentVertexSkipStatus" status, and continuing traversal until reaching vertices with the "requiredVertexStatus" status.
The traversal is concurrent and may process vertices in parallel.
Returns an error if the traversal encounters any issues, or nil if successful.
*/
func (g *Graph[V]) UpwardTraversal(ctx context.Context, processVertexFunc func(context.Context, V) error, adjacentVertexSkipStatus, requiredVertexStatus string) error {
	traversal := &graphTraversal[V]{
		mu:                       sync.Mutex{},
		seen:                     make(map[V]struct{}),
		findBoundaryVertices:     func(g *Graph[V]) []V { return g.leaves() },
		findAdjacentVertices:     func(g *Graph[V], v V) []V { return g.parents(v) },
		filterVerticesByStatus:   func(g *Graph[V], v V, status string) []V { return g.filterChildren(v, status) },
		requiredVertexStatus:     requiredVertexStatus,
		adjacentVertexSkipStatus: adjacentVertexSkipStatus,
		processVertex:            processVertexFunc,
	}
	return traversal.execute(ctx, g)
}

/*
DownwardTraversal performs a downward traversal on the graph starting from root nodes (nodes with no parents)
and moving towards leaf nodes (nodes with parents). It applies the specified process function to each
vertex in the graph, skipping vertices with the "adjacentVertexSkipStatus" status, and continuing traversal
until reaching vertices with the "requiredVertexStatus" status.
The traversal is concurrent and may process vertices in parallel.
Returns an error if the traversal encounters any issues.
*/
func (g *Graph[V]) DownwardTraversal(ctx context.Context, processVertexFunc func(context.Context, V) error, adjacentVertexSkipStatus, requiredVertexStatus string) error {
	traversal := &graphTraversal[V]{
		mu:                       sync.Mutex{},
		seen:                     make(map[V]struct{}),
		findBoundaryVertices:     func(g *Graph[V]) []V { return g.Roots() },
		findAdjacentVertices:     func(g *Graph[V], v V) []V { return g.children(v) },
		filterVerticesByStatus:   func(g *Graph[V], v V, status string) []V { return g.filterParents(v, status) },
		requiredVertexStatus:     requiredVertexStatus,
		adjacentVertexSkipStatus: adjacentVertexSkipStatus,
		processVertex:            processVertexFunc,
	}
	return traversal.execute(ctx, g)
}

type graphTraversal[V comparable] struct {
	mu                       sync.Mutex
	seen                     map[V]struct{}
	findBoundaryVertices     func(*Graph[V]) []V
	findAdjacentVertices     func(*Graph[V], V) []V
	filterVerticesByStatus   func(*Graph[V], V, string) []V
	requiredVertexStatus     string
	adjacentVertexSkipStatus string
	processVertex            func(context.Context, V) error
}

func (t *graphTraversal[V]) execute(ctx context.Context, graph *Graph[V]) error {
	vertexCount := len(graph.vertices)
	if vertexCount == 0 {
		return nil
	}
	eg, ctx := errgroup.WithContext(ctx)
	vertexCh := make(chan V, vertexCount)
	defer close(vertexCh)

	processVertices := func(ctx context.Context, graph *Graph[V], eg *errgroup.Group, vertices []V, vertexCh chan V) {
		for _, vertex := range vertices {
			vertex := vertex
			// Delay processing this vertex if any of its dependent vertices are yet to be processed.
			if len(t.filterVerticesByStatus(graph, vertex, t.adjacentVertexSkipStatus)) != 0 {
				continue
			}
			if !t.markAsSeen(vertex) {
				// Skip this vertex if it's already been processed by another routine.
				continue
			}
			eg.Go(func() error {
				if err := t.processVertex(ctx, vertex); err != nil {
					return err
				}
				// Assign new status to the vertex upon successful processing.
				graph.updateStatus(vertex, t.requiredVertexStatus)
				vertexCh <- vertex
				return nil
			})
		}
	}

	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case vertex := <-vertexCh:
				vertexCount--
				if vertexCount == 0 {
					return nil
				}
				processVertices(ctx, graph, eg, t.findAdjacentVertices(graph, vertex), vertexCh)
			}
		}
	})
	processVertices(ctx, graph, eg, t.findBoundaryVertices(graph), vertexCh)
	return eg.Wait()
}

func (t *graphTraversal[V]) markAsSeen(vertex V) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, seen := t.seen[vertex]; seen {
		return false
	}
	t.seen[vertex] = struct{}{}
	return true
}

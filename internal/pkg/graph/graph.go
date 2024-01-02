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

// LabeledGraph extends a generic Graph by associating a label (or status) with each vertex.
// It is concurrency-safe, utilizing a mutex lock for synchronized access.
type LabeledGraph[V comparable] struct {
	*Graph[V]
	status map[V]vertexStatus
	lock   sync.Mutex
}

// NewLabeledGraph initializes a LabeledGraph with specified vertices and set the status of each vertex to unvisited.
func NewLabeledGraph[V comparable](vertices []V) *LabeledGraph[V] {
	lg := &LabeledGraph[V]{
		Graph:  New(vertices...),
		status: make(map[V]vertexStatus),
		lock:   sync.Mutex{},
	}
	for _, vertex := range vertices {
		lg.status[vertex] = unvisited
	}
	return lg
}

// updateStatus updates the status of a vertex.
func (lg *LabeledGraph[V]) updateStatus(vertex V, status vertexStatus) {
	lg.lock.Lock()
	defer lg.lock.Unlock()
	lg.status[vertex] = status
}

// getStatus gets the status of a vertex.
func (lg *LabeledGraph[V]) getStatus(vertex V) vertexStatus {
	lg.lock.Lock()
	defer lg.lock.Unlock()
	return lg.status[vertex]
}

// getLeaves returns the leaves of a given vertex.
func (lg *LabeledGraph[V]) leaves() []V {
	lg.lock.Lock()
	defer lg.lock.Unlock()
	var leaves []V
	for vtx := range lg.vertices {
		if len(lg.vertices[vtx]) == 0 {
			leaves = append(leaves, vtx)
		}
	}
	return leaves
}

// getParents returns the parent vertices (incoming edges) of vertex.
func (lg *LabeledGraph[V]) parents(vtx V) []V {
	lg.lock.Lock()
	defer lg.lock.Unlock()
	var parents []V
	for v, neighbors := range lg.vertices {
		if neighbors[vtx] {
			parents = append(parents, v)
		}
	}
	return parents
}

// getChildren returns the child vertices (outgoing edges) of vertex.
func (lg *LabeledGraph[V]) children(vtx V) []V {
	lg.lock.Lock()
	defer lg.lock.Unlock()
	return lg.Neighbors(vtx)
}

// filterParents filters parents based on the vertex status.
func (lg *LabeledGraph[V]) filterParents(vtx V, status vertexStatus) []V {
	parents := lg.parents(vtx)
	var filtered []V
	for _, parent := range parents {
		if lg.getStatus(parent) == status {
			filtered = append(filtered, parent)
		}
	}
	return filtered
}

// filterChildren filters children based on the vertex status.
func (lg *LabeledGraph[V]) filterChildren(vtx V, status vertexStatus) []V {
	children := lg.children(vtx)
	var filtered []V
	for _, child := range children {
		if lg.getStatus(child) == status {
			filtered = append(filtered, child)
		}
	}
	return filtered
}

/*
UpwardTraversal performs a traversal from leaf nodes (with no children) to root nodes (with children).
It processes each vertex using processVertexFunc, and skips processing for vertices with specific statuses.
The traversal is concurrent, handling vertices in parallel, and returns an error if any issue occurs.
*/
func (lg *LabeledGraph[V]) UpwardTraversal(ctx context.Context, processVertexFunc func(context.Context, V) error) error {
	traversal := &graphTraversal[V]{
		findStartVertices:              func(lg *LabeledGraph[V]) []V { return lg.leaves() },
		findNextVertices:               func(lg *LabeledGraph[V], v V) []V { return lg.parents(v) },
		filterPreviousVerticesByStatus: func(g *LabeledGraph[V], v V, status vertexStatus) []V { return g.filterChildren(v, status) },
		processVertex:                  processVertexFunc,
	}
	return traversal.execute(ctx, lg)
}

/*
DownwardTraversal performs a traversal from root nodes (with no parents) to leaf nodes (with parents).
It applies processVertexFunc to each vertex, skipping those with specified statuses.
It conducts concurrent processing of vertices and returns an error for any encountered issues.
*/
func (lg *LabeledGraph[V]) DownwardTraversal(ctx context.Context, processVertexFunc func(context.Context, V) error) error {
	traversal := &graphTraversal[V]{
		findStartVertices:              func(lg *LabeledGraph[V]) []V { return lg.Roots() },
		findNextVertices:               func(lg *LabeledGraph[V], v V) []V { return lg.children(v) },
		filterPreviousVerticesByStatus: func(lg *LabeledGraph[V], v V, status vertexStatus) []V { return lg.filterParents(v, status) },
		processVertex:                  processVertexFunc,
	}
	return traversal.execute(ctx, lg)
}

type graphTraversal[V comparable] struct {
	findStartVertices              func(*LabeledGraph[V]) []V
	findNextVertices               func(*LabeledGraph[V], V) []V
	filterPreviousVerticesByStatus func(*LabeledGraph[V], V, vertexStatus) []V
	processVertex                  func(context.Context, V) error
}

func (t *graphTraversal[V]) execute(ctx context.Context, lg *LabeledGraph[V]) error {

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	vertexCount := len(lg.vertices)
	if vertexCount == 0 {
		return nil
	}
	eg, ctx := errgroup.WithContext(ctx)
	vertexCh := make(chan V, vertexCount)
	defer close(vertexCh)

	processVertices := func(ctx context.Context, graph *LabeledGraph[V], eg *errgroup.Group, vertices []V, vertexCh chan V) {
		for _, vertex := range vertices {
			vertex := vertex
			// If any of the vertices that should be visited before this vertex are yet to be processed, we delay processing it.
			if len(t.filterPreviousVerticesByStatus(graph, vertex, unvisited)) != 0 ||
				len(t.filterPreviousVerticesByStatus(graph, vertex, visiting)) != 0 {
				continue
			}
			// Check if the vertex is already visited or being visited
			if graph.getStatus(vertex) != unvisited {
				continue
			}
			// Mark the vertex as visiting
			graph.updateStatus(vertex, visiting)
			eg.Go(func() error {
				if err := t.processVertex(ctx, vertex); err != nil {
					return err
				}
				// Assign new status to the vertex upon successful processing.
				graph.updateStatus(vertex, visited)
				vertexCh <- vertex
				return nil
			})
		}
	}

	eg.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case vertex := <-vertexCh:
				vertexCount--
				if vertexCount == 0 {
					return nil
				}
				processVertices(ctx, lg, eg, t.findNextVertices(lg, vertex), vertexCh)
			}
		}
	})
	processVertices(ctx, lg, eg, t.findStartVertices(lg), vertexCh)
	return eg.Wait()
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package graph provides functionality for graphes.
package graph

// Graph represents a directed graph.
type Graph struct {
	Nodes map[string][]string
	Cycle []string
}

// NewGraph initiates a new Graph.
func NewGraph() *Graph {
	return &Graph{
		Nodes: make(map[string][]string),
	}
}

// Add adds a connection between two Nodes.
func (g *Graph) Add(fromNode, toNode string) {
	hasNode := false
	// Add origin node if doesn't exist.
	if _, ok := g.Nodes[fromNode]; !ok {
		g.Nodes[fromNode] = []string{}
	}
	// Check if edge exists between from and to Nodes.
	for _, node := range g.Nodes[fromNode] {
		if node == toNode {
			hasNode = true
		}
	}
	// Add edge if not there already.
	if !hasNode {
		g.Nodes[fromNode] = append(g.Nodes[fromNode], toNode)
	}
}

// IsAcyclic returns if the graph is acyclic.
func (g *Graph) IsAcyclic() bool {
	used := make(map[string]bool)
	path := make(map[string]bool)
	for node := range g.Nodes {
		if !used[node] && g.hasCycles(used, path, node) {
			return false
		}
	}
	return true
}

func (g *Graph) hasCycles(used map[string]bool, path map[string]bool, currNode string) bool {
	used[currNode] = true
	path[currNode] = true
	for _, node := range g.Nodes[currNode] {
		if !used[node] && g.hasCycles(used, path, node) {
			g.Cycle = append(g.Cycle, node)
			return true
		} else if path[node] {
			g.Cycle = append(g.Cycle, node)
			return true
		}
	}
	path[currNode] = false
	return false
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package graph provides functionality for graphes.
package graph

type nodeColor int

const (
	white nodeColor = iota + 1
	gray
	black
)

// Graph represents a directed graph.
type Graph struct {
	Nodes map[string][]string
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

type findCycleTempVars struct {
	color      map[string]nodeColor
	nodeParent map[string]string
	cycleStart string
	cycleEnd   string
}

// IsAcyclic checks if the graph is acyclic. If not, return the first detected cycle.
func (g *Graph) IsAcyclic() (bool, []string) {
	var cycle []string
	color := make(map[string]nodeColor)
	for node := range g.Nodes {
		color[node] = white
	}
	temp := findCycleTempVars{
		color:      color,
		nodeParent: make(map[string]string),
	}
	// We will run a series of DFS in the graph. Initially all vertices are colored white (unvisited).
	// From each white node, start the DFS, mark it gray while entering and mark it black on exit.
	// If DFS moves to a gray node, then we have found a cycle. The cycle itself can be reconstructed using parent map.
	// See https://cp-algorithms.com/graph/finding-cycle.html
	for node := range g.Nodes {
		if color[node] == white && g.hasCycles(&temp, node) {
			for n := temp.cycleStart; n != temp.cycleEnd; n = temp.nodeParent[n] {
				cycle = append(cycle, n)
			}
			cycle = append(cycle, temp.cycleEnd)
			return false, cycle
		}
	}
	return true, nil
}

func (g *Graph) hasCycles(temp *findCycleTempVars, currNode string) bool {
	temp.color[currNode] = gray
	for _, node := range g.Nodes[currNode] {
		if temp.color[node] == white {
			temp.nodeParent[node] = currNode
			if g.hasCycles(temp, node) {
				return true
			}
		} else if temp.color[node] == gray {
			temp.cycleStart = currNode
			temp.cycleEnd = node
			return true
		}
	}
	temp.color[currNode] = black
	return false
}

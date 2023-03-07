// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package diff provides functionalities to compare two YAML documents.
package diff

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Node represents a segment on a difference between two YAML documents.
type Node struct {
	key      string
	children map[string]*Node // A list of non-empty pointers to the children nodes.

	oldValue *yaml.Node // Only populated for a leaf node (i.e. that has no child node).
	newValue *yaml.Node // Only populated for a leaf node (i.e. that has no child node).
}

// String returns the string representation of the tree stemmed from the diffNode n.
func (n *Node) String() string {
	return ""
}

// From is the YAML document that another YAML document is compared against.
type From []byte

// Parse constructs a diff tree that represent the differences of a YAML document against the From document.
func (from From) Parse(to []byte) (*Node, error) {
	var toNode, fromNode yaml.Node
	if err := yaml.Unmarshal(to, &toNode); err != nil {
		return nil, fmt.Errorf("unmarshal current template: %w", err)
	}
	if err := yaml.Unmarshal(from, &fromNode); err != nil {
		return nil, fmt.Errorf("unmarshal old template: %w", err)
	}

	return parse(&fromNode, &toNode, "")

}

func parse(from, to *yaml.Node, key string) (*Node, error) {
	if to == nil || from == nil {
		return &Node{
			key:      key,
			oldValue: from,
			newValue: to,
		}, nil
	}
	if isYAMLLeaf(to) && isYAMLLeaf(from) {
		if to.Value == from.Value {
			return nil, nil
		}
		return &Node{
			key:      key,
			newValue: to,
			oldValue: from,
		}, nil
	}
	var children map[string]*Node
	var err error
	switch {
	case to.Kind == yaml.SequenceNode && from.Kind == yaml.SequenceNode:
		children, err = parseSequence(from, to)
	case to.Kind == yaml.DocumentNode && from.Kind == yaml.DocumentNode:
		fallthrough
	case to.Kind == yaml.MappingNode && from.Kind == yaml.MappingNode:
		children, err = parseMap(from, to)
	default:
		return nil, fmt.Errorf("unknown combination of node kinds: %v, %v", to.Kind, from.Kind)
	}
	if err != nil {
		return nil, fmt.Errorf("parse YAML content with key %s: %w", key, err)
	}
	if len(children) == 0 {
		return nil, nil
	}
	return &Node{
		key:      key,
		children: children,
	}, nil
}

func isYAMLLeaf(node *yaml.Node) bool {
	return len(node.Content) == 0
}

func parseSequence(from, to *yaml.Node) (map[string]*Node, error) {
	toSeq, fromSeq := make([]yaml.Node, len(to.Content)), make([]yaml.Node, len(from.Content))
	if err := to.Decode(&toSeq); err != nil {
		return nil, err
	}
	if err := from.Decode(&fromSeq); err != nil {
		return nil, err
	}
	seen, err := compare(fromSeq, toSeq)
	if err != nil {
		return nil, err
	}
	var index int
	children := make(map[string]*Node)
	for _, occurrence := range seen {
		if occurrence.inTo == occurrence.inFrom {
			continue
		}
		for i := 0; i < occurrence.inFrom-occurrence.inTo; i++ {
			children[strconv.Itoa(index)] = &Node{
				key:      strconv.Itoa(index),
				oldValue: &(occurrence.node),
			}
			index++
		}
		for i := 0; i < occurrence.inTo-occurrence.inFrom; i++ {
			children[strconv.Itoa(index)] = &Node{
				key:      strconv.Itoa(index),
				newValue: &(occurrence.node),
			}
			index++
		}
	}
	return children, nil
}

type nodeOccurrence struct {
	node   yaml.Node
	inTo   int
	inFrom int
}

func compare(from, to []yaml.Node) (map[string]*nodeOccurrence, error) {
	seen := make(map[string]*nodeOccurrence)
	for _, node := range from {
		b, err := yaml.Marshal(node)
		if err != nil {
			return nil, err
		}
		content := string(b)
		if v, ok := seen[content]; ok {
			v.inFrom++
			continue
		}
		seen[content] = &nodeOccurrence{
			node:   node,
			inFrom: 1,
		}
	}
	for _, node := range to {
		b, err := yaml.Marshal(node)
		if err != nil {
			return nil, err
		}
		content := string(b)
		if v, ok := seen[content]; ok {
			v.inTo++
			continue
		}
		seen[content] = &nodeOccurrence{
			node: node,
			inTo: 1,
		}
	}
	return seen, nil
}

func parseMap(from, to *yaml.Node) (map[string]*Node, error) {
	currMap, oldMap := make(map[string]yaml.Node), make(map[string]yaml.Node)
	if err := to.Decode(currMap); err != nil {
		return nil, err
	}
	if err := from.Decode(oldMap); err != nil {
		return nil, err
	}
	children := make(map[string]*Node)
	for k := range unionOfKeys(currMap, oldMap) {
		var currV, oldV *yaml.Node
		if v, ok := oldMap[k]; ok {
			oldV = &v
		}
		if v, ok := currMap[k]; ok {
			currV = &v
		}
		kDiff, err := parse(oldV, currV, k)
		if err != nil {
			return nil, err
		}
		if kDiff != nil {
			children[k] = kDiff
		}
	}
	return children, nil
}

func unionOfKeys[T any](a, b map[string]T) map[string]struct{} {
	exists, keys := struct{}{}, make(map[string]struct{})
	for k := range a {
		keys[k] = exists
	}
	for k := range b {
		keys[k] = exists
	}
	return keys
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package diff provides functionalities to compare two YAML documents.
package diff

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Node represents a segment on a difference between two YAML documents.
type Node struct {
	key      string
	children map[string]*Node // A list of non-empty pointers to the children nodes.

	oldValue *yaml.Node // Only populated for a leaf node (i.e. that has no child node).
	newValue *yaml.Node // Only populated for a leaf node (i.e. that has no child node).
}

// Parse constructs a diff tree that represent the differences between two YAML documents.
func Parse(curr, old []byte) (*Node, error) {
	var currNode, oldNode yaml.Node
	if err := yaml.Unmarshal(curr, &currNode); err != nil {
		return nil, fmt.Errorf("unmarshal current template: %w", err)
	}
	if err := yaml.Unmarshal(old, &oldNode); err != nil {
		return nil, fmt.Errorf("unmarshal old template: %w", err)
	}
	return parse(&currNode, &oldNode, "")

}

// String returns the string representation of the tree stemmed from the diffNode n.
func (n *Node) String() string {
	return ""
}

func parse(curr, old *yaml.Node, key string) (*Node, error) {
	if curr == nil || old == nil {
		return &Node{
			key:      key,
			oldValue: old,
			newValue: curr,
		}, nil
	}
	if isYAMLLeaf(curr) && isYAMLLeaf(old) {
		if curr.Value == old.Value {
			return nil, nil
		}
		return &Node{
			key:      key,
			newValue: curr,
			oldValue: old,
		}, nil
	}
	var children map[string]*Node
	var err error
	switch {
	case curr.Kind == yaml.SequenceNode && old.Kind == yaml.SequenceNode:
		children, err = parseSequence(curr, old)
	case curr.Kind == yaml.DocumentNode && old.Kind == yaml.DocumentNode:
		fallthrough
	case curr.Kind == yaml.MappingNode && old.Kind == yaml.MappingNode:
		children, err = parseMap(curr, old)
	default:
		return nil, fmt.Errorf("unknown combination of node kinds: %v, %v", curr.Kind, old.Kind)
	}
	if err != nil {
		return nil, err
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

func parseSequence(curr, old *yaml.Node) (map[string]*Node, error) {
	return nil, nil
}

func parseMap(curr, old *yaml.Node) (map[string]*Node, error) {
	currMap, oldMap := make(map[string]yaml.Node), make(map[string]yaml.Node)
	if err := curr.Decode(currMap); err != nil {
		return nil, err
	}
	if err := old.Decode(oldMap); err != nil {
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
		kDiff, err := parse(currV, oldV, k)
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

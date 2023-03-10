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
	// Handle base cases.
	if to == nil || from == nil || to.Kind != from.Kind {
		return &Node{
			key:      key,
			newValue: to,
			oldValue: from,
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
	var fromSeq, toSeq []yaml.Node
	if err := to.Decode(&toSeq); err != nil {
		return nil, err
	}
	if err := from.Decode(&fromSeq); err != nil {
		return nil, err
	}
	cachedDiff, cachedErr := make(map[string]*Node), make(map[string]error)
	lcsIdxInFrom, lcsIdxInTo := longestCommonSubsequence(fromSeq, toSeq, func(inA, inB int) bool {
		diff, err := parse(&(fromSeq[inA]), &(toSeq[inB]), "")
		if diff != nil { // NOTE: cache the diff only if a modification could have happened at this position.
			cachedDiff[cachedKey(inA, inB)] = diff
			cachedErr[cachedKey(inA, inB)] = err
		}
		return err == nil && diff == nil // NOTE: we swallow the error for now.
	})
	childKey := seqChildKeyFunc()
	children := make(map[string]*Node)
	var f, t, i int
	for {
		if i >= len(lcsIdxInFrom) {
			break
		}
		lcsF, lcsT := lcsIdxInFrom[i], lcsIdxInTo[i]
		switch {
		case f == lcsF && t == lcsT: // Match.
			f++
			t++
			i++
			// TODO(lou1415926): (x unchanged items)
		case f != lcsF && t != lcsT: // Modification.
			// TODO(lou1415926): handle list of maps modification
			diff, err := cachedDiff[cachedKey(f, t)], cachedErr[cachedKey(f, t)]
			if err != nil {
				return nil, err
			}
			children[childKey()] = diff
			f++
			t++
		case f != lcsF: // Deletion.
			children[childKey()] = &Node{oldValue: &(fromSeq[f])}
			f++
		case t != lcsT: // Insertion.
			children[childKey()] = &Node{newValue: &(toSeq[t])}
			t++
		}
	}
	for {
		if f >= len(fromSeq) && t >= len(toSeq) {
			break
		}
		switch {
		case f >= len(fromSeq) && t >= len(toSeq):
			break
		case f < len(fromSeq) && t < len(toSeq): // Modification.
			diff, err := parse(&(fromSeq[f]), &(toSeq[t]), "")
			if err != nil {
				return nil, err
			}
			children[childKey()] = diff
		case f < len(fromSeq): // Deletion.
			children[childKey()] = &Node{oldValue: &(fromSeq[f])}
		case t < len(toSeq): // Insertion.
			children[childKey()] = &Node{newValue: &(toSeq[t])}
		}
		f++
		t++
	}
	return children, nil
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

func cachedKey(inFrom, inTo int) string {
	return fmt.Sprintf("%d,%d", inFrom, inTo)
}

// TODO(lou1415926): use a more meaningful key for a seq child.
func seqChildKeyFunc() func() string {
	idx := -1
	return func() string {
		idx++
		return strconv.Itoa(idx)
	}
}

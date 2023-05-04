// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package diff provides functionalities to compare two YAML documents.
package diff

import (
	"fmt"
	"io"
	"sort"

	"gopkg.in/yaml.v3"
)

// Tree represents a difference tree between two YAML documents.
type Tree struct {
	root diffNode
}

func (t Tree) Write(w io.Writer) error {
	tw := &treeWriter{t, w}
	return tw.write()
}

// diffNode is the interface to represents the difference between two *yaml.Node.
type diffNode interface {
	key() string
	newYAML() *yaml.Node
	oldYAML() *yaml.Node
	children() []diffNode
}

// keyNode is a concrete implementation of a diffNode.
type keyNode struct {
	keyValue   string
	childNodes []diffNode // A list of non-empty pointers to the children nodes.

	oldV *yaml.Node // Only populated for a leaf node (i.e. that has no child node).
	newV *yaml.Node // Only populated for a leaf node (i.e. that has no child node).
}

func (n *keyNode) key() string {
	return n.keyValue
}

func (n *keyNode) newYAML() *yaml.Node {
	return n.newV
}

func (n *keyNode) oldYAML() *yaml.Node {
	return n.oldV
}

func (n *keyNode) children() []diffNode {
	return n.childNodes
}

type unchangedNode struct {
	count int
}

func (n *unchangedNode) children() []diffNode {
	return nil
}

func (n *unchangedNode) key() string {
	return ""
}

func (n *unchangedNode) newYAML() *yaml.Node {
	return nil
}

func (n *unchangedNode) oldYAML() *yaml.Node {
	return nil
}

func (n *unchangedNode) unchangedCount() int {
	return n.count
}

type seqItemNode struct {
	keyNode
}

// From is the YAML document that another YAML document is compared against.
type From []byte

// ParseWithCFNOverriders constructs a diff tree that represent the differences of a YAML document against the From document with 
// overriders designed for CFN documents, including:
// 1. An ignorer that ignores diffs under "Metadata.Manifest".
// 2. An overrider that is able to compare intrinsic functions with full/short form correctly.
func (from From) ParseWithCFNOverriders(to []byte) (Tree, error) {
	return from.Parse(to,
		&ignorer{
			curr: &ignoreSegment{
				key: "Metadata",
				next: &ignoreSegment{
					key: "Manifest",
				},
			},
		},
		&getAttConverter{},
		&intrinsicFuncMapTagConverter{})
}

// Parse constructs a diff tree that represent the differences of a YAML document against the From document.
func (from From) Parse(to []byte, overriders ...overrider) (Tree, error) {
	var toNode, fromNode yaml.Node
	if err := yaml.Unmarshal(to, &toNode); err != nil {
		return Tree{}, fmt.Errorf("unmarshal current template: %w", err)
	}
	if err := yaml.Unmarshal(from, &fromNode); err != nil {
		return Tree{}, fmt.Errorf("unmarshal old template: %w", err)
	}
	var root diffNode
	var err error
	switch {
	// NOTE: If Kind is 0, it means the document is empty and nothing is unmarshalled.
	case fromNode.Kind == 0 && toNode.Kind == 0:
		return Tree{}, nil
	case fromNode.Kind == 0:
		root, err = parse(nil, &toNode, "", overriders...)
	case toNode.Kind == 0:
		root, err = parse(&fromNode, nil, "", overriders...)
	default:
		root, err = parse(&fromNode, &toNode, "", overriders...)
	}
	if err != nil {
		return Tree{}, err
	}
	if root == nil {
		return Tree{}, nil
	}
	return Tree{
		root: root,
	}, nil
}

func parse(from, to *yaml.Node, key string, overriders ...overrider) (diffNode, error) {
	for _, overrider := range overriders {
		if overrider.match(from, to, key, overrider) {
			return overrider.parse(from, to, key, overrider)
		}
	}
	// Handle base cases.
	if to == nil || from == nil || to.Kind != from.Kind {
		return &keyNode{
			keyValue: key,
			newV:     to,
			oldV:     from,
		}, nil
	}
	if isYAMLLeaf(to) && isYAMLLeaf(from) {
		if to.Value == from.Value {
			return nil, nil
		}
		return &keyNode{
			keyValue: key,
			newV:     to,
			oldV:     from,
		}, nil
	}

	var children []diffNode
	var err error
	switch {
	case to.Kind == yaml.SequenceNode && from.Kind == yaml.SequenceNode:
		children, err = parseSequence(from, to, overriders...)
	case to.Kind == yaml.DocumentNode && from.Kind == yaml.DocumentNode:
		fallthrough
	case to.Kind == yaml.MappingNode && from.Kind == yaml.MappingNode:
		children, err = parseMap(from, to, overriders...)
	default:
		return nil, fmt.Errorf("unknown combination of node kinds: %v, %v", to.Kind, from.Kind)
	}
	if err != nil {
		return nil, fmt.Errorf("parse YAML content with key %s: %w", key, err)
	}
	if len(children) == 0 {
		return nil, nil
	}
	return &keyNode{
		keyValue:   key,
		childNodes: children,
	}, nil
}

func isYAMLLeaf(node *yaml.Node) bool {
	return len(node.Content) == 0
}

func parseSequence(fromNode, toNode *yaml.Node, overriders ...overrider) ([]diffNode, error) {
	fromSeq, toSeq := make([]yaml.Node, len(fromNode.Content)), make([]yaml.Node, len(toNode.Content)) // NOTE: should be the same as calling `Decode`.
	for idx, v := range fromNode.Content {
		fromSeq[idx] = *v
	}
	for idx, v := range toNode.Content {
		toSeq[idx] = *v
	}
	type cachedEntry struct {
		node diffNode
		err  error
	}
	cachedDiff := make(map[string]cachedEntry)
	lcsIndices := longestCommonSubsequence(fromSeq, toSeq, func(idxFrom, idxTo int) bool {
		// Note: This function passed as `eq` should be a pure function. Therefore, its output is the same
		// given the same `idxFrom` and `idxTo`. Hence, it is not necessary to parse the nodes again.
		// In `lcs.go`, `eq` can be called twice on the same indices: once when computing LCS length, and
		// once when back-tracing to construct the LCS.
		if diff, ok := cachedDiff[cacheKey(idxFrom, idxTo)]; ok {
			return diff.err == nil && diff.node == nil
		}
		diff, err := parse(&(fromSeq[idxFrom]), &(toSeq[idxTo]), "", overriders...)
		if diff != nil { // NOTE: cache the diff only if a modification could have happened at this position.
			cachedDiff[cacheKey(idxFrom, idxTo)] = cachedEntry{
				node: diff,
				err:  err,
			}
		}
		return err == nil && diff == nil
	})
	// No difference if the two sequences have the same size and the LCS is the entire sequence.
	if len(fromSeq) == len(toSeq) && len(lcsIndices) == len(fromSeq) {
		return nil, nil
	}
	var children []diffNode
	var matchCount int
	inspector := newLCSStateMachine(fromSeq, toSeq, lcsIndices)
	for action := inspector.action(); action != actionDone; action = inspector.action() {
		switch action {
		case actionMatch:
			matchCount++
			if action := inspector.peek(); action != actionMatch {
				children = append(children, &unchangedNode{count: matchCount})
				matchCount = 0
			}
		case actionMod:
			diff := cachedDiff[cacheKey(inspector.fromIndex(), inspector.toIndex())]
			if diff.err != nil {
				return nil, diff.err
			}
			children = append(children, &seqItemNode{
				keyNode{
					keyValue:   diff.node.key(),
					childNodes: diff.node.children(),
					oldV:       diff.node.oldYAML(),
					newV:       diff.node.newYAML(),
				},
			})
		case actionDel:
			item := inspector.fromItem()
			children = append(children, &seqItemNode{
				keyNode{
					oldV: &item,
				},
			})
		case actionInsert:
			item := inspector.toItem()
			children = append(children, &seqItemNode{
				keyNode{
					newV: &item,
				},
			})
		}
		inspector.next()
	}
	return children, nil
}

func parseMap(from, to *yaml.Node, overriders ...overrider) ([]diffNode, error) {
	currMap, oldMap := make(map[string]yaml.Node), make(map[string]yaml.Node)
	if err := to.Decode(currMap); err != nil {
		return nil, err
	}
	if err := from.Decode(oldMap); err != nil {
		return nil, err
	}
	keys := unionOfKeys(currMap, oldMap)
	sort.SliceStable(keys, func(i, j int) bool { return keys[i] < keys[j] }) // NOTE: to avoid flaky unit tests.
	var children []diffNode
	for _, k := range keys {
		var currV, oldV *yaml.Node
		if v, ok := oldMap[k]; ok {
			oldV = &v
		}
		if v, ok := currMap[k]; ok {
			currV = &v
		}
		kDiff, err := parse(oldV, currV, k, overriders...)
		if err != nil {
			return nil, err
		}
		if kDiff != nil {
			children = append(children, kDiff)
		}
	}
	return children, nil
}

func unionOfKeys[T any](a, b map[string]T) []string {
	exists, keys := struct{}{}, make(map[string]struct{})
	for k := range a {
		keys[k] = exists
	}
	for k := range b {
		keys[k] = exists
	}
	keySlice, idx := make([]string, len(keys)), 0
	for k := range keys {
		keySlice[idx] = k
		idx++
	}
	return keySlice
}

func cacheKey(inFrom, inTo int) string {
	return fmt.Sprintf("%d,%d", inFrom, inTo)
}

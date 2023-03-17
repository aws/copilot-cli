// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package diff provides functionalities to compare two YAML documents.
package diff

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// For a property whose value is a list of map, map its name to the identifying property of the items.
var mapItemsIdentifiedBy = map[string]string{
	"ContainerDefinitions": "Name",
}

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

// node is a concrete implementation of a diffNode.
type node struct {
	keyValue   string
	childNodes []diffNode // A list of non-empty pointers to the children nodes.

	oldV *yaml.Node // Only populated for a leaf node (i.e. that has no child node).
	newV *yaml.Node // Only populated for a leaf node (i.e. that has no child node).
}

func (n *node) key() string {
	return n.keyValue
}

func (n *node) newYAML() *yaml.Node {
	return n.newV
}

func (n *node) oldYAML() *yaml.Node {
	return n.oldV
}

func (n *node) children() []diffNode {
	return n.childNodes
}

type seqItemNode struct {
	node
	context *seqItemContext
}

type seqItemContext struct {
	mappingNode  *yaml.Node // A seq item that needs context should have a yaml.MappingNode.
	identifiedBy string     // The field name that a seq item of yaml.MappingNode is implicitly identified by.
}

// From is the YAML document that another YAML document is compared against.
type From []byte

// Parse constructs a diff tree that represent the differences of a YAML document against the From document.
func (from From) Parse(to []byte) (Tree, error) {
	var toNode, fromNode yaml.Node
	if err := yaml.Unmarshal(to, &toNode); err != nil {
		return Tree{}, fmt.Errorf("unmarshal current template: %w", err)
	}
	if err := yaml.Unmarshal(from, &fromNode); err != nil {
		return Tree{}, fmt.Errorf("unmarshal old template: %w", err)
	}
	root, err := parse(&fromNode, &toNode, "")
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

func parse(from, to *yaml.Node, key string) (diffNode, error) {
	// Handle base cases.
	if to == nil || from == nil || to.Kind != from.Kind {
		return &node{
			keyValue: key,
			newV:     to,
			oldV:     from,
		}, nil
	}
	if isYAMLLeaf(to) && isYAMLLeaf(from) {
		if to.Value == from.Value {
			return nil, nil
		}
		return &node{
			keyValue: key,
			newV:     to,
			oldV:     from,
		}, nil
	}

	var children []diffNode
	var err error
	switch {
	case to.Kind == yaml.SequenceNode && from.Kind == yaml.SequenceNode:
		children, err = parseSequence(from, to, key)
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
	return &node{
		keyValue:   key,
		childNodes: children,
	}, nil
}

func isYAMLLeaf(node *yaml.Node) bool {
	return len(node.Content) == 0
}

func parseSequence(fromNode, toNode *yaml.Node, key string) ([]diffNode, error) {
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
		diff, err := parse(&(fromSeq[idxFrom]), &(toSeq[idxTo]), "")
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
	inspector := newLCSStateMachine(fromSeq, toSeq, lcsIndices)
	for action := inspector.action(); action != actonDone; action = inspector.action() {
		switch action {
		case actionMatch:
			// TODO(lou1415926): (x unchanged items)
		case actionMod:
			diff := cachedDiff[cacheKey(inspector.fromIndex(), inspector.toIndex())]
			if diff.err != nil {
				return nil, diff.err
			}
			var context *seqItemContext
			if byKey, ok := mapItemsIdentifiedBy[key]; ok {
				item := inspector.fromItem()
				if item.Kind == yaml.MappingNode {
					context = &seqItemContext{
						mappingNode:  &item,
						identifiedBy: byKey,
					}
				}
			}
			children = append(children, &seqItemNode{
				node: node{
					keyValue:   diff.node.key(),
					childNodes: diff.node.children(),
					oldV:       diff.node.oldYAML(),
					newV:       diff.node.newYAML(),
				},
				context: context,
			})
		case actionDel:
			item := inspector.fromItem()
			children = append(children, &seqItemNode{
				node: node{
					oldV: &item,
				},
			})
		case actionInsert:
			item := inspector.toItem()
			children = append(children, &seqItemNode{
				node: node{
					newV: &item,
				},
			})
		}
		inspector.next()
	}
	return children, nil
}

func parseMap(from, to *yaml.Node) ([]diffNode, error) {
	currMap, oldMap := make(map[string]yaml.Node), make(map[string]yaml.Node)
	if err := to.Decode(currMap); err != nil {
		return nil, err
	}
	if err := from.Decode(oldMap); err != nil {
		return nil, err
	}
	var children []diffNode
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
			children = append(children, kDiff)
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

func cacheKey(inFrom, inTo int) string {
	return fmt.Sprintf("%d,%d", inFrom, inTo)
}

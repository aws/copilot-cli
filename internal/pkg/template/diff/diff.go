// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package diff provides functionalities to compare two YAML documents.
package diff

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Node represents a segment on a difference between two YAML documents.
type Node interface {
	key() string
	newValue() *yaml.Node
	oldValue() *yaml.Node
	children() []Node
}

type basicNode struct {
	keyValue   string
	childNodes []Node // A list of non-empty pointers to the children nodes.

	oldV *yaml.Node // Only populated for a leaf node (i.e. that has no child node).
	newV *yaml.Node // Only populated for a leaf node (i.e. that has no child node).
}

func (n *basicNode) key() string {
	return n.keyValue
}

func (n *basicNode) newValue() *yaml.Node {
	return n.newV
}

func (n *basicNode) oldValue() *yaml.Node {
	return n.oldV
}

func (n *basicNode) children() []Node {
	return n.childNodes
}

type seqItemNode struct {
	basicNode
}

// From is the YAML document that another YAML document is compared against.
type From []byte

// Parse constructs a diff tree that represent the differences of a YAML document against the From document.
func (from From) Parse(to []byte) (Node, error) {
	var toNode, fromNode yaml.Node
	if err := yaml.Unmarshal(to, &toNode); err != nil {
		return nil, fmt.Errorf("unmarshal current template: %w", err)
	}
	if err := yaml.Unmarshal(from, &fromNode); err != nil {
		return nil, fmt.Errorf("unmarshal old template: %w", err)
	}
	return parse(&fromNode, &toNode, "")

}

func parse(from, to *yaml.Node, key string) (Node, error) {
	// Handle base cases.
	if to == nil || from == nil || to.Kind != from.Kind {
		return &basicNode{
			keyValue: key,
			newV:     to,
			oldV:     from,
		}, nil
	}
	if isYAMLLeaf(to) && isYAMLLeaf(from) {
		if to.Value == from.Value {
			return nil, nil
		}
		return &basicNode{
			keyValue: key,
			newV:     to,
			oldV:     from,
		}, nil
	}

	var children []Node
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
	return &basicNode{
		keyValue:   key,
		childNodes: children,
	}, nil
}

func isYAMLLeaf(node *yaml.Node) bool {
	return len(node.Content) == 0
}

func parseSequence(fromNode, toNode *yaml.Node) ([]Node, error) {
	fromSeq, toSeq := make([]yaml.Node, len(fromNode.Content)), make([]yaml.Node, len(toNode.Content)) // NOTE: should be the same as calling `Decode`.
	for idx, v := range fromNode.Content {
		fromSeq[idx] = *v
	}
	for idx, v := range toNode.Content {
		toSeq[idx] = *v
	}
	type cachedEntry struct {
		node Node
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
	if len(fromSeq) == len(toSeq) && len(lcsIndices) == len(fromSeq) {
		return nil, nil
	}
	children, inspector := make([]Node, 0), newLCSStateMachine(fromSeq, toSeq, lcsIndices)
	for action := inspector.action(); action != actonDone; action = inspector.action() {
		switch action {
		case actionMatch:
			// TODO(lou1415926): (x unchanged items)
		case actionMod:
			// TODO(lou1415926): handle list of maps modification
			diff := cachedDiff[cacheKey(inspector.fromIndex(), inspector.toIndex())]
			if diff.err != nil {
				return nil, diff.err
			}
			children = append(children, &seqItemNode{
				basicNode: *(diff.node.(*basicNode)),
			})
		case actionDel:
			item := inspector.fromItem()
			children = append(children, &seqItemNode{
				basicNode{
					oldV: &item,
				},
			})
		case actionInsert:
			item := inspector.toItem()
			children = append(children, &seqItemNode{
				basicNode{
					newV: &item,
				},
			})
		}
		inspector.next()
	}
	return children, nil
}

func parseMap(from, to *yaml.Node) ([]Node, error) {
	currMap, oldMap := make(map[string]yaml.Node), make(map[string]yaml.Node)
	if err := to.Decode(currMap); err != nil {
		return nil, err
	}
	if err := from.Decode(oldMap); err != nil {
		return nil, err
	}
	children := make([]Node, 0)
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

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package diff provides functionalities to compare two YAML documents.
package diff

import (
	"gopkg.in/yaml.v3"
)

// Node represents a difference between two YAML documents.
type Node struct {
	key      string
	children map[string]*Node // A list of non-empty pointers to the children nodes.

	oldValue *yaml.Node // Only populated for a leaf node (i.e. that has no child node).
	newValue *yaml.Node // Only populated for a leaf node (i.e. that has no child node).
}

// Parse constructs a diff tree that represent the differences between two YAML documents.
func Parse(curr, old []byte) (*Node, error) {
	return &Node{}, nil
}

// String returns the string representation of the tree stemmed from the diffNode n.
func (n *Node) String() string {
	return ""
}

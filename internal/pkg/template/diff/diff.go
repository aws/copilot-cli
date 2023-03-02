// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"gopkg.in/yaml.v3"
)

// DiffNode represents a difference between two YAML documents.
type DiffNode struct {
	key      string
	children map[string]*DiffNode // A list of non-empty pointers to the children nodes.

	oldValue *yaml.Node // Only populated for a leaf node.
	newValue *yaml.Node // Only populated for a leaf node.
}

// ConstructDiffTree constructs a diff tree that represent the differences between two YAML documents.
func ConstructDiffTree(curr, old []byte) (*DiffNode, error) {
	return &DiffNode{}, nil
}

// String returns the string representation of the tree stemmed from the diffNode n.
func (n *DiffNode) String() string {
	return ""
}

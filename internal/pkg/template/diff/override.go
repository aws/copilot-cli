// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import "gopkg.in/yaml.v3"

// overrider overrides the parsing behavior between two yaml nodes under certain keys.
type overrider interface {
	match(from, to *yaml.Node, key string) bool
	parse(from, to *yaml.Node, key string) (diffNode, error)
}

type ignoreSegment struct {
	key  string
	next *ignoreSegment
}

// ignorer ignores the diff between two yaml nodes under specified key paths.
type ignorer struct {
	curr *ignoreSegment
}

// match returns true if the difference between the from and to at the key should be ignored.
func (m *ignorer) match(_, _ *yaml.Node, key string) bool {
	if key != m.curr.key {
		return false
	}
	if m.curr.next == nil {
		return true
	}
	m.curr = m.curr.next
	return false
}

// Parse is a no-op for an ignorer.
func (m *ignorer) parse(_, _ *yaml.Node, _ string) (diffNode, error) {
	return nil, nil
}

type noneOverrider struct{}

// Match always returns false for a noneOverrider.
func (_ *noneOverrider) match(_, _ *yaml.Node, _ string) bool {
	return false
}

// Parse is a no-op for a noneOverrider.
func (m *noneOverrider) parse(_, _ *yaml.Node, _ string) (diffNode, error) {
	return nil, nil
}

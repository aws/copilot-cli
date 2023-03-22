// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import "gopkg.in/yaml.v3"

type Overrider interface {
	Match(from, to *yaml.Node, key string) bool
	Parse(from, to *yaml.Node, key string) (diffNode, error)
}

// CFNOverrider returns an Overrider that handles special behaviors when parsing the diff tree of two CFN documents written in YAML.
func CFNOverrider() Overrider {
	// return &noneOverrider{}
	return &ignorer{
		curr: &ignoreSegment{
			key: "Metadata",
			next: &ignoreSegment{
				key: "Manifest",
			},
		},
	}
}

type ignoreSegment struct {
	key  string
	next *ignoreSegment
}

type ignorer struct {
	curr *ignoreSegment
}

// Match returns true if the difference between the from and to at the key should be ignored.
func (m *ignorer) Match(_, _ *yaml.Node, key string) bool {
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
func (m *ignorer) Parse(_, _ *yaml.Node, _ string) (diffNode, error) {
	return nil, nil
}

type noneOverrider struct{}

// Match always returns false for a noneOverrider.
func (_ *noneOverrider) Match(_, _ *yaml.Node, _ string) bool {
	return false
}

// Parse is a no-op for a noneOverrider.
func (m *noneOverrider) Parse(_, _ *yaml.Node, _ string) (diffNode, error) {
	return nil, nil
}

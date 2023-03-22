// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import "gopkg.in/yaml.v3"

type overrider interface {
	match(from, to *yaml.Node, key string) bool
	parse(from, to *yaml.Node, key string) (diffNode, error)
}

// CFNOverrider returns an overrider that handles special behaviors when parsing the diff tree of two CFN documents written in YAML.
func CFNOverrider() overrider {
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

func (m *ignorer) parse(from, to *yaml.Node, key string) (diffNode, error) {
	return nil, nil
}

type noneOverrider struct{}

func (_ *noneOverrider) match(_, _ *yaml.Node, _ string) bool {
	return false
}
func (m *noneOverrider) parse(_, _ *yaml.Node, _ string) (diffNode, error) {
	return nil, nil
}

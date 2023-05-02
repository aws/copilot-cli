// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"gopkg.in/yaml.v3"
)

// overrider overrides the parsing behavior between two yaml nodes under certain keys.
type overrider interface {
	match(from, to *yaml.Node, key string, overrider overrider) bool
	parse(from, to *yaml.Node, key string, overrider overrider) (diffNode, error)
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
func (m *ignorer) match(_, _ *yaml.Node, key string, _ overrider) bool {
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
func (m *ignorer) parse(_, _ *yaml.Node, _ string, _ overrider) (diffNode, error) {
	return nil, nil
}

// intrinsicFuncFullShortFormConverter handles comparison between full/short form of an intrinsic function.
type intrinsicFuncFullShortFormConverter struct{}

// match returns true if from and to node represent the same intrinsic function written in different (full/short) form.
// Example1: "!Ref" and "Ref:" will return true.
// Example2: "!Ref" and "!Ref" will return false because they are written in the same form (i.e. short).
// Example3: "!Ref" and "Fn::GetAtt:" will return false because they are different intrinsic functions.
// For more on intrinsic functions and full/short forms, read https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/intrinsic-function-reference-ToJsonString.html.
func (_ *intrinsicFuncFullShortFormConverter) match(from, to *yaml.Node, _ string, _ overrider) bool {
	if from == nil || to == nil {
		return false
	}
	if (from.Kind == to.Kind) || (from.Kind != yaml.MappingNode && to.Kind != yaml.MappingNode) {
		// A full/short form conversion always involve at least one mapping node.
		return false
	}
	var fullFormNode, shortFormNode *yaml.Node
	if from.Kind == yaml.MappingNode {
		fullFormNode, shortFormNode = from, to
	} else {
		fullFormNode, shortFormNode = to, from
	}
	if len(fullFormNode.Content) != 2 {
		// The full form mapping node always contain only one child node, whose key is the func name in full form.
		// Read https://www.efekarakus.com/2020/05/30/deep-dive-go-yaml-cfn.html.
		return false
	}
	return eqIntrinsicFunc(intrinsicFuncFullFormName(fullFormNode), shortFormNode.Tag)
}

func (_ *intrinsicFuncFullShortFormConverter) parse(from, to *yaml.Node, key string, overrider overrider) (diffNode, error) {
	var diff diffNode
	var err error
	if from.Kind == yaml.MappingNode {
		// The full form mapping node always contain only one child node. The second element in `Content` is the 
		// value of the child node. Read https://www.efekarakus.com/2020/05/30/deep-dive-go-yaml-cfn.html.
		diff, err = parse(from.Content[1], stripTag(to), intrinsicFuncFullFormName(from), overrider)
	} else {
		diff, err = parse(stripTag(from), to.Content[1], intrinsicFuncFullFormName(to), overrider)
	}
	if err != nil {
		return nil, err
	}
	if diff == nil {
		return nil, nil
	}
	return &keyNode{
		keyValue:   key,
		childNodes: []diffNode{diff},
	}, nil
}

func stripTag(node *yaml.Node) *yaml.Node {
	return &yaml.Node{
		Kind:    node.Kind,
		Style:   node.Style,
		Content: node.Content,
		Value:   node.Value,
	}
}

func intrinsicFuncFullFormName(fullFormNode *yaml.Node) string {
	return fullFormNode.Content[0].Value
}

// Explicitly maintain a map so that we don't accidentally match nodes that are not actually intrinsic function
// but happen to match the "Fn::" and "!" format.
var intrinsicFuncFull2Short = map[string]string{
	"Ref":             "!Ref",
	"Fn::Base64":      "!Base64",
	"Fn::Cidr":        "!Cidr",
	"Fn::FindInMap":   "!FindInMap",
	"Fn::GetAtt":      "!GetAtt",
	"Fn::GetAZs":      "!GetAZs",
	"Fn::ImportValue": "!ImportValue",
	"Fn::Join":        "!Join",
	"Fn::Select":      "!Select",
	"Fn::Split":       "!Split",
	"Fn::Sub":         "!Sub",
	"Fn::Transform":   "Transform",
}

func eqIntrinsicFunc(fullFormName, shortFormName string) bool {
	expectedShort, ok := intrinsicFuncFull2Short[fullFormName]
	return ok && shortFormName == expectedShort
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

var intrinsicFunGetAttFullFormName = "Fn::GetAtt"

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

// intrinsicFuncMatcher matches intrinsic function nodes.
type intrinsicFuncMatcher struct{}

// match returns true if from and to node represent the same intrinsic function.
// Example1: "!Ref" and "Ref:" will return true.
// Example2: "!Ref" and "!Ref" will return true.
// Example3: "!Ref" and "Fn::GetAtt:" will return false because they are different intrinsic functions.
func (_ *intrinsicFuncMatcher) match(from, to *yaml.Node, _ string, _ overrider) bool {
	if from == nil || to == nil {
		return false
	}
	fromFunc, toFunc := intrinsicFuncName(from), intrinsicFuncName(to)
	if fromFunc == "" || toFunc == "" {
		return false
	}
	return funcName(fromFunc) == funcName(toFunc)
}

// intrinsicFuncMatcher matches intrinsic function nodes written in different form (full/short).
type intrinsicFuncMapTagConverter struct {
	intrinsicFunc intrinsicFuncMatcher
}

// match returns true if from and to node represent the same intrinsic function written in different (full/short) form.
// Example1: "!Ref" and "Ref:" will return true.
// Example2: "!Ref" and "!Ref" will return false because they are written in the same form (i.e. short).
// Example3: "!Ref" and "Fn::GetAtt:" will return false because they are different intrinsic functions.
// For more on intrinsic functions and full/short forms, read https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/intrinsic-function-reference-ToJsonString.html.
func (converter *intrinsicFuncMapTagConverter) match(from, to *yaml.Node, key string, overrider overrider) bool {
	if !converter.intrinsicFunc.match(from, to, key, overrider) {
		return false
	}
	// Exactly one of from and to is full form.
	return (from.Kind == yaml.MappingNode || to.Kind == yaml.MappingNode) && (from.Kind != to.Kind)
}

// parse compares two intrinsic function nodes written in different form (full vs. short).
// When the inputs to the intrinsic functions have different data types, parse assumes that no type conversion is needed
// for correct comparison.
// E.g., given "!Func: [1,2]" and "Fn::Func: '1,2'", parse assumes that comparing [1,2] with "1,2" produces the desired result.
// Note that this does not hold for "GetAtt" function: "!GetAtt: [1,2]" and "Fn::GetAtt: 1.2" should be considered the same.
func (converter *intrinsicFuncMapTagConverter) parse(from, to *yaml.Node, key string, overrider overrider) (diffNode, error) {
	var diff diffNode
	var err error
	if from.Kind == yaml.MappingNode {
		// The full form mapping node always contain only one child node. The second element in `Content` is the 
		// value of the child node. Read https://www.efekarakus.com/2020/05/30/deep-dive-go-yaml-cfn.html.
		diff, err = parse(from.Content[1], stripTag(to), intrinsicFuncFullFormName(from), overrider)
	} else {
		diff, err = parse(stripTag(from), to.Content[1], intrinsicFuncFullFormName(to), overrider)
	}
	if diff == nil {
		return nil, err
	}
	return &keyNode{
		keyValue:   key,
		childNodes: []diffNode{diff},
	}, nil
}

// getAttConverter compares two YAML nodes that calls the intrinsic function "GetAtt", one using full form, and the other short form.
// The input to "GetAtt" could be either a sequence or a scalar. All the followings are valid and should be considered equal.
// Fn::GetAtt: LogicalID.Att.SubAtt, Fn::GetAtt: [LogicalID, Att.SubAtt], !GetAtt LogicalID.Att.SubAtt, !GetAtt [LogicalID, Att.SubAtt].
type getAttConverter struct {
	mapTag intrinsicFuncMapTagConverter
}

func (converter *getAttConverter) match(from, to *yaml.Node, key string, overrider overrider) bool {
	if !converter.mapTag.intrinsicFunc.match(from, to, key, overrider) {
		return false
	}
	var fullFormNode, shortFormNode *yaml.Node
	if from.Kind == yaml.MappingNode {
		fullFormNode, shortFormNode = from, to
	} else {
		fullFormNode, shortFormNode = to, from
	}
	if intrinsicFuncFullFormName(fullFormNode) != "Fn::GetAtt" {
		return false
	}
	switch {
	case fullFormNode.Content[1].Kind == yaml.ScalarNode && shortFormNode.Kind == yaml.ScalarNode:
		fallthrough
	case fullFormNode.Content[1].Kind == yaml.SequenceNode && shortFormNode.Kind == yaml.SequenceNode:
		fallthrough
	case fullFormNode.Content[1].Kind == yaml.SequenceNode && shortFormNode.Kind == yaml.ScalarNode || fullFormNode.Content[1].Kind == yaml.ScalarNode && shortFormNode.Kind == yaml.SequenceNode:
		return true
	default:
		return false
	}
}

func (converter *getAttConverter) parse(from, to *yaml.Node, key string, overrider overrider) (diffNode, error) {
	fromValue, toValue := from, to
	switch {
	case from.Kind == yaml.MappingNode:
		fromValue = from.Content[1]
	case to.Kind == yaml.MappingNode:
		toValue = to.Content[1]
	}
	if fromValue.Kind == toValue.Kind {
		return converter.mapTag.parse(from, to, key, overrider)
	}
	var err error
	switch {
	case fromValue.Kind == yaml.ScalarNode:
		fromValue, err = getAttScalarToSeq(fromValue)
	case toValue.Kind == yaml.ScalarNode:
		toValue, err = getAttScalarToSeq(toValue)
	}
	if err != nil {
		return nil, err
	}
	diff, err := parse(fromValue, toValue, intrinsicFunGetAttFullFormName, overrider)
	if diff == nil {
		return nil, err
	}
	return &keyNode{
		keyValue:   key,
		childNodes: []diffNode{diff},
	}, nil
}

func intrinsicFuncName(node *yaml.Node) string {
	if node.Kind != yaml.MappingNode {
		return node.Tag
	}
	if len(node.Content) != 2 {
		// The full form mapping node always contain only one child node, whose key is the func name in full form.
		// Read https://www.efekarakus.com/2020/05/30/deep-dive-go-yaml-cfn.html.
		return ""
	}
	return node.Content[0].Value
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

// Transform scalar node "LogicalID.Attr" to sequence node [LogicalID, Attr].
func getAttScalarToSeq(scalarNode *yaml.Node) (*yaml.Node, error) {
	split := strings.SplitN(scalarNode.Value, ".", 2) // split has at least one element in it.
	var seqFromScalar yaml.Node
	if err := yaml.Unmarshal([]byte(fmt.Sprintf("[%s]", strings.Join(split, ","))), &seqFromScalar); err != nil {
		return nil, err
	}
	if len(seqFromScalar.Content) == 0 {
		return nil, nil
	}
	return seqFromScalar.Content[0], nil
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

func funcName(name string) string {
	return strings.TrimPrefix(strings.TrimPrefix(name, "Fn::"), "!")
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"fmt"
	"strings"

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

// Check https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/intrinsic-function-reference.html for
// a complete list of intrinsic functions. Some are not included here as they do not need an overrider.
var (
	exists                     = struct{}{}
	intrinsicFunctionFullNames = map[string]struct{}{
		"Ref":             exists,
		"Fn::Base64":      exists,
		"Fn::Cidr":        exists,
		"Fn::FindInMap":   exists,
		"Fn::GetAtt":      exists,
		"Fn::GetAZs":      exists,
		"Fn::ImportValue": exists,
		"Fn::Join":        exists,
		"Fn::Select":      exists,
		"Fn::Split":       exists,
		"Fn::Sub":         exists,
		"Fn::Transform":   exists,
		// Condition functions.
		"Condition":  exists,
		"Fn::And":    exists,
		"Fn::Equals": exists,
		"Fn::If":     exists,
		"Fn::Not":    exists,
		"Fn::Or":     exists,
	}
	intrinsicFunctionShortNames = map[string]struct{}{
		"!Ref":         exists,
		"!Base64":      exists,
		"!Cidr":        exists,
		"!FindInMap":   exists,
		"!GetAtt":      exists,
		"!GetAZs":      exists,
		"!ImportValue": exists,
		"!Join":        exists,
		"!Select":      exists,
		"!Split":       exists,
		"!Sub":         exists,
		"Transform":    exists,
		// Condition functions.
		"!Condition": exists,
		"!And":       exists,
		"!Equals":    exists,
		"!If":        exists,
		"!Not":       exists,
		"!Or":        exists,
	}
)

// intrinsicFuncMatcher matches intrinsic function nodes.
type intrinsicFuncMatcher struct{}

// match returns true if from and to node represent the same intrinsic function.
// Example1: "!Ref" and "Ref:" will return true.
// Example2: "!Ref" and "!Ref" will return true.
// Example3: "!Ref" and "Fn::GetAtt:" will return false because they are different intrinsic functions.
// Example4: "!Magic" and "Fn::Magic" will return false because they are not intrinsic functions.
func (_ *intrinsicFuncMatcher) match(from, to *yaml.Node, _ string, _ overrider) bool {
	if from == nil || to == nil {
		return false
	}
	fromFunc, toFunc := intrinsicFuncName(from), intrinsicFuncName(to)
	return fromFunc != "" && toFunc != "" && fromFunc == toFunc
}

// intrinsicFuncMatcher matches and parses two intrinsic function nodes written in different form (full/short).
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
// E.g. given "!Func: [1,2]" and "Fn::Func: '1,2'", parse assumes that comparing [1,2] with "1,2" produces the desired result.
// Note that this does not hold for "GetAtt" function: "!GetAtt: [1,2]" and "!GetAtt: 1.2" should be considered the same.
// parse assumes that from and to are matched by intrinsicFuncMapTagConverter.
func (*intrinsicFuncMapTagConverter) parse(from, to *yaml.Node, key string, overrider overrider) (diffNode, error) {
	var diff diffNode
	var err error
	if from.Kind == yaml.MappingNode {
		// The full form mapping node always contain only one child node. The second element in `Content` is the 
		// value of the child node. Read https://www.efekarakus.com/2020/05/30/deep-dive-go-yaml-cfn.html.
		diff, err = parse(from.Content[1], stripTag(to), from.Content[0].Value, overrider)
	} else {
		diff, err = parse(stripTag(from), to.Content[1], to.Content[0].Value, overrider)
	}
	if diff == nil {
		return nil, err
	}
	return &keyNode{
		keyValue:   key,
		childNodes: []diffNode{diff},
	}, nil
}

// getAttConverter matches and parses two YAML nodes that calls the intrinsic function "GetAtt".
// Unlike intrinsicFuncMapTagConverter, getAttConverter does not require "from" and "to" to be written in different form.
// The input to "GetAtt" could be either a sequence or a scalar. All the followings are valid and should be considered equal.
// Fn::GetAtt: LogicalID.Att.SubAtt, Fn::GetAtt: [LogicalID, Att.SubAtt], !GetAtt LogicalID.Att.SubAtt, !GetAtt [LogicalID, Att.SubAtt].
type getAttConverter struct {
	intrinsicFuncMapTagConverter
}

// match returns true if both from node and to node are calling the "GetAtt" intrinsic function.
// "GetAtt" only accepts either sequence or scalar, therefore match returns false if either of from and to has invalid 
// input node to "GetAtt".
// Example1: "!GetAtt" and "!GetAtt" returns true.
// Example2: "!GetAtt" and "Fn::GetAtt" returns true.
// Example3: "!Ref" and "!GetAtt" returns false.
// Example4: "!GetAtt [a,b]" and "Fn::GetAtt: a:b" returns false because the input type is wrong.
func (converter *getAttConverter) match(from, to *yaml.Node, key string, overrider overrider) bool {
	if !converter.intrinsicFunc.match(from, to, key, overrider) {
		return false
	}
	if intrinsicFuncName(from) != "GetAtt" {
		return false
	}
	fromValue, toValue := from, to
	if from.Kind == yaml.MappingNode {
		// A valid full-form intrinsic function always contain a child node.
		// This must be valid because it has passed `converter.intrinsicFunc.match`.
		fromValue = from.Content[1]
	}
	if to.Kind == yaml.MappingNode {
		toValue = to.Content[1]
	}
	return (fromValue.Kind == yaml.ScalarNode || fromValue.Kind == yaml.SequenceNode) && (toValue.Kind == yaml.ScalarNode || toValue.Kind == yaml.SequenceNode)
}

// parse compares two nodes that call the "GetAtt" function. Both from and to can be written in either full or short form.
// parse assumes that from and to are already matched by getAttConverter.
func (converter *getAttConverter) parse(from, to *yaml.Node, key string, overrider overrider) (diffNode, error) {
	// Extract the input node to GetAtt.
	fromValue, toValue := from, to
	if from.Kind == yaml.MappingNode {
		fromValue = from.Content[1] // A valid full-form intrinsic function always contain a child node.
	}
	if to.Kind == yaml.MappingNode {
		toValue = to.Content[1]
	}
	// If the input node are of the same type (i.e. both seq or both scalar), parse them normally.
	// Otherwise, first convert the scalar input to seq input, then parse.
	if fromValue.Kind != toValue.Kind {
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
	}
	diff, err := parse(stripTag(fromValue), stripTag(toValue), "Fn::GetAtt", overrider)
	if diff == nil {
		return nil, err
	}
	return &keyNode{
		keyValue:   key,
		childNodes: []diffNode{diff},
	}, nil
}

// intrinsicFuncName returns the name ofo the intrinsic function given a node.
// If the node is not an intrinsic function node, it returns an empty string.
func intrinsicFuncName(node *yaml.Node) string {
	if node.Kind != yaml.MappingNode {
		if _, ok := intrinsicFunctionShortNames[node.Tag]; !ok {
			return ""
		}
		return strings.TrimPrefix(node.Tag, "!")
	}
	if len(node.Content) != 2 {
		// The full form mapping node always contain only one child node, whose key is the func name in full form.
		// Read https://www.efekarakus.com/2020/05/30/deep-dive-go-yaml-cfn.html.
		return ""
	}
	if _, ok := intrinsicFunctionFullNames[node.Content[0].Value]; !ok {
		return ""
	}
	return strings.TrimPrefix(node.Content[0].Value, "Fn::")
}

func stripTag(node *yaml.Node) *yaml.Node {
	return &yaml.Node{
		Kind:    node.Kind,
		Style:   node.Style,
		Content: node.Content,
		Value:   node.Value,
	}
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

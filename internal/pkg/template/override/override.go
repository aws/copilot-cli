// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package override renders the manifest override rules to the CloudFormation template.
package override

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// CloudFormationTemplate overrides the given CloudFormation template by applying
// the override rules.
func CloudFormationTemplate(overrideRules []Rule, origTemp []byte) ([]byte, error) {
	content, err := unmarshalCFNYaml(origTemp)
	if err != nil {
		return nil, err
	}
	ruleNodes, err := parseRules(overrideRules)
	if err != nil {
		return nil, err
	}
	if err := applyRulesToCFNTemplate(ruleNodes, content); err != nil {
		return nil, err
	}
	output, err := marshalCFNYaml(content)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func unmarshalCFNYaml(temp []byte) (*yaml.Node, error) {
	return nil, nil
}

func marshalCFNYaml(content *yaml.Node) ([]byte, error) {
	return nil, nil
}

func applyRulesToCFNTemplate(rules []*ruleNode, content *yaml.Node) error {
	contentNode, err := getCFNTemplateDocument(content)
	if err != nil {
		return err
	}
	for _, rule := range rules {
		if err := applyRuleToCFNTemplate(rule, contentNode); err != nil {
			return err
		}
	}
	return nil
}

// getCFNTemplateDocument gets the document content of the unmarshalled CloudFormation template node
func getCFNTemplateDocument(content *yaml.Node) (*yaml.Node, error) {
	if content != nil && len(content.Content) != 0 {
		return content.Content[0], nil
	}
	return nil, fmt.Errorf("cannot apply override rule on empty CloudFormation template")
}

func applyRuleToCFNTemplate(rule *ruleNode, content *yaml.Node) error {
	if rule == nil || content == nil {
		return nil
	}
	ruleNode := rule
	var err error
	for {
		nextContentNode := content
		switch ruleNode.valueType {
		case mapType:
			nextContentNode = upsertMapNode(ruleNode, content)
		case seqType:
			if nextContentNode, err = upsertSeqNode(ruleNode, content); err != nil {
				return err
			}
		case endNodeType:
			upsertEndNode(ruleNode, content)
		}
		if ruleNode.next == nil {
			break
		}
		content = nextContentNode
		ruleNode = ruleNode.next
	}
	return nil
}

func upsertMapNode(rule *ruleNode, content *yaml.Node) *yaml.Node {
	for i := 0; i < len(content.Content); i += 2 {
		// The content of a map always come in pairs.
		// The first element represents a key, ex: {Value: "ELBIngressGroup", Kind: ScalarNode, Tag: "!!str", Content: nil}
		// The second element holds the value, ex: {Value: "", Kind: MappingNode, Tag:"!!map", Content:[...]}
		if content.Content[i].Value == rule.name {
			return content.Content[i+1]
		}
	}
	newLabelNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   nodeTagStr,
		Value: rule.name,
	}
	content.Content = append(content.Content, newLabelNode)
	newValNode := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  nodeTagMap,
	}
	content.Content = append(content.Content, newValNode)
	return newValNode
}

func upsertSeqNode(rule *ruleNode, content *yaml.Node) (*yaml.Node, error) {
	for i := 0; i < len(content.Content); i += 2 {
		if content.Content[i].Value == rule.name {
			seqNode := content.Content[i+1]
			if rule.seqValue.appendToLast {
				newMapNode := &yaml.Node{
					Kind: yaml.MappingNode,
					Tag:  nodeTagMap,
				}
				seqNode.Content = append(seqNode.Content, newMapNode)
				return newMapNode, nil
			}
			if rule.seqValue.index < len(seqNode.Content) {
				return seqNode.Content[rule.seqValue.index], nil
			} else {
				return nil, fmt.Errorf("cannot specify %s[%d] because the current length is %d. Use [%s] to append to the sequence instead",
					rule.name, rule.seqValue.index, len(seqNode.Content), seqAppendToLastSymbol)
			}
		}
	}
	newLabelNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   nodeTagStr,
		Value: rule.name,
	}
	content.Content = append(content.Content, newLabelNode)
	newValNode := &yaml.Node{
		Kind: yaml.SequenceNode,
		Tag:  nodeTagSeq,
	}
	content.Content = append(content.Content, newValNode)
	newMapNode := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  nodeTagMap,
	}
	newValNode.Content = append(newValNode.Content, newMapNode)
	return newMapNode, nil
}

func upsertEndNode(rule *ruleNode, content *yaml.Node) {
	for ind, c := range content.Content {
		if c.Value == rule.name && ind < len(content.Content) {
			content.Content[ind+1] = rule.endNodeValue
			return
		}
	}
	newLabelNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   nodeTagStr,
		Value: rule.name,
	}
	content.Content = append(content.Content, newLabelNode)
	content.Content = append(content.Content, rule.endNodeValue)
}

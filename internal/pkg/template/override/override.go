// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package override provides functionality to replace content from vended templates.
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

func applyRulesToCFNTemplate(rules []contentUpserter, content *yaml.Node) error {
	contentNode, err := getTemplateDocument(content)
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

// getTemplateDocument gets the document content of the unmarshalled YMAL template node
func getTemplateDocument(content *yaml.Node) (*yaml.Node, error) {
	if content != nil && len(content.Content) != 0 {
		return content.Content[0], nil
	}
	return nil, fmt.Errorf("cannot apply override rule on empty YAML template")
}

func applyRuleToCFNTemplate(ruleSegment contentUpserter, content *yaml.Node) error {
	if ruleSegment == nil || content == nil {
		return nil
	}
	var err error
	var nextContentNode *yaml.Node
	for {
		if nextContentNode, err = ruleSegment.Upsert(content); err != nil {
			return err
		}
		if ruleSegment.NextNode() == nil {
			break
		}
		content = nextContentNode
		ruleSegment = ruleSegment.NextNode()
	}
	return nil
}

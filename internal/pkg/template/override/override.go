// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package override renders the manifest override rules to the CloudFormation template.
package override

import "gopkg.in/yaml.v3"

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
	return nil
}

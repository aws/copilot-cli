// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package override renders the manifest override rules to the CloudFormation template.
package override

type templateContent map[interface{}]interface{}

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
	overriddenContent, err := content.applyRulesToCFNTemplate(ruleNodes)
	if err != nil {
		return nil, err
	}
	output, err := marshalCFNYaml(overriddenContent)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func unmarshalCFNYaml(temp []byte) (templateContent, error) {
	return nil, nil
}

func marshalCFNYaml(content templateContent) ([]byte, error) {
	return nil, nil
}

func (c templateContent) applyRulesToCFNTemplate(rules []*ruleNode) (templateContent, error) {
	return nil, nil
}

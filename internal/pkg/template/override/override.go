// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package override renders the manifest override rules to the CloudFormation template.
package override

// CloudFormationTemplate overrides the given CloudFormation template by applying
// the override rules.
func CloudFormationTemplate(overrideRules []Rule, origTemp string) (string, error) {
	content, err := unmarshalCFNYaml(origTemp)
	if err != nil {
		return "", err
	}
	ruleNodes, err := parseRules(overrideRules)
	if err != nil {
		return "", err
	}
	destContent, err := applyRulesToCFNTemplate(ruleNodes, content)
	if err != nil {
		return "", err
	}
	output, err := marshalCFNYaml(destContent)
	if err != nil {
		return "", err
	}
	return output, nil
}

func unmarshalCFNYaml(temp string) (map[interface{}]interface{}, error) {
	return nil, nil
}

func marshalCFNYaml(content map[interface{}]interface{}) (string, error) {
	return "", nil
}

func applyRulesToCFNTemplate(rules []*ruleNode, origContent map[interface{}]interface{}) (map[interface{}]interface{}, error) {
	return nil, nil
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package override renders the manifest override rules to the CloudFormation template.
package override

// OverrideCloudFormationTemplate overrides the given CloudFormation template by applying
// the override rules.
func OverrideCloudFormationTemplate(overrideRules []string, origTemp string) (string, error) {
	content, err := unmarshalCFNYaml(origTemp)
	if err != nil {
		return "", err
	}
	rules, err := parseRules(overrideRules)
	if err != nil {
		return "", err
	}
	destContent, err := applyRulesToCFNTemplate(rules, content)
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

func applyRulesToCFNTemplate(rules []cfnOverrideRule, origContent map[interface{}]interface{}) (map[interface{}]interface{}, error) {
	return nil, nil
}

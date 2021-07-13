// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

type cfnOverrideRule struct {
	// ruleString example: "ContainerDefinitions[0].Ulimits.HardLimit: 1024"
	ruleString string
}

func parseRules(inputs []string) ([]cfnOverrideRule, error) {
	var rules []cfnOverrideRule
	for _, input := range inputs {
		r := cfnOverrideRule{
			ruleString: input,
		}
		if err := r.validate(); err != nil {
			return nil, err
		}
		if err := r.parse(); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

func (r cfnOverrideRule) validate() error {
	return nil
}

func (r cfnOverrideRule) parse() error {
	return nil
}

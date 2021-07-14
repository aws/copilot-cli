// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

// Rule is the override rule override package uses.
type Rule struct {
	// pathSegment example: "ContainerDefinitions[0].Ulimits.HardLimit: 1024"
	pathSegment string
	value       interface{}
}

type ruleNode struct {
	next *ruleNode
}

func parseRules(rules []Rule) ([]*ruleNode, error) {
	var ruleNodes []*ruleNode
	for _, r := range rules {
		if err := r.validate(); err != nil {
			return nil, err
		}
		node, err := r.parse()
		if err != nil {
			return nil, err
		}
		ruleNodes = append(ruleNodes, node)
	}
	return ruleNodes, nil
}

func (r Rule) validate() error {
	return nil
}

func (r Rule) parse() (*ruleNode, error) {
	return nil, nil
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import "gopkg.in/yaml.v3"

const (
	seqAppendToLastSymbol = "-"
)

type nodeValueType int

const (
	endNodeType nodeValueType = iota + 1
	seqType
	mapType
)

const (
	nodeTagBool = "!!bool"
	nodeTagInt  = "!!int"
	nodeTagStr  = "!!str"
	nodeTagSeq  = "!!seq"
	nodeTagMap  = "!!map"
)

// Rule is the override rule override package uses.
type Rule struct {
	// PathSegment example: "ContainerDefinitions[0].Ulimits.HardLimit"
	// PathSegment string
	// Value       *yaml.Node
}

type ruleNode struct {
	name         string
	valueType    nodeValueType
	seqValue     nodeSeqValue
	endNodeValue *yaml.Node
	next         *ruleNode
}

type nodeSeqValue struct {
	index        int
	appendToLast bool
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

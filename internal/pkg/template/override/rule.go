// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package override

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

const (
	seqAppendToLastSymbol = "-"
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

type contentUpserter interface {
	Upsert(content *yaml.Node) (*yaml.Node, error)
	NextNode() contentUpserter
}

// upsertNode represents a node that needs to be upserted.
type upsertNode struct {
	key           string
	valueToInsert *yaml.Node
	next          contentUpserter
}

// NextNode returns the next node.
func (m *upsertNode) NextNode() contentUpserter {
	return m.next
}

// mapUpsertNode represents a map node that needs to be upserted at the given key.
type mapUpsertNode struct {
	upsertNode
}

// Upsert upserts a node into given yaml content.
func (m *mapUpsertNode) Upsert(content *yaml.Node) (*yaml.Node, error) {
	// If it contains the value to insert, upsert the value to the yaml.
	if m.valueToInsert != nil {
		m.upsertValue(content)
		return nil, nil
	}
	for i := 0; i < len(content.Content); i += 2 {
		// The content of a map always come in pairs. If the node pair exists, return the map node.
		// Note that the rest of code massively uses yaml node tree.
		// Please refer to https://www.efekarakus.com/2020/05/30/deep-dive-go-yaml-cfn.html
		if content.Content[i].Value == m.key {
			return content.Content[i+1], nil
		}
	}
	// If the node pair doesn't exist, create the label node first and then a map node.
	// Finally we return the created map node.
	newLabelNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   nodeTagStr,
		Value: m.key,
	}
	content.Content = append(content.Content, newLabelNode)
	newValNode := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  nodeTagMap,
	}
	content.Content = append(content.Content, newValNode)
	return newValNode, nil
}

func (m *mapUpsertNode) upsertValue(content *yaml.Node) {
	// If the node pair exists, substitute with the value node.
	for i := 0; i < len(content.Content); i += 2 {
		if m.key == content.Content[i].Value {
			content.Content[i+1] = m.valueToInsert
		}
	}
	// Otherwise, we create the label node then append the value node.
	newLabelNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   nodeTagStr,
		Value: m.key,
	}
	content.Content = append(content.Content, newLabelNode)
	content.Content = append(content.Content, m.valueToInsert)
}

// seqIdxUpsertNode represents a sequence node that needs to be upserted at index.
type seqIdxUpsertNode struct {
	index        int
	appendToLast bool
	upsertNode
}

// Upsert upserts a node into given yaml content.
func (s *seqIdxUpsertNode) Upsert(content *yaml.Node) (*yaml.Node, error) {
	// If it contains the value to insert, upsert the value to the yaml.
	if s.valueToInsert != nil {
		return nil, s.upsertValue(content)
	}
	// If the node pair exists, we check if we need to append the node to the end.
	// If so, create a map node and return it since we want to go deeper to upsert the value.
	// Here we assume it is not possible for the yaml we want to override to have nested sequence:
	// Mapping01:
	//   - - foo
	//     - bar
	//   - - boo
	// The example above will be translated to "Mapping01[0][1]" to refer to "bar".
	// If not check if the given index is within the sequence range.
	for i := 0; i < len(content.Content); i += 2 {
		if content.Content[i].Value == s.key {
			seqNode := content.Content[i+1]
			if s.appendToLast {
				newMapNode := &yaml.Node{
					Kind: yaml.MappingNode,
					Tag:  nodeTagMap,
				}
				seqNode.Content = append(seqNode.Content, newMapNode)
				return newMapNode, nil
			}
			if s.index < len(seqNode.Content) {
				return seqNode.Content[s.index], nil
			} else {
				return nil, fmt.Errorf("cannot specify %s[%d] because the current length is %d. Use [%s] to append to the sequence instead",
					s.key, s.index, len(seqNode.Content), seqAppendToLastSymbol)
			}
		}
	}
	// If the node pair doesn't exist, create the sequence node pair and a map node.
	// Then return the created map node, since we want to go deeper to upsert the value.
	newLabelNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   nodeTagStr,
		Value: s.key,
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

func (s *seqIdxUpsertNode) upsertValue(content *yaml.Node) error {
	for i := 0; i < len(content.Content); i += 2 {
		if content.Content[i].Value == s.key {
			seqNode := content.Content[i+1]
			if s.appendToLast {
				seqNode.Content = append(seqNode.Content, s.valueToInsert)
				return nil
			}
			if s.index < len(seqNode.Content) {
				seqNode.Content[s.index] = s.valueToInsert
				return nil
			}
			return fmt.Errorf("cannot specify %s[%d] because the current length is %d. Use [%s] to append to the sequence instead",
				s.key, s.index, len(seqNode.Content), seqAppendToLastSymbol)
		}
	}
	newLabelNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   nodeTagStr,
		Value: s.key,
	}
	content.Content = append(content.Content, newLabelNode)
	newValNode := &yaml.Node{
		Kind:    yaml.SequenceNode,
		Tag:     nodeTagSeq,
		Content: []*yaml.Node{s.valueToInsert},
	}
	content.Content = append(content.Content, newValNode)
	return nil
}

func parseRules(rules []Rule) ([]contentUpserter, error) {
	var ruleNodes []contentUpserter
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

func (r Rule) parse() (contentUpserter, error) {
	return nil, nil
}

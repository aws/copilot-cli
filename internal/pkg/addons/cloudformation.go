// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"gopkg.in/yaml.v3"
)

type cfnSection int

const (
	metadataSection cfnSection = iota + 1
	parametersSection
)

// cfnTemplate represents a parsed YAML AWS CloudFormation template.
type cfnTemplate struct {
	Metadata   yaml.Node `yaml:"Metadata,omitempty"`
	Parameters yaml.Node `yaml:"Parameters,omitempty"`
}

// merge combines non-empty fields of other with cf's fields.
func (t *cfnTemplate) merge(other cfnTemplate) error {
	if err := t.mergeMetadata(other.Metadata); err != nil {
		return err
	}
	if err := t.mergeParameters(other.Parameters); err != nil {
		return err
	}
	return nil
}

// mergeMetadata updates cf's Metadata with additional metadata.
// If the key already exists in Metadata but with a different definition, returns errMetadataAlreadyExists.
func (t *cfnTemplate) mergeMetadata(metadata yaml.Node) error {
	if err := mergeMapNodes(&t.Metadata, &metadata); err != nil {
		return wrapKeyAlreadyExistsErr(metadataSection, err)
	}
	return nil
}

// mergeParameters updates cf's Parameters with additional parameters.
// If the parameterLogicalID already exists but with a different value, returns errParameterAlreadyExists.
func (t *cfnTemplate) mergeParameters(params yaml.Node) error {
	if err := mergeMapNodes(&t.Parameters, &params); err != nil {
		return wrapKeyAlreadyExistsErr(parametersSection, err)
	}
	return nil
}

// mergeMapNodes merges the src node to dst.
// It assumes that both nodes have a "mapping" type. See https://yaml.org/spec/1.2/spec.html#id2802432
//
// If a key in src already exists in dst and the values are different, then returns a errKeyAlreadyExists.
// If a key in src already exists in dst and the values are equal, then do nothing.
// If a key in src doesn't exist in dst, then add the key and its value to dst.
func mergeMapNodes(dst, src *yaml.Node) error {
	if src.IsZero() {
		return nil
	}

	if dst.IsZero() {
		*dst = *src
		return nil
	}

	dstMap := mappingNode(dst)
	var newContent []*yaml.Node
	for i := 0; i < len(src.Content); i += 2 {
		// The content of a map always come in pairs.
		// The first element represents a key, ex: {Value: "ELBIngressGroup", Kind: ScalarNode, Tag: "!!str", Content: nil}
		// The second element holds the value, ex: {Value: "", Kind: MappingNode, Tag:"!!map", Content:[...]}
		key := src.Content[i].Value
		srcValue := src.Content[i+1]

		dstValue, ok := dstMap[key]
		if !ok {
			// The key doesn't exist in dst, we want to retain the two src nodes.
			newContent = append(newContent, src.Content[i], src.Content[i+1])
			continue
		}

		if !isEqual(dstValue, srcValue) {
			return &errKeyAlreadyExists{
				Key:    key,
				First:  dstValue,
				Second: srcValue,
			}
		}
	}
	dst.Content = append(dst.Content, newContent...)
	return nil
}

// mappingNode transforms a flat "mapping" yaml.Node to a hashmap.
func mappingNode(n *yaml.Node) map[string]*yaml.Node {
	m := make(map[string]*yaml.Node)
	for i := 0; i < len(n.Content); i += 2 {
		m[n.Content[i].Value] = n.Content[i+1]
	}
	return m
}

// isEqual returns true if the first and second nodes are deeply equal in all of their values except stylistic ones.
//
// We ignore the style (ex: single quote vs. double) in which the nodes are defined, the comments associated with
// the nodes, and the indentation and position of the nodes as they're only visual properties and don't matter.
func isEqual(first *yaml.Node, second *yaml.Node) bool {
	if first == nil {
		return second == nil
	}
	if second == nil {
		return false
	}
	if len(first.Content) != len(second.Content) {
		return false
	}
	hasSameContent := true
	for i := 0; i < len(first.Content); i += 1 {
		hasSameContent = hasSameContent && isEqual(first.Content[i], second.Content[i])
	}
	return first.Kind == second.Kind &&
		first.Tag == second.Tag &&
		first.Value == second.Value &&
		first.Anchor == second.Anchor &&
		isEqual(first.Alias, second.Alias) &&
		hasSameContent
}

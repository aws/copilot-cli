// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"reflect"

	"gopkg.in/yaml.v3"
)

type cfnSection int

const (
	metadataSection cfnSection = iota + 1
)

// cloudformation represents a parsed YAML AWS CloudFormation template.
type cloudformation struct {
	Metadata yaml.Node `yaml:"Metadata,omitempty"`
}

// merge combines non-empty fields of other with cf's fields.
func (cf *cloudformation) merge(other cloudformation) error {
	if err := cf.mergeMetadata(other.Metadata); err != nil {
		return err
	}
	return nil
}

// mergeMetadata updates cf's Metadata with additional metadata.
// If the key already exists in Metadata but with a different definition, returns errMetadataKeyAlreadyExists.
func (cf *cloudformation) mergeMetadata(metadata yaml.Node) error {
	if err := mergeMapNodes(&cf.Metadata, &metadata); err != nil {
		return wrapKeyAlreadyExistsErr(metadataSection, err)
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
	newSrcContent := make([]*yaml.Node, len(src.Content))
	copy(newSrcContent, src.Content)
	for i := 0; i < len(src.Content); i += 2 {
		// The content of a map always come in pairs.
		// The first element represents a key, ex: {Value: "ELBIngressGroup", Kind: ScalarNode, Tag: "!!str", Content: nil}
		// The second element is another map that holds the value, ex: {Value: "", Kind: MappingNode, Tag:"!!map", Content:[...]}
		key := src.Content[i].Value
		srcValue := src.Content[i+1]

		dstValue, ok := dstMap[key]
		if !ok {
			// The key doesn't exist in dst, we want to retain the two src nodes.
			continue
		}
		if !reflect.DeepEqual(dstValue, srcValue) {
			return &errKeyAlreadyExists{
				Key:    key,
				First:  dstValue,
				Second: srcValue,
			}
		}
		// Remove the two src nodes since they already exists in dst.
		newSrcContent = append(newSrcContent[:i], newSrcContent[i+2:]...)
	}
	dst.Content = append(dst.Content, newSrcContent...)
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

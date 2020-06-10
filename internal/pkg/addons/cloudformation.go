// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addons

import (
	"errors"
	"fmt"

	"gopkg.in/yaml.v3"
)

type cfnSection int

const (
	metadataSection cfnSection = iota + 1
	parametersSection
	mappingsSection
)

// cfnTemplate represents a parsed YAML AWS CloudFormation template.
type cfnTemplate struct {
	Metadata   yaml.Node `yaml:"Metadata,omitempty"`
	Parameters yaml.Node `yaml:"Parameters,omitempty"`
	Mappings   yaml.Node `yaml:"Mappings,omitempty"`
	Conditions yaml.Node `yaml:"Conditions,omitempty"`
}

// merge combines non-empty fields of other with t's fields.
func (t *cfnTemplate) merge(other cfnTemplate) error {
	if err := t.mergeMetadata(other.Metadata); err != nil {
		return err
	}
	if err := t.mergeParameters(other.Parameters); err != nil {
		return err
	}
	if err := t.mergeMappings(other.Mappings); err != nil {
		return err
	}
	if err := t.mergeConditions(other.Conditions); err != nil {
		return err
	}
	return nil
}

// mergeMetadata updates t's Metadata with additional metadata.
// If the key already exists in Metadata but with a different definition, returns errMetadataAlreadyExists.
func (t *cfnTemplate) mergeMetadata(metadata yaml.Node) error {
	if err := mergeSingleLevelMaps(&t.Metadata, &metadata); err != nil {
		return wrapKeyAlreadyExistsErr(metadataSection, err)
	}
	return nil
}

// mergeParameters updates t's Parameters with additional parameters.
// If the parameterLogicalID already exists but with a different value, returns errParameterAlreadyExists.
func (t *cfnTemplate) mergeParameters(params yaml.Node) error {
	if err := mergeSingleLevelMaps(&t.Parameters, &params); err != nil {
		return wrapKeyAlreadyExistsErr(parametersSection, err)
	}
	return nil
}

// mergeMappings updates t's Mappings with additional mappings.
// If a mapping already exists with a different value, returns errMappingAlreadyExists.
func (t *cfnTemplate) mergeMappings(mappings yaml.Node) error {
	if err := mergeTwoLevelMaps(&t.Mappings, &mappings); err != nil {
		return wrapKeyAlreadyExistsErr(mappingsSection, err)
	}
	return nil
}

// mergeConditions updates t's Conditions with additional conditions.
func (t *cfnTemplate) mergeConditions(conditions yaml.Node) error {
	if err := mergeSingleLevelMaps(&t.Conditions, &conditions); err != nil {
		return err
	}
	return nil
}

// mergeTwoLevelMaps merges the top and second level keys of src node to dst.
// It assumes that both nodes are nested maps. For example, a node can hold:
// Mapping01:  # Top Level is a map.
//    Key01:   # Second Level is also a map.
//      Name: Value01
//    Key02:   # Second Level.
//      Name: Value02
//
// If a second-level key exists in both src and dst but has different values, then returns an errKeyAlreadyExists.
// If a second-level key exists in src but not in dst, it merges the second level key to dst.
// If a top-level key exists in src but not in dst, merges it.
func mergeTwoLevelMaps(dst, src *yaml.Node) error {
	secondLevelHandler := func(key string, dstVal, srcVal *yaml.Node) error {
		if err := mergeSingleLevelMaps(dstVal, srcVal); err != nil {
			var keyExistsErr *errKeyAlreadyExists
			if errors.As(err, &keyExistsErr) {
				keyExistsErr.Key = fmt.Sprintf("%s.%s", key, keyExistsErr.Key)
				return keyExistsErr
			}
			return err
		}
		return nil
	}
	return mergeMapNodes(dst, src, secondLevelHandler)
}

// mergeSingleLevelMaps merges the keys of src node to dst.
// It assumes that both nodes are a map. For example, a node can hold:
// Resources:
//    MyResourceName:
//        ...  # If the contents of "MyResourceName" are not equal in both src and dst then err.
//
// If a key exists in both src and dst but has different values, then returns an errKeyAlreadyExists.
// If a key exists in both src and dst and the values are equal, then do nothing.
// If a key exists in src but not in dst, merges it.
func mergeSingleLevelMaps(dst, src *yaml.Node) error {
	areValuesEqualHandler := func(key string, dstVal, srcVal *yaml.Node) error {
		if !isEqual(dstVal, srcVal) {
			return &errKeyAlreadyExists{
				Key:    key,
				First:  dstVal,
				Second: srcVal,
			}
		}
		return nil
	}
	return mergeMapNodes(dst, src, areValuesEqualHandler)
}

type keyExistsHandler func(key string, dstVal, srcVal *yaml.Node) error

// mergeMapNodes merges the src node to dst.
// It assumes that both nodes have a "mapping" type. See https://yaml.org/spec/1.2/spec.html#id2802432
//
// If a key exists in src but not in dst, then adds the key and value to dst.
// If a key exists in both src and dst, invokes the keyExistsHandler.
func mergeMapNodes(dst, src *yaml.Node, handler keyExistsHandler) error {
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

		if err := handler(key, dstValue, srcValue); err != nil {
			return err
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

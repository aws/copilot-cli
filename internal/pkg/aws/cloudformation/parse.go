// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseTemplateDescriptions parses a YAML CloudFormation template to retrieve all human readable
// descriptions associated with a resource. It assumes that all description comments are defined immediately
// under the logical ID of the resource.
//
// For example, if a resource in a template is defined as:
//   Cluster:
//     # An ECS Cluster to hold your services.
//     Type: AWS::ECS::Cluster
//
// The output will be descriptionFor["Cluster"] = "An ECS Cluster to hold your services."
func ParseTemplateDescriptions(body string) (descriptionFor map[string]string, err error) {
	type template struct {
		Resources yaml.Node `yaml:"Resources"`
	}
	var tpl template
	if err := yaml.Unmarshal([]byte(body), &tpl); err != nil {
		return nil, fmt.Errorf("unmarshal cloudformation template: %w", err)
	}

	descriptionFor = make(map[string]string)
	for i := 0; i < len(tpl.Resources.Content); i += 2 {
		// The content of a map, like the "Resources" field, always come in pairs.
		// The first element represents a key, ex: {Value: "Cluster", Kind: ScalarNode, Tag: "!!str", Content: nil}
		// The second element holds the properties, ex: {Value: "", Kind: MappingNode, Tag:"!!map", Content:[...]}
		resource := tpl.Resources.Content[i]
		properties := tpl.Resources.Content[i+1]
		description := properties.Content[0].HeadComment // The description is the comment above the first property.
		if description == "" {
			continue
		}

		logicalID := resource.Value
		descriptionFor[logicalID] = strings.Trim(description, " #" /* remove both spaces and the # character */)
	}
	return descriptionFor, nil
}

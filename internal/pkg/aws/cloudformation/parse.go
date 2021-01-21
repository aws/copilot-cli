// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"fmt"

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
		Resources map[string]yaml.Node `yaml:"Resources"`
	}
	var tpl template
	if err := yaml.Unmarshal([]byte(body), &tpl); err != nil {
		return nil, fmt.Errorf("unmarshal cloudformation template: %w", err)
	}
	type metadata struct {
		Description string `yaml:"aws:copilot:description"`
	}
	type resource struct {
		Metadata metadata `yaml:"Metadata"`
	}

	descriptionFor = make(map[string]string)
	for logicalID, value := range tpl.Resources {
		var r resource
		if err := value.Decode(&r); err != nil {
			return nil, fmt.Errorf("decode resource Metadata for description: %w", err)
		}
		if description := r.Metadata.Description; description != "" {
			descriptionFor[logicalID] = description
		}
	}
	return descriptionFor, nil
}

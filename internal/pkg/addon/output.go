// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// AWS CloudFormation resource types.
const (
	secretManagerSecretType = "AWS::SecretsManager::Secret"
	iamManagedPolicyType    = "AWS::IAM::ManagedPolicy"
	securityGroupType       = "AWS::EC2::SecurityGroup"
)

// Output represents an output from a CloudFormation template.
type Output struct {
	// Name is the Logical ID of the output.
	Name string
	// IsSecret is true if the output value refers to a SecretsManager ARN. Otherwise, false.
	IsSecret bool
	// IsManagedPolicy is true if the output value refers to an IAM ManagedPolicy ARN. Otherwise, false.
	IsManagedPolicy bool
	// SecurityGroup is true if the output value refers a SecurityGroup ARN. Otherwise, false.
	IsSecurityGroup bool
}

// Outputs parses the Outputs section of a CloudFormation template to extract logical IDs and returns them.
func Outputs(template string) ([]Output, error) {
	type cfnTemplate struct {
		Resources yaml.Node `yaml:"Resources"`
		Outputs   yaml.Node `yaml:"Outputs"`
	}
	var tpl cfnTemplate
	if err := yaml.Unmarshal([]byte(template), &tpl); err != nil {
		return nil, fmt.Errorf("unmarshal addon cloudformation template: %w", err)
	}

	typeFor, err := parseTypeByLogicalID(&tpl.Resources)
	if err != nil {
		return nil, err
	}

	outputNodes, err := parseOutputNodes(&tpl.Outputs)
	if err != nil {
		return nil, err
	}

	var outputs []Output
	for _, outputNode := range outputNodes {
		output := Output{
			Name:            outputNode.name(),
			IsSecret:        false,
			IsManagedPolicy: false,
			IsSecurityGroup: false,
		}
		ref, ok := outputNode.ref()
		if ok {
			output.IsSecret = typeFor[ref] == secretManagerSecretType
			output.IsManagedPolicy = typeFor[ref] == iamManagedPolicyType
			output.IsSecurityGroup = typeFor[ref] == securityGroupType
		}
		outputs = append(outputs, output)
	}
	return outputs, nil
}

// parseTypeByLogicalID returns a map where the key is the resource's logical ID and the value is the CloudFormation Type
// of the resource such as "AWS::IAM::Role".
func parseTypeByLogicalID(resourcesNode *yaml.Node) (typeFor map[string]string, err error) {
	if resourcesNode.Kind != yaml.MappingNode {
		// "Resources" is a required field in CloudFormation, check if it's defined as a map.
		return nil, errors.New(`"Resources" field in cloudformation template is not a map`)
	}

	typeFor = make(map[string]string)
	for _, content := range mappingContents(resourcesNode) {
		logicalIDNode := content.keyNode
		fieldsNode := content.valueNode
		fields := struct {
			Type string `yaml:"Type"`
		}{}
		if err := fieldsNode.Decode(&fields); err != nil {
			return nil, fmt.Errorf(`decode the "Type" field of resource "%s": %w`, logicalIDNode.Value, err)
		}
		typeFor[logicalIDNode.Value] = fields.Type
	}
	return typeFor, nil
}

func parseOutputNodes(outputsNode *yaml.Node) ([]*outputNode, error) {
	if outputsNode.IsZero() {
		// "Outputs" is an optional field so we can skip it.
		return nil, nil
	}

	if outputsNode.Kind != yaml.MappingNode {
		return nil, errors.New(`"Outputs" field in cloudformation template is not a map`)
	}

	var nodes []*outputNode
	for _, content := range mappingContents(outputsNode) {
		nameNode := content.keyNode

		fields := struct {
			Value yaml.Node `yaml:"Value"`
		}{}
		if err := content.valueNode.Decode(&fields); err != nil {
			return nil, fmt.Errorf(`decode the "Value" field of output "%s": %w`, nameNode.Value, err)
		}
		nodes = append(nodes, &outputNode{
			nameNode:  nameNode,
			valueNode: &fields.Value,
		})
	}
	return nodes, nil
}

type outputNode struct {
	nameNode  *yaml.Node
	valueNode *yaml.Node
}

func (n *outputNode) name() string {
	return n.nameNode.Value
}

func (n *outputNode) ref() (string, bool) {
	switch n.valueNode.Kind {
	case yaml.ScalarNode:
		// It's a string like "!Ref MyDynamoDBTable"
		if n.valueNode.Tag != "!Ref" {
			return "", false
		}
		return strings.TrimSpace(n.valueNode.Value), true
	case yaml.MappingNode:
		// Check if it's a map like "Ref: MyDynamoDBTable"
		fields := struct {
			Ref string `yaml:"Ref"`
		}{}
		_ = n.valueNode.Decode(&fields)
		if fields.Ref == "" {
			return "", false
		}
		return fields.Ref, true
	default:
		return "", false
	}
}

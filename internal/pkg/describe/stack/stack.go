// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
)

type cfn interface {
	Describe(name string) (*cloudformation.StackDescription, error)
	StackResources(name string) ([]*cloudformation.StackResource, error)
	Metadata(opt cloudformation.MetadataOpts) (string, error)
}

// StackDescription is the description of a cloudformation stack.
type StackDescription struct {
	Parameters map[string]string
	Tags       map[string]string
	Outputs    map[string]string
}

// Resource contains cloudformation stack resource info.
type Resource struct {
	Type       string `json:"type"`
	PhysicalID string `json:"physicalID"`
	LogicalID  string `json:"logicalID,omitempty"`
}

// HumanString returns the stringified Resource struct with human readable format.
func (c Resource) HumanString() string {
	return fmt.Sprintf("%s\t%s\n", c.Type, c.PhysicalID)
}

// StackDescriber retrieves information about a stack.
type StackDescriber struct {
	name string
	cfn  cfn
}

// NewStackDescriber instantiates a new StackDescriber.
func NewStackDescriber(stackName string, sess *session.Session) *StackDescriber {
	return &StackDescriber{
		name: stackName,
		cfn:  cloudformation.New(sess),
	}
}

// Describe retrieves information about a cloudformation stack.
func (d *StackDescriber) Describe() (StackDescription, error) {
	descr, err := d.cfn.Describe(d.name)
	if err != nil {
		return StackDescription{}, fmt.Errorf("describe stack %s: %w", d.name, err)
	}
	params := make(map[string]string)
	for _, param := range descr.Parameters {
		params[aws.StringValue(param.ParameterKey)] = aws.StringValue(param.ParameterValue)
	}
	outputs := make(map[string]string)
	for _, out := range descr.Outputs {
		outputs[aws.StringValue(out.OutputKey)] = aws.StringValue(out.OutputValue)
	}
	tags := make(map[string]string)
	for _, tag := range descr.Tags {
		tags[aws.StringValue(tag.Key)] = aws.StringValue(tag.Value)
	}
	return StackDescription{
		Parameters: params,
		Tags:       tags,
		Outputs:    outputs,
	}, nil
}

// Resources retrieves the information about a stack's resources.
func (d *StackDescriber) Resources() ([]*Resource, error) {
	resources, err := d.cfn.StackResources(d.name)
	if err != nil {
		return nil, fmt.Errorf("retrieve resources for stack %s: %w", d.name, err)
	}
	return flattenResources(resources), nil
}

// StackMetadata returns the metadata of the stack.
func (d *StackDescriber) StackMetadata() (string, error) {
	metadata, err := d.cfn.Metadata(cloudformation.MetadataWithStackName(d.name))
	if err != nil {
		return "", fmt.Errorf("get metadata for stack %s: %w", d.name, err)
	}
	return metadata, nil
}

// StackSetMetadata returns the metadata of the stackset.
func (d *StackDescriber) StackSetMetadata() (string, error) {
	metadata, err := d.cfn.Metadata(cloudformation.MetadataWithStackSetName(d.name))
	if err != nil {
		return "", fmt.Errorf("get metadata for stack set %s: %w", d.name, err)
	}
	return metadata, nil
}

func flattenResources(stackResources []*cloudformation.StackResource) []*Resource {
	var resources []*Resource
	for _, stackResource := range stackResources {
		resources = append(resources, &Resource{
			Type:       aws.StringValue(stackResource.ResourceType),
			PhysicalID: aws.StringValue(stackResource.PhysicalResourceId),
			LogicalID:  aws.StringValue(stackResource.LogicalResourceId),
		})
	}
	return resources
}

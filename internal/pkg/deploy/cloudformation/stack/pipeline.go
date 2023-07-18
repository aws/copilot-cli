// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
)

type pipelineStackConfig struct {
	*deploy.CreatePipelineInput
	parser pipelineParser
}

// NewPipelineStackConfig sets up a struct which can provide values to CloudFormation for
// spinning up a pipeline.
func NewPipelineStackConfig(in *deploy.CreatePipelineInput) *pipelineStackConfig {
	return &pipelineStackConfig{
		CreatePipelineInput: in,
		parser:              template.New(),
	}
}

// StackName returns the name of the CloudFormation stack.
func (p *pipelineStackConfig) StackName() string {
	return NameForPipeline(p.AppName, p.Name, p.IsLegacy)
}

// Template returns the CloudFormation template for the service parametrized for the environment.
func (p *pipelineStackConfig) Template() (string, error) {
	content, err := p.parser.ParsePipeline(p)
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

// SerializedParameters returns the CloudFormation stack's parameters serialized to a JSON document.
func (p *pipelineStackConfig) SerializedParameters() (string, error) {
	// No-op for now.
	return "", nil
}

// Parameters returns the list of CloudFormation parameters used by the template.
func (p *pipelineStackConfig) Parameters() ([]*cloudformation.Parameter, error) {
	return nil, nil
}

// Tags returns the tags that should be applied to the pipeline CloudFormation stack.
func (p *pipelineStackConfig) Tags() []*cloudformation.Tag {
	defaultTags := map[string]string{
		deploy.AppTagKey: p.AppName,
	}
	if !p.IsLegacy {
		defaultTags[deploy.PipelineTagKey] = p.Name
	}
	return mergeAndFlattenTags(p.AdditionalTags, defaultTags)
}

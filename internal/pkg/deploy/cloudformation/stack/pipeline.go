// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/aws-sdk-go/service/cloudformation"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
)

const pipelineCfnTemplatePath = "cicd/pipeline_cfn.yml"

type pipelineStackConfig struct {
	*deploy.CreatePipelineInput
	parser template.Parser
}

func NewPipelineStackConfig(in *deploy.CreatePipelineInput) *pipelineStackConfig {
	return &pipelineStackConfig{
		CreatePipelineInput: in,
		parser:              template.New(),
	}
}

func (p *pipelineStackConfig) StackName() string {
	return p.Name
}

func (p *pipelineStackConfig) Template() (string, error) {
	content, err := p.parser.Parse(pipelineCfnTemplatePath, p, template.WithFuncs(cfTemplateFunctions))
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

func (p *pipelineStackConfig) Parameters() ([]*cloudformation.Parameter, error) {
	return nil, nil
}

func (p *pipelineStackConfig) Tags() []*cloudformation.Tag {
	return mergeAndFlattenTags(p.AdditionalTags, map[string]string{
		AppTagKey: p.AppName,
	})
}

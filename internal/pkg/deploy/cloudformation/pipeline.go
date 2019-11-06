// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"bytes"
	"fmt"
	"regexp"
	"text/template"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/templates"
)

const pipelineCfnTemplatePath = "cicd/pipeline_cfn.yml"

// matches a line of comment in the template
var commentRegex = regexp.MustCompile(`\s*#.*`)

type pipelineStackConfig struct {
	*deploy.CreatePipelineInput
}

func newPipelineStackConfig(in *deploy.CreatePipelineInput) *pipelineStackConfig {
	return &pipelineStackConfig{
		CreatePipelineInput: in,
	}
}

func (p *pipelineStackConfig) StackName() string {
	return p.ProjectName + "-" + p.Name
}

func (p *pipelineStackConfig) Template() (string, error) {
	content, err := templates.Box().FindString(pipelineCfnTemplatePath)
	if err != nil {
		return "", &ErrTemplateNotFound{templateLocation: pipelineCfnTemplatePath, parentErr: err}
	}
	
	tpl, err := template.New("pipelineCfn").Parse(content)
	if err != nil {
		return "", fmt.Errorf("parse CloudFormation template for project %s, pipeline %s, error: %w",
		p.ProjectName, p.Name, err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, p); err != nil {
		return "", fmt.Errorf("execute CloudFormation template for project %s, pipeline %s, error: %w",
		p.ProjectName, p.Name, err)
	}
	return buf.String(), nil
}

func (p *pipelineStackConfig) Parameters() []*cloudformation.Parameter {
	return nil
}

func (p *pipelineStackConfig) Tags() []*cloudformation.Tag {
	return []*cloudformation.Tag{
		{
			Key:   aws.String(projectTagKey),
			Value: aws.String(p.ProjectName),
		},
	}
}

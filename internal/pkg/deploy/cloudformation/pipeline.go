// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
)

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
	// TODO: Render the template
	return "", nil
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

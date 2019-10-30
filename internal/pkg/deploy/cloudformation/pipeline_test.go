// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
)

const (
	projectName = "chickenProject"
	pipelineName = "wingspipeline"
)

func TestPipelineParameters(t *testing.T) {
	pipeline := newPipelineStackConfig(
		mockDeployPipelineInput(),
	)

	require.Nil(t, pipeline.Parameters(), "pipeline cloudformation template should not expose any parameters")
}

func TestPipelineTags(t *testing.T) {
	pipeline := newPipelineStackConfig(
		mockDeployPipelineInput(),
	)

	expectedTags := []*cloudformation.Tag{
		{
			Key:   aws.String(projectTagKey),
			Value: aws.String(projectName),
		},
	}
	require.ElementsMatch(t, expectedTags, pipeline.Tags())
}

func TestPipelineStackName(t *testing.T) {
	pipeline := newPipelineStackConfig(
		mockDeployPipelineInput(),
	)

	require.Equal(t, projectName+"-"+pipelineName,
		pipeline.StackName(), "unexpected StackName")
}

func mockAssociatedEnv(envName, region string, isProd bool) *deploy.AssociatedEnvironment {
	return &deploy.AssociatedEnvironment{
		Name: envName,
		Region: region,
		AccountId: "012345678910",
		Prod: isProd,
	}
}

func mockDeployPipelineInput() *deploy.CreatePipelineInput {	
	return &deploy.CreatePipelineInput{
		ProjectName: projectName,
		Name:            pipelineName         ,
		Source: &deploy.Source{
			ProviderName: "GitHub",
			Properties: map[string]interface{}{
				"repository": "aws/somethingCool",
				"branch":     "master",
			},

		},
		Stages: []deploy.PipelineStage{
			{
				AssociatedEnvironment: mockAssociatedEnv("test", "us-west-2", false),
				LocalApplications: []string{"frontend", "backend"},
			},
			{
				AssociatedEnvironment: mockAssociatedEnv("prod", "us-east-1", true),
				LocalApplications: []string{"frontend", "backend"},
			},
		},
	}
}
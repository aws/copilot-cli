// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
)

const projectName = "chickenProject"

func TestPipelineParameters(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pipelineManifest := mockPipelineManifest()
	mockEnvG := validMockEnvironment(ctrl)
	mockWs := validMockWorkspace(ctrl)
	pipeline, err := newPipelineStackConfig(
		mockEnvG, mockWs, pipelineManifest,
	)
	require.NoError(t, err, "newPipelineStackConfig fails")

	require.Nil(t, pipeline.Parameters(), "pipeline cloudformation template should not expose any parameters")
}

func TestPipelineTags(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pipelineManifest := mockPipelineManifest()
	mockEnvG := validMockEnvironment(ctrl)
	mockWs := validMockWorkspace(ctrl)
	pipeline, err := newPipelineStackConfig(
		mockEnvG, mockWs, pipelineManifest,
	)
	require.NoError(t, err, "newPipelineStackConfig fails")
	expectedTags := []*cloudformation.Tag{
		{
			Key:   aws.String(projectTagKey),
			Value: aws.String(projectName),
		},
	}
	require.ElementsMatch(t, expectedTags, pipeline.Tags())
}

func TestPipelineStackName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	pipelineManifest := mockPipelineManifest()
	mockEnvG := validMockEnvironment(ctrl)
	mockWs := validMockWorkspace(ctrl)
	pipeline, err := newPipelineStackConfig(
		mockEnvG, mockWs, pipelineManifest,
	)
	require.NoError(t, err, "newPipelineStackConfig fails")

	require.Equal(t, projectName+"-"+pipelineManifest.Name,
		pipeline.StackName(), "unexpected StackName")
}

func validMockEnvironment(ctrl *gomock.Controller) archer.EnvironmentGetter {
	mockEnvG := mocks.NewMockEnvironmentGetter(ctrl)
	mockEnvG.EXPECT().
		GetEnvironment(gomock.Eq(projectName), gomock.Any()).
		Return(mockEnvironment("wingsEnv", "us-west-2", false), nil)

	return mockEnvG
}

func validMockWorkspace(ctrl *gomock.Controller) archer.Workspace {
	mockWs := mocks.NewMockWorkspace(ctrl)
	mockWs.EXPECT().Summary().Return(&archer.WorkspaceSummary{
		ProjectName: projectName,
	}, nil)

	mockWs.EXPECT().AppNames().Return([]string{"frontend", "backend"}, nil)
	return mockWs
}

func mockEnvironment(envName string, region string, isProd bool) *archer.Environment {
	return &archer.Environment{
		Project:     projectName,
		Name:        envName,
		Region:      region,
		AccountID:   "012345678910",
		Prod:        isProd,
		RegistryURL: "dontCare",
	}
}

func mockPipelineManifest() *manifest.PipelineManifest {
	return &manifest.PipelineManifest{
		Name:    "pipeline",
		Version: manifest.Ver1,
		Source: &manifest.Source{
			ProviderName: "GitHub",
			Properties: map[string]interface{}{
				"repository": "aws/somethingCool",
				"branch":     "master",
			},
		},
		Stages: []manifest.PipelineStage{
			{
				Name: "chicken",
			},
		},
	}
}

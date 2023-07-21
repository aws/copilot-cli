// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
)

const (
	projectName  = "chickenProject"
	pipelineName = "wingspipeline"

	toolsAccountID = "012345678910"
	defaultBranch  = "main"
)

func TestPipelineParameters(t *testing.T) {
	pipeline := NewPipelineStackConfig(
		mockCreatePipelineInput(),
	)
	params, _ := pipeline.Parameters()

	require.Nil(t, params, "pipeline cloudformation template should not expose any parameters")
}

func TestPipelineTags(t *testing.T) {
	testCases := map[string]struct {
		in         *deploy.CreatePipelineInput
		wantedTags []*cloudformation.Tag
	}{
		"pipeline with legacy naming": {
			in: &deploy.CreatePipelineInput{
				AppName:  projectName,
				Name:     pipelineName,
				IsLegacy: true,
			},
			wantedTags: []*cloudformation.Tag{
				{
					Key:   aws.String(deploy.AppTagKey),
					Value: aws.String(projectName),
				},
			},
		},
		"pipeline with namespaced naming and additional tags": {
			in: mockCreatePipelineInput(),
			wantedTags: []*cloudformation.Tag{
				{
					Key:   aws.String(deploy.AppTagKey),
					Value: aws.String(projectName),
				},
				{
					Key:   aws.String(deploy.PipelineTagKey),
					Value: aws.String(pipelineName),
				},
				{
					Key:   aws.String("owner"),
					Value: aws.String("boss"),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			pipeline := NewPipelineStackConfig(
				tc.in,
			)
			require.ElementsMatch(t, tc.wantedTags, pipeline.Tags())
		})
	}
}

func TestPipelineStackName(t *testing.T) {
	testCases := map[string]struct {
		in              *deploy.CreatePipelineInput
		wantedStackName string
	}{
		"pipeline with legacy naming": {
			in: &deploy.CreatePipelineInput{
				AppName:  projectName,
				Name:     pipelineName,
				IsLegacy: true,
			},
			wantedStackName: pipelineName,
		},
		"pipeline with namespaced naming and additional tags": {
			in:              mockCreatePipelineInput(),
			wantedStackName: fmt.Sprintf("pipeline-%s-%s", projectName, pipelineName),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			pipeline := NewPipelineStackConfig(
				tc.in,
			)
			require.Equal(t, tc.wantedStackName, pipeline.StackName(), "unexpected StackName")
		})
	}
}

func TestPipelineStackConfig_Template(t *testing.T) {
	testCases := map[string]struct {
		in               *deploy.CreatePipelineInput
		mockDependencies func(ctrl *gomock.Controller, c *pipelineStackConfig)

		wantedTemplate string
		wantedError    error
	}{
		"error parsing file": {
			in: mockCreatePipelineInput(),
			mockDependencies: func(ctrl *gomock.Controller, c *pipelineStackConfig) {
				m := mocks.NewMockpipelineParser(ctrl)
				m.EXPECT().ParsePipeline(c).Return(nil, errors.New("some error"))
				c.parser = m
			},
			wantedError: errors.New("some error"),
		},
		"successfully parses file": {
			in: mockCreatePipelineInput(),
			mockDependencies: func(ctrl *gomock.Controller, c *pipelineStackConfig) {
				m := mocks.NewMockpipelineParser(ctrl)
				m.EXPECT().ParsePipeline(c).Return(&template.Content{
					Buffer: bytes.NewBufferString("pipeline"),
				}, nil)
				c.parser = m
			},
			wantedTemplate: "pipeline",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := &pipelineStackConfig{
				CreatePipelineInput: tc.in,
			}
			tc.mockDependencies(ctrl, c)

			// WHEN
			template, err := c.Template()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedTemplate, template)
		})
	}
}

func mockCreatePipelineInput() *deploy.CreatePipelineInput {
	return &deploy.CreatePipelineInput{
		AppName: projectName,
		Name:    pipelineName,
		Source: &deploy.GitHubSource{
			RepositoryURL: "hencrice/amazon-ecs-cli-v2",
			Branch:        defaultBranch,
		},
		Stages: []deploy.PipelineStage{},
		ArtifactBuckets: []deploy.ArtifactBucket{
			{
				BucketName: "chicken-us-east-1",
				KeyArn:     fmt.Sprintf("arn:aws:kms:us-east-1:%s:key/30131d3f-c30f-4d49-beaa-cf4bfc07f34e", toolsAccountID),
			},
			{
				BucketName: "chicken-us-west-2",
				KeyArn:     fmt.Sprintf("arn:aws:kms:us-west-2:%s:key/80de5f7f-422d-4dff-8f4d-01f6ec5715bc", toolsAccountID),
			},
			// assume the pipeline is hosted in a region that does not contain any copilot environment
			{
				BucketName: "chicken-us-west-1",
				KeyArn:     fmt.Sprintf("arn:aws:kms:us-west-1:%s:key/75668c57-ec4b-4d0c-b880-8dc3fa78f6d1", toolsAccountID),
			},
		},
		AdditionalTags: map[string]string{
			"owner": "boss",
		},
	}
}

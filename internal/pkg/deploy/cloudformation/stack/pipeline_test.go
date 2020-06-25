// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
)

const (
	projectName  = "chickenProject"
	pipelineName = "wingspipeline"

	toolsAccountID = "012345678910"
	envAccountID   = "109876543210"
)

func TestPipelineParameters(t *testing.T) {
	pipeline := NewPipelineStackConfig(
		mockCreatePipelineInput(),
	)
	params, _ := pipeline.Parameters()

	require.Nil(t, params, "pipeline cloudformation template should not expose any parameters")
}

func TestPipelineTags(t *testing.T) {
	pipeline := NewPipelineStackConfig(
		mockCreatePipelineInput(),
	)

	expectedTags := []*cloudformation.Tag{
		{
			Key:   aws.String(AppTagKey),
			Value: aws.String(projectName),
		},
		{
			Key:   aws.String("owner"),
			Value: aws.String("boss"),
		},
	}
	require.ElementsMatch(t, expectedTags, pipeline.Tags())
}

func TestPipelineStackName(t *testing.T) {
	pipeline := NewPipelineStackConfig(
		mockCreatePipelineInput(),
	)

	require.Equal(t, pipelineName, pipeline.StackName(), "unexpected StackName")
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
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Parse(pipelineCfnTemplatePath, c, gomock.Any()).Return(nil, errors.New("some error"))
				c.parser = m
			},
			wantedError: errors.New("some error"),
		},
		"successfully parses file": {
			in: mockCreatePipelineInput(),
			mockDependencies: func(ctrl *gomock.Controller, c *pipelineStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Parse(pipelineCfnTemplatePath, c, gomock.Any()).Return(&template.Content{
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

func mockAssociatedEnv(envName, region string, isProd bool) *deploy.AssociatedEnvironment {
	return &deploy.AssociatedEnvironment{
		Name:      envName,
		Region:    region,
		AccountID: envAccountID,
		Prod:      isProd,
	}
}

func mockCreatePipelineInput() *deploy.CreatePipelineInput {
	return &deploy.CreatePipelineInput{
		AppName: projectName,
		Name:    pipelineName,
		Source: &deploy.Source{
			ProviderName: "GitHub",
			Properties: map[string]interface{}{
				"repository":          "hencrice/amazon-ecs-cli-v2",
				"branch":              "master",
				"access_token_secret": "testGitHubSecret",
			},
		},
		Stages: []deploy.PipelineStage{
			{
				AssociatedEnvironment: mockAssociatedEnv("test-chicken", "us-west-2", false),
				LocalServices:         []string{"frontend", "backend"},
				TestCommands:          []string{"echo 'bok bok bok'", "make test"},
			},
			{
				AssociatedEnvironment: mockAssociatedEnv("prod-can-fly", "us-east-1", true),
				LocalServices:         []string{"frontend", "backend"},
			},
		},
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

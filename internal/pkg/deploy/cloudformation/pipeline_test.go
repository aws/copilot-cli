// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudformation

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
)

const (
	projectName = "chickenProject"
	pipelineName = "wingspipeline"

	toolsAccountID = "620297136107"
	envAccountID =  "786043277641"
)

func TestPipelineParameters(t *testing.T) {
	pipeline := newPipelineStackConfig(
		mockCreatePipelineInput(),
	)

	require.Nil(t, pipeline.Parameters(), "pipeline cloudformation template should not expose any parameters")
}

func TestPipelineTags(t *testing.T) {
	pipeline := newPipelineStackConfig(
		mockCreatePipelineInput(),
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
		mockCreatePipelineInput(),
	)

	require.Equal(t, projectName+"-"+pipelineName,
		pipeline.StackName(), "unexpected StackName")
}

func TestPipelineTemplateRendering(t *testing.T) {
    expectedTemplate, err := ioutil.ReadFile("./testdata/rendered_pipeline_cfn_template.yml")
    require.NoError(t, err, "expected template can not be read")

	pipeline := newPipelineStackConfig(
		mockCreatePipelineInput(),
	)
	tmpl, err := pipeline.Template()
    require.NoError(t, err, "template serialization failed")
    require.Equal(t, string(tmpl), string(expectedTemplate), "the rendered template differs from the expected")
}

func mockAssociatedEnv(envName, region string, isProd bool) *deploy.AssociatedEnvironment {
	return &deploy.AssociatedEnvironment{
		Name: envName,
		Region: region,
		AccountID: envAccountID,
		Prod: isProd,
	}
}

func mockCreatePipelineInput() *deploy.CreatePipelineInput {	
	return &deploy.CreatePipelineInput{
		ProjectName: projectName,
		Name:            pipelineName         ,
		Source: &deploy.Source{
			ProviderName: "GitHub",
			Properties: map[string]interface{}{
				"repository": "hencrice/amazon-ecs-cli-v2",
				"branch":     "master",
				deploy.GithubSecretIdKeyName: "testGitHubSecret",
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
		ArtifactBuckets: []deploy.ArtifactBucket{
			{
				BucketArn: "arn:aws:s3:::chicken-us-east-1",
				KeyArn: fmt.Sprintf("arn:aws:kms:us-east-1:%s:key/30131d3f-c30f-4d49-beaa-cf4bfc07f34e", toolsAccountID),
			},
			{
				BucketArn: "arn:aws:s3:::chicken-us-west-2",
				KeyArn: fmt.Sprintf("arn:aws:kms:us-west-2:%s:key/80de5f7f-422d-4dff-8f4d-01f6ec5715bc", toolsAccountID),
			},
			// assume the pipeline is hosted in a region that does not contain any archer environment
			{
				BucketArn: "arn:aws:s3:::chicken-us-west-1",
				KeyArn: fmt.Sprintf("arn:aws:kms:us-west-1:%s:key/75668c57-ec4b-4d0c-b880-8dc3fa78f6d1", toolsAccountID),
			},
		},
	}
}
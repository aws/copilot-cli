// +build integration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/stretchr/testify/require"
)

// TestPipeline_Template ensures that the CloudFormation template generated for a pipeline matches our pre-defined template.
func TestPipeline_Template(t *testing.T) {
	ps := stack.NewPipelineStackConfig(&deploy.CreatePipelineInput{
		AppName: "phonetool",
		Name:    "phonetool-pipeline",
		Source: &deploy.Source{
			ProviderName: "GitHub",
			Properties: map[string]interface{}{
				"repository":          "https://github.com/aws/phonetool",
				"branch":              "mainline",
				"access_token_secret": "mysecret",
			},
		},
		Stages: []deploy.PipelineStage{
			{
				AssociatedEnvironment: &deploy.AssociatedEnvironment{
					Name:      "test",
					Region:    "us-west-2",
					AccountID: "1111",
				},
				LocalServices:    []string{"api"},
				RequiresApproval: false,
				TestCommands:     []string{`echo "test"`},
			},
		},
		ArtifactBuckets: []deploy.ArtifactBucket{
			{
				BucketName: "fancy-bucket",
				KeyArn:     "arn:aws:kms:us-west-2:1111:key/abcd",
			},
		},
		AdditionalTags: nil,
	})

	actual, err := ps.Template()
	require.NoError(t, err, "template should have rendered successfully")
	expected, err := ioutil.ReadFile(filepath.Join("testdata", "pipeline", "template.yaml"))
	require.NoError(t, err, "should be able to read expected template file")
	require.Equal(t, string(expected), actual)
}

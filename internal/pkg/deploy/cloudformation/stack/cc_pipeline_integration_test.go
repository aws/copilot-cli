//go:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
)

// TestCC_Pipeline_Template ensures that the CloudFormation template generated for a pipeline matches our pre-defined template.
func TestCC_Pipeline_Template(t *testing.T) {
	var build deploy.Build
	build.Init(nil, "copilot/pipelines/phonetool-pipeline/")

	var stage deploy.PipelineStage
	stage.Init(&config.Environment{
		App:              "phonetool",
		Name:             "staging-test",
		Region:           "us-west-2",
		AccountID:        "1111",
		ExecutionRoleARN: "arn:aws:iam::1111:role/phonetool-staging-test-CFNExecutionRole",
		ManagerRoleARN:   "arn:aws:iam::1111:role/phonetool-staging-test-EnvManagerRole",
	}, &manifest.PipelineStage{
		Name:         "staging-test",
		TestCommands: []string{`echo "test"`},
	}, []string{"api"})
	ps := stack.NewPipelineStackConfig(&deploy.CreatePipelineInput{
		AppName: "phonetool",
		Name:    "phonetool-pipeline",
		Source: &deploy.CodeCommitSource{
			ProviderName:         manifest.CodeCommitProviderName,
			RepositoryURL:        "https://us-west-2.console.aws.amazon.com/codesuite/codecommit/repositories/aws-sample/browse",
			Branch:               "main",
			OutputArtifactFormat: "CODEBUILD_CLONE_REF",
		},
		Build:  &build,
		Stages: []deploy.PipelineStage{stage},
		ArtifactBuckets: []deploy.ArtifactBucket{
			{
				BucketName: "fancy-bucket",
				KeyArn:     "arn:aws:kms:us-west-2:1111:key/abcd",
			},
		},
		AdditionalTags: nil,
		Version:        "v1.28.0",
	})

	actual, err := ps.Template()
	require.NoError(t, err, "template should have rendered successfully")
	actualInBytes := []byte(actual)
	m1 := make(map[interface{}]interface{})
	require.NoError(t, yaml.Unmarshal(actualInBytes, m1))

	wanted, err := os.ReadFile(filepath.Join("testdata", "pipeline", "cc_template.yaml"))
	require.NoError(t, err, "should be able to read expected template file")
	wantedInBytes := []byte(wanted)
	m2 := make(map[interface{}]interface{})
	require.NoError(t, yaml.Unmarshal(wantedInBytes, m2))

	require.Equal(t, m2, m1)
}

//go:build integration || localintegration
// +build integration localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"gopkg.in/yaml.v3"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/stretchr/testify/require"
)

// TestGHPipeline_Template ensures that the CloudFormation template generated for a pipeline matches our pre-defined template.
func TestGHPipeline_Template(t *testing.T) {
	var build deploy.Build
	build.Init(nil, "copilot/pipelines/phonetool-pipeline/")
	var stage deploy.PipelineStage
	stage.Init(&config.Environment{
		App:              "phonetool",
		Name:             "test",
		Region:           "us-west-2",
		AccountID:        "1111",
		ExecutionRoleARN: "arn:aws:iam::1111:role/phonetool-test-CFNExecutionRole",
		ManagerRoleARN:   "arn:aws:iam::1111:role/phonetool-test-EnvManagerRole",
	}, &manifest.PipelineStage{
		Name:         "test",
		TestCommands: []string{`echo "test"`},
	}, []string{"api"})
	ps := stack.NewPipelineStackConfig(&deploy.CreatePipelineInput{
		AppName: "phonetool",
		Name:    "phonetool-pipeline",
		Source: &deploy.GitHubSource{
			ProviderName:  manifest.GithubProviderName,
			RepositoryURL: "https://github.com/aws/phonetool",
			Branch:        "mainline",
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
	})

	actual, err := ps.Template()
	require.NoError(t, err, "template should have rendered successfully")
	actualInBytes := []byte(actual)
	m1 := make(map[interface{}]interface{})
	require.NoError(t, yaml.Unmarshal(actualInBytes, m1))

	wanted, err := ioutil.ReadFile(filepath.Join("testdata", "pipeline", "gh_template.yaml"))
	require.NoError(t, err, "should be able to read expected template file")
	wantedInBytes := []byte(wanted)
	m2 := make(map[interface{}]interface{})
	require.NoError(t, yaml.Unmarshal(wantedInBytes, m2))

	require.Equal(t, m2, m1)
}

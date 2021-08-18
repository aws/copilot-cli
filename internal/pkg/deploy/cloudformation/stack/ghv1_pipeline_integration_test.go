// +build integration localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"gopkg.in/yaml.v3"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/stretchr/testify/require"
)

// TestGHv1Pipeline_Template ensures that the CloudFormation template generated for a pipeline matches our pre-defined template.
func TestGHv1Pipeline_Template(t *testing.T) {
	ps := stack.NewPipelineStackConfig(&deploy.CreatePipelineInput{
		AppName: "phonetool",
		Name:    "phonetool-pipeline",
		Source: &deploy.GitHubV1Source{
			ProviderName:                manifest.GithubV1ProviderName,
			RepositoryURL:               "https://github.com/aws/phonetool",
			Branch:                      "mainline",
			PersonalAccessTokenSecretID: "my secret",
		},
		Build: &deploy.Build{
			Image: "aws/codebuild/amazonlinux2-x86_64-standard:3.0",
		},
		Stages: []deploy.PipelineStage{
			{
				AssociatedEnvironment: &deploy.AssociatedEnvironment{
					Name:      "test",
					Region:    "us-west-2",
					AccountID: "1111",
				},
				LocalWorkloads:   []string{"api"},
				RequiresApproval: false,
				TestCommands:     []string{`echo "test"`},
			},
		},
		ArtifactBuckets: []deploy.Bucket{
			{
				Region:       "us-west-2",
				Name:         "fancy-bucket",
				Environments: []string{"test"},
				KeyARN:       "arn:aws:kms:us-west-2:1111:key/abcd",
			},
		},
		AdditionalTags: nil,
	})

	actual, err := ps.Template()
	require.NoError(t, err, "template should have rendered successfully")
	actualInBytes := []byte(actual)
	m1 := make(map[interface{}]interface{})
	require.NoError(t, yaml.Unmarshal(actualInBytes, m1))

	wanted, err := ioutil.ReadFile(filepath.Join("testdata", "pipeline", "ghv1_template.yml"))
	require.NoError(t, err, "should be able to read expected template file")
	wantedInBytes := []byte(wanted)
	m2 := make(map[interface{}]interface{})
	require.NoError(t, yaml.Unmarshal(wantedInBytes, m2))

	require.Equal(t, m2, m1)
}

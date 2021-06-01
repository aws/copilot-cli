// +build integration localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
)

// TestBB_Pipeline_Template ensures that the CloudFormation template generated for a pipeline matches our pre-defined template.
func TestBB_Pipeline_Template(t *testing.T) {
	ps := stack.NewPipelineStackConfig(&deploy.CreatePipelineInput{
		AppName: "phonetool",
		Name:    "phonetool-pipeline",
		Source: &deploy.BitbucketSource{
			ProviderName:  manifest.BitbucketProviderName,
			RepositoryURL: "https://huanjani@bitbucket.org/huanjani/sample",
			Branch:        "main",
		},
		Build: deploy.PipelineBuildFromManifest(nil),
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

	wanted, err := ioutil.ReadFile(filepath.Join("testdata", "pipeline", "bb_template.yaml"))
	require.NoError(t, err, "should be able to read expected template file")
	wantedInBytes := []byte(wanted)
	m2 := make(map[interface{}]interface{})
	require.NoError(t, yaml.Unmarshal(wantedInBytes, m2))

	require.Equal(t, m2, m1)
}

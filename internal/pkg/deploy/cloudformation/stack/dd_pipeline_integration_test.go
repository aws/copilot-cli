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

const (
	pipelineManifestPath = "dd_manifest.yaml"
	pipelineStackPath    = "dd_stack_template.yaml"
)

// TestDD_Pipeline_Template ensures that the CloudFormation template generated for a pipeline matches our pre-defined template.
func TestDD_Pipeline_Template(t *testing.T) {
	path := filepath.Join("testdata", "pipeline", pipelineManifestPath)
	manifestBytes, err := os.ReadFile(path)
	require.NoError(t, err)

	pipelineMft, err := manifest.UnmarshalPipeline([]byte(manifestBytes))
	require.NoError(t, err)

	var build deploy.Build

	if err = build.Init(pipelineMft.Build, ""); err != nil {
		t.Errorf("build init: %v", err)
	}

	source, _, err := deploy.PipelineSourceFromManifest(pipelineMft.Source)

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

	serializer := stack.NewPipelineStackConfig(&deploy.CreatePipelineInput{
		AppName: "phonetool",
		Build:   &build,
		Source:  source,
		Stages:  []deploy.PipelineStage{stage},
		ArtifactBuckets: []deploy.ArtifactBucket{
			{
				BucketName: "fancy-bucket",
				KeyArn:     "arn:aws:kms:us-west-2:1111:key/abcd",
			},
		},
		AdditionalTags: nil,
		Version:        "v1.28.0",
	})

	actual, err := serializer.Template()
	require.NoError(t, err, "template should have rendered successfully")
	actualInBytes := []byte(actual)
	m1 := make(map[any]any)
	require.NoError(t, yaml.Unmarshal(actualInBytes, m1))

	wanted, err := os.ReadFile(filepath.Join("testdata", "pipeline", pipelineStackPath))
	require.NoError(t, err, "should be able to read expected template file")
	wantedInBytes := []byte(wanted)
	m2 := make(map[any]any)
	require.NoError(t, yaml.Unmarshal(wantedInBytes, m2))

	require.Equal(t, m2, m1)
}

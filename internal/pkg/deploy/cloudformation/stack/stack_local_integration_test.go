//go:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

const (
	autoScalingManifestPath = "manifest.yml"

	appName  = "my-app"
	imageURL = "mockImageURL"
	imageTag = "latest"
)

var envName = "test"

func Test_Stack_Local_Integration(t *testing.T) {
	const (
		wantedAutoScalingCFNTemplatePath  = "autoscaling-svc-cf.yml"
		wantedAutoScalingCFNParameterPath = "cf.params.json"
		wantedOverrideCFNTemplatePath     = "override-cf.yml"
	)

	path := filepath.Join("testdata", "stacklocal", autoScalingManifestPath)
	wantedManifestBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	mft, err := manifest.UnmarshalWorkload(wantedManifestBytes)
	require.NoError(t, err)
	content := mft.Manifest()

	v, ok := content.(*manifest.LoadBalancedWebService)
	require.Equal(t, ok, true)

	// Create in-memory mock file system.
	wd, err := os.Getwd()
	require.NoError(t, err)
	fs := afero.NewMemMapFs()
	_ = fs.MkdirAll(fmt.Sprintf("%s/copilot", wd), 0755)
	_ = afero.WriteFile(fs, fmt.Sprintf("%s/copilot/.workspace", wd), []byte(fmt.Sprintf("---\napplication: %s", "DavidsApp")), 0644)
	require.NoError(t, err)

	ws, err := workspace.Use(fs)
	require.NoError(t, err)

	_, err = addon.ParseFromWorkload(aws.StringValue(v.Name), ws)
	var notFound *addon.ErrAddonsNotFound
	require.ErrorAs(t, err, &notFound)

	envConfig := &manifest.Environment{
		Workload: manifest.Workload{
			Name: &envName,
		},
	}
	envConfig.HTTPConfig.Public.Certificates = []string{"mockCertARN"}
	serializer, err := stack.NewLoadBalancedWebService(stack.LoadBalancedWebServiceConfig{
		App:                &config.Application{Name: appName},
		EnvManifest:        envConfig,
		Manifest:           v,
		ArtifactBucketName: "bucket",
		RuntimeConfig: stack.RuntimeConfig{
			PushedImages: map[string]stack.ECRImage{
				aws.StringValue(v.Name): {
					RepoURL:  imageURL,
					ImageTag: imageTag,
				},
			},
			EnvVersion: "v1.42.0",
			Version:    "v1.29.0",
		},
	})
	require.NoError(t, err)
	tpl, err := serializer.Template()
	require.NoError(t, err)

	t.Run("CloudFormation template must contain autoscaling resources", func(t *testing.T) {
		path := filepath.Join("testdata", "stacklocal", wantedAutoScalingCFNTemplatePath)
		wantedCFNBytes, err := os.ReadFile(path)
		require.NoError(t, err)

		require.Contains(t, tpl, string(wantedCFNBytes))
	})

	t.Run("CloudFormation template must be overridden correctly", func(t *testing.T) {
		path := filepath.Join("testdata", "stacklocal", wantedOverrideCFNTemplatePath)
		wantedCFNBytes, err := os.ReadFile(path)
		require.NoError(t, err)

		require.Contains(t, tpl, string(wantedCFNBytes))
	})

	t.Run("CloudFormation template parameter values must match", func(t *testing.T) {
		params, err := serializer.SerializedParameters()
		require.NoError(t, err)

		path := filepath.Join("testdata", "stacklocal", wantedAutoScalingCFNParameterPath)
		wantedCFNParamsBytes, err := os.ReadFile(path)
		require.NoError(t, err)

		require.Equal(t, params, string(wantedCFNParamsBytes))
	})
}

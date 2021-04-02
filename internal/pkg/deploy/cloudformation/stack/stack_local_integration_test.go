// +build integration localintegration
// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/stretchr/testify/require"
)

const (
	manifestPath           = "autoscaling-svc-manifest.yml"
	wantedCFNTemplatePath  = "autoscaling-svc-cf.yml"
	wantedCFNParameterPath = "autoscaling-svc-cf.params.json"

	appName  = "my-app"
	envName  = "test"
	imageURL = "mockImageURL"
	imageTag = "latest"
)

func Test_Autoscaling_Integration(t *testing.T) {
	path := filepath.Join("testdata", "autoscaling", manifestPath)
	wantedManifestBytes, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	mft, err := manifest.UnmarshalWorkload(wantedManifestBytes)
	require.NoError(t, err)
	v, ok := mft.(*manifest.LoadBalancedWebService)
	require.Equal(t, ok, true)
	serializer, err := stack.NewHTTPSLoadBalancedWebService(v, envName, appName, stack.RuntimeConfig{
		Image: &stack.ECRImage{
			RepoURL:  imageURL,
			ImageTag: imageTag,
		},
	})
	require.NoError(t, err)
	tpl, err := serializer.Template()
	require.NoError(t, err)

	t.Run("CloudFormation template must contain autoscaling resources", func(t *testing.T) {
		path := filepath.Join("testdata", "autoscaling", wantedCFNTemplatePath)
		wantedCFNBytes, err := ioutil.ReadFile(path)
		require.NoError(t, err)

		require.Contains(t, tpl, string(wantedCFNBytes))
	})

	t.Run("CloudFormation template parameter values must match", func(t *testing.T) {
		params, err := serializer.SerializedParameters()
		require.NoError(t, err)

		path := filepath.Join("testdata", "autoscaling", wantedCFNParameterPath)
		wantedCFNParamsBytes, err := ioutil.ReadFile(path)
		require.NoError(t, err)

		require.Equal(t, params, string(wantedCFNParamsBytes))
	})
}

//go:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const (
	workerManifestPath = "worker-manifest.yml"
	workerStackPath    = "worker-test.stack.yml"
	workerParamsPath   = "worker-test.params.json"
)

func TestWorkerService_Template(t *testing.T) {
	path := filepath.Join("testdata", "workloads", workerManifestPath)
	manifestBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	mft, err := manifest.UnmarshalWorkload(manifestBytes)
	require.NoError(t, err)
	envMft, err := mft.ApplyEnv(envName)
	require.NoError(t, err)
	err = envMft.Validate()
	require.NoError(t, err)
	err = envMft.Load(session.New())
	require.NoError(t, err)
	content := envMft.Manifest()

	v, ok := content.(*manifest.WorkerService)
	require.True(t, ok)

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

	serializer, err := stack.NewWorkerService(stack.WorkerServiceConfig{
		App: &config.Application{
			Name: appName,
		},
		Env:                envName,
		Manifest:           v,
		ArtifactBucketName: "bucket",
		ArtifactKey:        "arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab",
		RawManifest:        string(manifestBytes),
		RuntimeConfig: stack.RuntimeConfig{
			ServiceDiscoveryEndpoint: "test.my-app.local",
			AccountID:                "123456789123",
			Region:                   "us-west-2",
			EnvVersion:               "v1.42.0",
			Version:                  "v1.29.0",
		},
	})

	tpl, err := serializer.Template()
	require.NoError(t, err, "template should render")
	regExpGUID := regexp.MustCompile(`([a-f\d]{8}-)([a-f\d]{4}-){3}([a-f\d]{12})`) // Matches random guids

	t.Run("CF Template should be equal", func(t *testing.T) {
		actualBytes := []byte(tpl)
		// Cut random GUID from template.
		actualBytes = regExpGUID.ReplaceAll(actualBytes, []byte("RandomGUID"))
		mActual := make(map[interface{}]interface{})
		require.NoError(t, yaml.Unmarshal(actualBytes, mActual))

		expected, err := os.ReadFile(filepath.Join("testdata", "workloads", workerStackPath))
		require.NoError(t, err, "should be able to read expected bytes")
		mExpected := make(map[interface{}]interface{})

		require.NoError(t, yaml.Unmarshal(expected, mExpected))

		resetCustomResourceLocations(mActual)
		compareStackTemplate(t, mExpected, mActual)
	})

	t.Run("Parameter values should render properly", func(t *testing.T) {
		actualParams, err := serializer.SerializedParameters()
		require.NoError(t, err)

		path := filepath.Join("testdata", "workloads", workerParamsPath)
		wantedCFNParamsBytes, err := os.ReadFile(path)
		require.NoError(t, err)

		require.Equal(t, string(wantedCFNParamsBytes), actualParams)
	})
}

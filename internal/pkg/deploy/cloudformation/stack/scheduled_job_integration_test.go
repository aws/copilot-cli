// +build integration localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/stretchr/testify/require"
)

const (
	jobManifestPath   = "job-manifest.yml"
	jobStackPath      = "job-test.stack.yml"
	jobParamsPath     = "job-test.params.json"
	envControllerPath = "custom-resources/env-controller.js"
)

func TestScheduledJob_Template(t *testing.T) {
	path := filepath.Join("testdata", "workloads", jobManifestPath)
	manifestBytes, err := ioutil.ReadFile(path)
	require.NoError(t, err)

	mft, err := manifest.UnmarshalWorkload(manifestBytes)
	require.NoError(t, err)

	envMft, err := mft.ApplyEnv(envName)
	require.NoError(t, err)

	err = mft.Validate()
	require.NoError(t, err)

	v, ok := envMft.(*manifest.ScheduledJob)
	require.True(t, ok)

	serializer, err := stack.NewScheduledJob(v, envName, appName, stack.RuntimeConfig{
		ServiceDiscoveryEndpoint: "test.my-app.local",
	})

	tpl, err := serializer.Template()
	require.NoError(t, err, "template should render")
	parser := template.New()
	envController, err := parser.Read(envControllerPath)
	require.NoError(t, err)
	zipFile := envController.String()
	t.Run("CF Template should be equal", func(t *testing.T) {
		actualBytes := []byte(tpl)
		actualString := string(actualBytes)
		// Cut out zip file from EnvControllerAction for more readable output
		actualString = strings.ReplaceAll(actualString, zipFile, "Abracadabra")
		actualBytes = []byte(actualString)
		mActual := make(map[interface{}]interface{})
		require.NoError(t, yaml.Unmarshal(actualBytes, mActual))

		expected, err := ioutil.ReadFile(filepath.Join("testdata", "workloads", jobStackPath))
		require.NoError(t, err, "should be able to read expected bytes")
		expectedBytes := []byte(expected)
		mExpected := make(map[interface{}]interface{})
		require.NoError(t, yaml.Unmarshal(expectedBytes, mExpected))
		// Cut out zip file from EnvControllerAction
		require.Equal(t, mExpected, mActual)
	})

	t.Run("Parameter values should render properly", func(t *testing.T) {
		actualParams, err := serializer.SerializedParameters()
		require.NoError(t, err)

		path := filepath.Join("testdata", "workloads", jobParamsPath)
		wantedCFNParamsBytes, err := ioutil.ReadFile(path)
		require.NoError(t, err)

		require.Equal(t, string(wantedCFNParamsBytes), actualParams)
	})

}

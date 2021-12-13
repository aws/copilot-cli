//go:build integration || localintegration
// +build integration localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/aws/copilot-cli/internal/pkg/manifest"

	"github.com/stretchr/testify/require"
)

const (
	workerManifestPath       = "worker-manifest.yml"
	workerStackPath          = "worker-test.stack.yml"
	workerParamsPath         = "worker-test.params.json"
	backlogPerTaskLambdaPath = "custom-resources/backlog-per-task-calculator.js"
)

func TestWorkerService_Template(t *testing.T) {
	path := filepath.Join("testdata", "workloads", workerManifestPath)
	manifestBytes, err := ioutil.ReadFile(path)
	require.NoError(t, err)

	mft, err := manifest.UnmarshalWorkload(manifestBytes)
	require.NoError(t, err)

	envMft, err := mft.ApplyEnv(envName)
	require.NoError(t, err)

	err = envMft.Validate()
	require.NoError(t, err)

	v, ok := envMft.(*manifest.WorkerService)
	require.True(t, ok)

	serializer, err := stack.NewWorkerService(v, envName, appName, stack.RuntimeConfig{
		ServiceDiscoveryEndpoint: "test.my-app.local",
		AccountID:                "123456789123",
		Region:                   "us-west-2",
	})

	tpl, err := serializer.Template()
	require.NoError(t, err, "template should render")
	regExpGUID := regexp.MustCompile(`([a-f\d]{8}-)([a-f\d]{4}-){3}([a-f\d]{12})`) // Matches random guids

	parser := template.New()
	envController, err := parser.Read(envControllerPath)
	require.NoError(t, err)
	envControllerZipFile := envController.String()

	backlogPerTaskLambda, err := parser.Read(backlogPerTaskLambdaPath)
	require.NoError(t, err)

	t.Run("CF Template should be equal", func(t *testing.T) {
		actualBytes := []byte(tpl)
		// Cut random GUID from template.
		actualBytes = regExpGUID.ReplaceAll(actualBytes, []byte("RandomGUID"))
		actualString := string(actualBytes)
		// Cut out zip file for more readable output
		actualString = strings.ReplaceAll(actualString, envControllerZipFile, "mockEnvControllerZipFile")
		actualString = strings.ReplaceAll(actualString, backlogPerTaskLambda.String(), "mockBacklogPerTaskLambda")
		actualBytes = []byte(actualString)
		mActual := make(map[interface{}]interface{})
		require.NoError(t, yaml.Unmarshal(actualBytes, mActual))

		expected, err := ioutil.ReadFile(filepath.Join("testdata", "workloads", workerStackPath))
		require.NoError(t, err, "should be able to read expected bytes")
		expectedBytes := []byte(expected)
		mExpected := make(map[interface{}]interface{})

		require.NoError(t, yaml.Unmarshal(expectedBytes, mExpected))
		require.Equal(t, mExpected, mActual)
	})

	t.Run("Parameter values should render properly", func(t *testing.T) {
		actualParams, err := serializer.SerializedParameters()
		require.NoError(t, err)

		path := filepath.Join("testdata", "workloads", workerParamsPath)
		wantedCFNParamsBytes, err := ioutil.ReadFile(path)
		require.NoError(t, err)

		require.Equal(t, string(wantedCFNParamsBytes), actualParams)
	})
}

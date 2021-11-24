//go:build integration || localintegration
// +build integration localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"gopkg.in/yaml.v3"

	"github.com/aws/copilot-cli/internal/pkg/manifest"

	"github.com/stretchr/testify/require"
)

const (
	windowsSvcManifestPath = "windows-svc-manifest.yml"
	windowsSvcStackPath    = "windows-svc-test.stack.yml"
	windowsSvcParamsPath   = "windows-svc-test.params.json"
)

func TestWindowsLoadBalancedWebService_Template(t *testing.T) {
	path := filepath.Join("testdata", "workloads", windowsSvcManifestPath)
	manifestBytes, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	mft, err := manifest.UnmarshalWorkload([]byte(manifestBytes))
	require.NoError(t, err)
	envMft, err := mft.ApplyEnv(envName)
	require.NoError(t, err)
	err = envMft.Validate()
	require.NoError(t, err)
	v, ok := envMft.(*manifest.LoadBalancedWebService)
	require.True(t, ok)

	svcDiscoveryEndpointName := fmt.Sprintf("%s.%s.local", envName, appName)
	serializer, err := stack.NewLoadBalancedWebService(v, envName, appName, stack.RuntimeConfig{
		ServiceDiscoveryEndpoint: svcDiscoveryEndpointName,
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
	rulePriority, err := parser.Read(rulePriorityPath)
	require.NoError(t, err)
	rulePriorityZipFile := rulePriority.String()

	t.Run("CF template should be equal", func(t *testing.T) {
		actualBytes := []byte(tpl)
		// Cut random GUID from template.
		actualBytes = regExpGUID.ReplaceAll(actualBytes, []byte("RandomGUID"))
		actualString := string(actualBytes)
		// Cut out zip file for more readable output
		actualString = strings.ReplaceAll(actualString, envControllerZipFile, "mockEnvControllerZipFile")
		actualString = strings.ReplaceAll(actualString, rulePriorityZipFile, "mockRulePriorityZipFile")

		actualBytes = []byte(actualString)
		mActual := make(map[interface{}]interface{})
		require.NoError(t, yaml.Unmarshal(actualBytes, mActual))

		expected, err := ioutil.ReadFile(filepath.Join("testdata", "workloads", windowsSvcStackPath))
		require.NoError(t, err, "should be able to read expected bytes")
		expectedBytes := []byte(expected)
		mExpected := make(map[interface{}]interface{})
		require.NoError(t, yaml.Unmarshal(expectedBytes, mExpected))
		require.Equal(t, mExpected, mActual)
	})

	t.Run("Parameter values should render properly", func(t *testing.T) {
		actualParams, err := serializer.SerializedParameters()
		require.NoError(t, err)

		path := filepath.Join("testdata", "workloads", windowsSvcParamsPath)
		wantedCFNParamsBytes, error := ioutil.ReadFile(path)
		require.NoError(t, error)

		require.Equal(t, string(wantedCFNParamsBytes), actualParams)
	})
}

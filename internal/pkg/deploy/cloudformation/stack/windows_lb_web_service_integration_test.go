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
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/golang/mock/gomock"
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	addons := mocks.NewMockaddons(ctrl)
	addons.EXPECT().Parameters().Return("", &addon.ErrAddonsNotFound{})
	addons.EXPECT().Template().Return("", &addon.ErrAddonsNotFound{})

	path := filepath.Join("testdata", "workloads", windowsSvcManifestPath)
	manifestBytes, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	mft, err := manifest.UnmarshalWorkload([]byte(manifestBytes))
	require.NoError(t, err)
	envMft, err := mft.ApplyEnv(envName)
	require.NoError(t, err)
	err = envMft.Validate()
	require.NoError(t, err)
	content := envMft.Manifest()

	v, ok := content.(*manifest.LoadBalancedWebService)
	require.True(t, ok)

	svcDiscoveryEndpointName := fmt.Sprintf("%s.%s.local", envName, appName)
	serializer, err := stack.NewLoadBalancedWebService(stack.LoadBalancedWebServiceConfig{
		App: &config.Application{Name: appName},
		EnvManifest: &manifest.Environment{
			Workload: manifest.Workload{
				Name: &envName,
			},
		},
		Manifest: v,
		RuntimeConfig: stack.RuntimeConfig{
			AccountID:                "123456789123",
			Region:                   "us-west-2",
			ServiceDiscoveryEndpoint: svcDiscoveryEndpointName,
		},
		Addons: addons,
	})

	tpl, err := serializer.Template()
	require.NoError(t, err, "template should render")
	regExpGUID := regexp.MustCompile(`([a-f\d]{8}-)([a-f\d]{4}-){3}([a-f\d]{12})`) // Matches random guids

	t.Run("CF template should be equal", func(t *testing.T) {
		actualBytes := []byte(tpl)
		// Cut random GUID from template.
		actualBytes = regExpGUID.ReplaceAll(actualBytes, []byte("RandomGUID"))
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

//go:build integration || localintegration || temporary
// +build integration localintegration temporary

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/aws/copilot-cli/internal/pkg/manifest"

	"github.com/stretchr/testify/require"
)

const (
	nlbSvcManifestPath = "svc-nlb-manifest.yml"

	dynamicDesiredCountPath = "custom-resources/desired-count-delegation.js"
	rulePriorityPath        = "custom-resources/alb-rule-priority-generator.js"
)

const (
	envControllerPath = "custom-resources/env-controller.js"
	appName           = "my-app"
)

func TestNetworkLoadBalancedWebService_Template(t *testing.T) {
	testCases := map[string]struct {
		envName       string
		svcStackPath  string
		svcParamsPath string
	}{
		"test env": {
			envName:       "test",
			svcStackPath:  "svc-nlb-test.stack.yml",
			svcParamsPath: "svc-nlb-test.params.json",
		},
		"dev env": {
			envName:       "dev",
			svcStackPath:  "svc-nlb-dev.stack.yml",
			svcParamsPath: "svc-nlb-dev.params.json",
		},
		"prod env": {
			envName:       "prod",
			svcStackPath:  "svc-nlb-prod.stack.yml",
			svcParamsPath: "svc-nlb-prod.params.json",
		},
	}
	val, exist := os.LookupEnv("TAG")
	require.NoError(t, os.Setenv("TAG", "cicdtest"))
	defer func() {
		if !exist {
			require.NoError(t, os.Unsetenv("TAG"))
			return
		}
		require.NoError(t, os.Setenv("TAG", val))
	}()
	path := filepath.Join("testdata", "workloads", nlbSvcManifestPath)
	manifestBytes, err := ioutil.ReadFile(path)
	require.NoError(t, err)
	for name, tc := range testCases {
		interpolated, err := manifest.NewInterpolator(appName, tc.envName).Interpolate(string(manifestBytes))
		require.NoError(t, err)
		mft, err := manifest.UnmarshalWorkload([]byte(interpolated))
		require.NoError(t, err)
		envMft, err := mft.ApplyEnv(tc.envName)
		require.NoError(t, err)

		err = envMft.Validate()
		require.NoError(t, err)

		v, ok := envMft.(*manifest.LoadBalancedWebService)
		require.True(t, ok)

		svcDiscoveryEndpointName := fmt.Sprintf("%s.%s.local", tc.envName, appName)

		serializer, err := stack.NewLoadBalancedWebService(v, tc.envName, appName, stack.RuntimeConfig{
			ServiceDiscoveryEndpoint: svcDiscoveryEndpointName,
			AccountID:                "123456789123",
			Region:                   "us-west-2",
		}, stack.WithNLB([]string{"10.0.0.0/24", "10.1.0.0/24"}), stack.WithHTTPS(), stack.WithDNSDelegation(deploy.AppInformation{
			AccountPrincipalARN: "arn:aws:iam::123456789123:root",
			DNSName:             "example.com",
			Name:                appName,
		}))
		tpl, err := serializer.Template()
		require.NoError(t, err, "template should render")
		regExpGUID := regexp.MustCompile(`([a-f\d]{8}-)([a-f\d]{4}-){3}([a-f\d]{12})`) // Matches random guids
		testName := fmt.Sprintf("CF Template should be equal/%s", name)
		parser := template.New()
		envController, err := parser.Read(envControllerPath)
		require.NoError(t, err)
		envControllerZipFile := envController.String()
		dynamicDesiredCount, err := parser.Read(dynamicDesiredCountPath)
		require.NoError(t, err)
		dynamicDesiredCountZipFile := dynamicDesiredCount.String()
		rulePriority, err := parser.Read(rulePriorityPath)
		require.NoError(t, err)
		rulePriorityZipFile := rulePriority.String()

		t.Run(testName, func(t *testing.T) {
			actualBytes := []byte(tpl)
			// Cut random GUID from template.
			actualBytes = regExpGUID.ReplaceAll(actualBytes, []byte("RandomGUID"))
			actualString := string(actualBytes)
			// Cut out zip file for more readable output
			actualString = strings.ReplaceAll(actualString, envControllerZipFile, "mockEnvControllerZipFile")
			actualString = strings.ReplaceAll(actualString, dynamicDesiredCountZipFile, "mockDynamicDesiredCountZipFile")
			actualString = strings.ReplaceAll(actualString, rulePriorityZipFile, "mockRulePriorityZipFile")

			actualBytes = []byte(actualString)
			mActual := make(map[interface{}]interface{})
			require.NoError(t, yaml.Unmarshal(actualBytes, mActual))

			expected, err := ioutil.ReadFile(filepath.Join("testdata", "workloads", tc.svcStackPath))
			require.NoError(t, err, "should be able to read expected bytes")
			expectedBytes := []byte(expected)
			mExpected := make(map[interface{}]interface{})
			require.NoError(t, yaml.Unmarshal(expectedBytes, mExpected))
			require.Equal(t, mExpected, mActual)
		})

		testName = fmt.Sprintf("Parameter values should render properly/%s", name)
		t.Run(testName, func(t *testing.T) {
			actualParams, err := serializer.SerializedParameters()
			require.NoError(t, err)

			path := filepath.Join("testdata", "workloads", tc.svcParamsPath)
			wantedCFNParamsBytes, err := ioutil.ReadFile(path)
			require.NoError(t, err)

			require.Equal(t, string(wantedCFNParamsBytes), actualParams)
		})
	}
}

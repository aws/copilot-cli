//go:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestBackendService_TemplateAndParamsGeneration(t *testing.T) {
	const (
		appName = "my-app"
	)
	envName := "my-env"

	testDir := filepath.Join("testdata", "workloads", "backend")

	tests := map[string]struct {
		ManifestPath        string
		TemplatePath        string
		ParamsPath          string
		EnvImportedCertARNs []string
	}{
		"simple": {
			ManifestPath: filepath.Join(testDir, "simple-manifest.yml"),
			TemplatePath: filepath.Join(testDir, "simple-template.yml"),
			ParamsPath:   filepath.Join(testDir, "simple-params.json"),
		},
		"simple without port config": {
			ManifestPath: filepath.Join(testDir, "simple-manifest-without-port-config.yml"),
			TemplatePath: filepath.Join(testDir, "simple-template-without-port-config.yml"),
			ParamsPath:   filepath.Join(testDir, "simple-params-without-port-config.json"),
		},
		"http only path configured": {
			ManifestPath: filepath.Join(testDir, "http-only-path-manifest.yml"),
			TemplatePath: filepath.Join(testDir, "http-only-path-template.yml"),
			ParamsPath:   filepath.Join(testDir, "http-only-path-params.json"),
		},
		"http full config": {
			ManifestPath: filepath.Join(testDir, "http-full-config-manifest.yml"),
			TemplatePath: filepath.Join(testDir, "http-full-config-template.yml"),
			ParamsPath:   filepath.Join(testDir, "http-full-config-params.json"),
		},
		"https path and alias configured": {
			ManifestPath:        filepath.Join(testDir, "https-path-alias-manifest.yml"),
			TemplatePath:        filepath.Join(testDir, "https-path-alias-template.yml"),
			ParamsPath:          filepath.Join(testDir, "https-path-alias-params.json"),
			EnvImportedCertARNs: []string{"exampleComCertARN"},
		},
		"http with autoscaling by requests configured": {
			ManifestPath: filepath.Join(testDir, "http-autoscaling-manifest.yml"),
			TemplatePath: filepath.Join(testDir, "http-autoscaling-template.yml"),
			ParamsPath:   filepath.Join(testDir, "http-autoscaling-params.json"),
		},
	}

	// run tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// parse files
			manifestBytes, err := os.ReadFile(tc.ManifestPath)
			require.NoError(t, err)
			tmplBytes, err := os.ReadFile(tc.TemplatePath)
			require.NoError(t, err)
			paramsBytes, err := os.ReadFile(tc.ParamsPath)
			require.NoError(t, err)

			dynamicMft, err := manifest.UnmarshalWorkload([]byte(manifestBytes))
			require.NoError(t, err)
			require.NoError(t, dynamicMft.Validate())
			mft := dynamicMft.Manifest()

			envConfig := &manifest.Environment{
				Workload: manifest.Workload{
					Name: &envName,
				},
			}
			envConfig.HTTPConfig.Private.Certificates = tc.EnvImportedCertARNs
			serializer, err := stack.NewBackendService(stack.BackendServiceConfig{
				App: &config.Application{
					Name: appName,
				},
				EnvManifest:        envConfig,
				ArtifactBucketName: "bucket",
				ArtifactKey:        "arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab",
				Manifest:           mft.(*manifest.BackendService),
				RuntimeConfig: stack.RuntimeConfig{
					ServiceDiscoveryEndpoint: fmt.Sprintf("%s.%s.local", envName, appName),
					EnvVersion:               "v1.42.0",
					Version:                  "v1.29.0",
				},
			})
			require.NoError(t, err)

			// validate generated template
			tmpl, err := serializer.Template()
			require.NoError(t, err)
			var actualTmpl map[any]any
			require.NoError(t, yaml.Unmarshal([]byte(tmpl), &actualTmpl))

			// change the random DynamicDesiredCountAction UpdateID to an expected value
			if v, ok := actualTmpl["Resources"]; ok {
				if v, ok := v.(map[string]any)["DynamicDesiredCountAction"]; ok {
					if v, ok := v.(map[string]any)["Properties"]; ok {
						if v, ok := v.(map[string]any); ok {
							v["UpdateID"] = "AVeryRandomUUID"
						}
					}
				}
			}
			resetCustomResourceLocations(actualTmpl)

			var expectedTmpl map[any]any
			require.NoError(t, yaml.Unmarshal(tmplBytes, &expectedTmpl))
			compareStackTemplate(t, expectedTmpl, actualTmpl)

			// validate generated params
			params, err := serializer.SerializedParameters()
			require.NoError(t, err)
			var actualParams map[string]any
			require.NoError(t, json.Unmarshal([]byte(params), &actualParams))

			var expectedParams map[string]any
			require.NoError(t, json.Unmarshal(paramsBytes, &expectedParams))

			require.Equal(t, expectedParams, actualParams, "param mismatch")
		})
	}
}

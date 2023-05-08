//go:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"fmt"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

func TestStaticSiteService_TemplateAndParamsGeneration(t *testing.T) {
	const (
		appName = "my-app"
	)
	envName := "my-env"

	testDir := filepath.Join("testdata", "workloads")

	tests := map[string]struct {
		ManifestPath        string
		TemplatePath        string
		ParamsPath          string
		EnvImportedCertARNs []string
	}{
		"simple": {
			ManifestPath: filepath.Join(testDir, "static-site-manifest.yml"),
			TemplatePath: filepath.Join(testDir, "static-site.stack.yml"),
			ParamsPath:   filepath.Join(testDir, "static-site.params.json"),
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

			// Create in-memory mock file system.
			wd, err := os.Getwd()
			require.NoError(t, err)
			fs := afero.NewMemMapFs()
			_ = fs.MkdirAll(fmt.Sprintf("%s/foo", wd), 0755)
			_ = afero.WriteFile(fs, fmt.Sprintf("%s/frontend/dist", wd), []byte("good stuff"), 0644)
			require.NoError(t, err)

			serializer, err := stack.NewStaticSite(&stack.StaticSiteConfig{
				App: &config.Application{
					Name: appName,
				},
				EnvManifest: envConfig,
				Manifest:    mft.(*manifest.StaticSite),
				RuntimeConfig: stack.RuntimeConfig{
					EnvVersion: "v1.42.0",
				},
				AssetMappingURL: "s3://stackset-bucket/mappingfile",
			})
			require.NoError(t, err)
			// validate generated template
			tmpl, err := serializer.Template()
			require.NoError(t, err)
			var actualTmpl map[any]any
			require.NoError(t, yaml.Unmarshal([]byte(tmpl), &actualTmpl))

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

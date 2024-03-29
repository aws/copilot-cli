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

func TestStaticSiteService_TemplateAndParamsGeneration(t *testing.T) {
	const manifestPath = "static-site-manifest.yml"
	testCases := map[string]struct {
		envName       string
		svcStackPath  string
		svcParamsPath string
	}{
		"simple": {
			envName:       "my-env",
			svcStackPath:  "static-site.stack.yml",
			svcParamsPath: "static-site.params.json",
		},
		"test": {
			envName:       "test",
			svcStackPath:  "static-site-test.stack.yml",
			svcParamsPath: "static-site-test.params.json",
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
	path := filepath.Join("testdata", "workloads", manifestPath)
	manifestBytes, err := os.ReadFile(path)
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
		err = envMft.Load(session.New())
		require.NoError(t, err)
		content := envMft.Manifest()

		v, ok := content.(*manifest.StaticSite)
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

		envConfig := &manifest.Environment{
			Workload: manifest.Workload{
				Name: &tc.envName,
			},
		}
		serializer, err := stack.NewStaticSite(&stack.StaticSiteConfig{
			App: &config.Application{
				Name:   appName,
				Domain: "example.com",
			},
			EnvManifest: envConfig,
			Manifest:    v,
			RuntimeConfig: stack.RuntimeConfig{
				EnvVersion: "v1.42.0",
				Version:    "v1.29.0",
				Region:     "us-west-2",
			},
			ArtifactBucketName: "stackset-bucket",
			ArtifactKey:        "arn:aws:kms:us-west-2:111122223333:key/1234abcd-12ab-34cd-56ef-1234567890ab",
			AssetMappingURL:    "s3://stackset-bucket/mappingfile",
			RootUserARN:        "arn:aws:iam::123456789123:root",
		})
		require.NoError(t, err, "stack should be able to be initialized")
		tpl, err := serializer.Template()
		require.NoError(t, err, "template should render")
		testName := fmt.Sprintf("CF Template should be equal/%s", name)

		t.Run(testName, func(t *testing.T) {
			actualBytes := []byte(tpl)
			mActual := make(map[interface{}]interface{})
			require.NoError(t, yaml.Unmarshal(actualBytes, mActual))

			expected, err := os.ReadFile(filepath.Join("testdata", "workloads", tc.svcStackPath))
			require.NoError(t, err, "should be able to read expected bytes")
			expectedBytes := []byte(expected)
			mExpected := make(map[interface{}]interface{})
			require.NoError(t, yaml.Unmarshal(expectedBytes, mExpected))

			resetCustomResourceLocations(mActual)
			compareStackTemplate(t, mExpected, mActual)
		})

		testName = fmt.Sprintf("Parameter values should render properly/%s", name)
		t.Run(testName, func(t *testing.T) {
			actualParams, err := serializer.SerializedParameters()
			require.NoError(t, err)

			path := filepath.Join("testdata", "workloads", tc.svcParamsPath)
			wantedCFNParamsBytes, err := os.ReadFile(path)
			require.NoError(t, err)

			require.Equal(t, string(wantedCFNParamsBytes), actualParams)
		})
	}
}

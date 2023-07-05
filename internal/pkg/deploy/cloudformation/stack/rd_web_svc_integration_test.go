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
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRDWS_Template(t *testing.T) {
	const manifestFileName = "rdws-manifest.yml"
	testCases := map[string]struct {
		envName      string
		svcStackPath string
	}{
		"test env": {
			envName:      "test",
			svcStackPath: "rdws-test.stack.yml",
		},
		"prod env": {
			envName:      "prod",
			svcStackPath: "rdws-prod.stack.yml",
		},
	}

	// Read manifest.
	manifestBytes, err := os.ReadFile(filepath.Join("testdata", "workloads", manifestFileName))
	require.NoError(t, err, "read manifest file")
	mft, err := manifest.UnmarshalWorkload(manifestBytes)
	require.NoError(t, err, "unmarshal manifest file")
	for _, tc := range testCases {
		envMft, err := mft.ApplyEnv(tc.envName)
		require.NoError(t, err, "apply test env to manifest")
		err = envMft.Validate()
		require.NoError(t, err)
		err = envMft.Load(session.New())
		require.NoError(t, err)
		content := envMft.Manifest()

		v, ok := content.(*manifest.RequestDrivenWebService)
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

		// Read wanted stack template.
		wantedTemplate, err := os.ReadFile(filepath.Join("testdata", "workloads", tc.svcStackPath))
		require.NoError(t, err, "read cloudformation stack")

		// Read actual stack template.
		serializer, err := stack.NewRequestDrivenWebService(stack.RequestDrivenWebServiceConfig{
			App: deploy.AppInformation{
				Name: appName,
			},
			Env:                tc.envName,
			Manifest:           v,
			ArtifactBucketName: "bucket",
			RuntimeConfig: stack.RuntimeConfig{
				AccountID:  "123456789123",
				Region:     "us-west-2",
				EnvVersion: "v1.42.0",
				Version:    "v1.29.0",
			},
		})
		require.NoError(t, err, "create rdws serializer")
		actualTemplate, err := serializer.Template()
		require.NoError(t, err, "get cloudformation template for rdws")
		// Compare the two.
		wanted := make(map[interface{}]interface{})
		require.NoError(t, yaml.Unmarshal(wantedTemplate, wanted), "unmarshal wanted template to map[interface{}]interface{}")
		actual := make(map[interface{}]interface{})
		require.NoError(t, yaml.Unmarshal([]byte(actualTemplate), actual), "unmarshal actual template to map[interface{}]interface{}")

		resetCustomResourceLocations(actual)
		compareStackTemplate(t, wanted, actual)
	}
}

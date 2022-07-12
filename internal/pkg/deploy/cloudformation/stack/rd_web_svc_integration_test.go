//go:build integration || localintegration
// +build integration localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack_test

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/aws/copilot-cli/internal/pkg/addon"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
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
	manifestBytes, err := ioutil.ReadFile(filepath.Join("testdata", "workloads", manifestFileName))
	require.NoError(t, err, "read manifest file")

	mft, err := manifest.UnmarshalWorkload(manifestBytes)
	require.NoError(t, err, "unmarshal manifest file")
	for _, tc := range testCases {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		addons := mocks.NewMockaddons(ctrl)
		addons.EXPECT().Parameters().Return("", &addon.ErrAddonsNotFound{})
		addons.EXPECT().Template().Return("", &addon.ErrAddonsNotFound{})

		envMft, err := mft.ApplyEnv(tc.envName)
		require.NoError(t, err, "apply test env to manifest")

		err = envMft.Validate()
		require.NoError(t, err)

		v, ok := envMft.(*manifest.RequestDrivenWebService)
		require.True(t, ok)

		// Read wanted stack template.
		wantedTemplate, err := ioutil.ReadFile(filepath.Join("testdata", "workloads", tc.svcStackPath))
		require.NoError(t, err, "read cloudformation stack")

		// Read actual stack template.
		serializer, err := stack.NewRequestDrivenWebService(stack.RequestDrivenWebServiceConfig{
			App: deploy.AppInformation{
				Name: appName,
			},
			Env:      tc.envName,
			Manifest: v,
			RuntimeConfig: stack.RuntimeConfig{
				AccountID: "123456789123",
				Region:    "us-west-2",
			},
			Addons: addons,
		})
		require.NoError(t, err, "create rdws serializer")
		actualTemplate, err := serializer.Template()
		require.NoError(t, err, "get cloudformation template for rdws")

		// Compare the two.
		wanted := make(map[interface{}]interface{})
		require.NoError(t, yaml.Unmarshal(wantedTemplate, wanted), "unmarshal wanted template to map[interface{}]interface{}")

		actual := make(map[interface{}]interface{})
		require.NoError(t, yaml.Unmarshal([]byte(actualTemplate), actual), "unmarshal actual template to map[interface{}]interface{}")

		require.Equal(t, wanted, actual, "templates do not match")
	}
}

//gox:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/template"
)

func TestBackendService_Template_Integ(t *testing.T) {
	const (
		appName = "my-app"
		envName = "my-env"
		// accountID = "123456789123"
		// region    = "us-west-2"
		manifestSuffix = "-manifest.yml"
		stackSuffix    = "-stack.yml"
		paramsSuffix   = "-params.json"
	)

	// discover test cases
	tests := make(map[string]string) // name -> path prefix
	dir := filepath.Join("testdata", "workloads", "backend")
	filepath.Walk(dir, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}

		if strings.HasSuffix(info.Name(), manifestSuffix) {
			name := strings.TrimSuffix(info.Name(), manifestSuffix)
			tests[name] = strings.TrimSuffix(path, manifestSuffix)
		}

		return nil
	})

	t.Logf("tests: %#v", tests)

	for name, pathPrefix := range tests {
	}

	/*
		testCases := map[string]struct {
			envName      string
			manifestPath string
			stackPath    string
		}{
			"simple": {
				envName:      envName,
				manifestPath: "simple-manifest.yml",
				stackPath:    "simple-stack.yml",
			},
		}

		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				defer ctrl.Finish()

				path := filepath.Join("testdata", "workloads", "backend", tc.manifestPath)
				manifestBytes, err := ioutil.ReadFile(path)
				require.NoError(t, err)

				mft, err := manifest.UnmarshalWorkload([]byte(manifestBytes))
				require.NoError(t, err)

				mft, err = mft.ApplyEnv(tc.envName)
				require.NoError(t, err)

				require.NoError(t, mft.Validate())

				svcMft, ok := mft.(*manifest.BackendService)
				require.True(t, ok)

				svcDiscoveryEndpointName := fmt.Sprintf("%s.%s.local", tc.envName, appName)
				serializer, err := NewBackendService(svcMft, tc.envName, appName,
					RuntimeConfig{
						ServiceDiscoveryEndpoint: svcDiscoveryEndpointName,
						// AccountID:                accountID,
						// Region:                   region,
					})
				require.NoError(t, err)

				realParser := serializer.parser

				mockParser := mocks.NewMockbackendSvcReadParser(ctrl)
				mockParser.EXPECT().Read(albRulePriorityGeneratorPath).Return(newTemplateContent("albRulePriorityGenerator"), nil)
				mockParser.EXPECT().Read(desiredCountGeneratorPath).Return(newTemplateContent("desiredCountGenerator"), nil)
				mockParser.EXPECT().Read(envControllerPath).Return(newTemplateContent("envController"), nil)
				mockParser.EXPECT().ParseBackendService(gomock.Any()).DoAndReturn(func(data template.WorkloadOpts) (*template.Content, error) {
					return realParser.ParseBackendService(data)
				})

				serializer.parser = mockParser

				tmpl, err := serializer.Template()
				require.NoError(t, err)
				var actualYaml map[any]any
				require.NoError(t, yaml.Unmarshal([]byte(tmpl), &actualYaml))

				path = filepath.Join("testdata", "workloads", "backend", tc.stackPath)
				expectedBytes, err := ioutil.ReadFile(path)
				require.NoError(t, err)
				var expectedYaml map[any]any
				require.NoError(t, yaml.Unmarshal(expectedBytes, &expectedYaml))

				require.Equal(t, expectedYaml, actualYaml)

			})
		}
	*/
}

func newTemplateContent(str string) *template.Content {
	return &template.Content{
		Buffer: bytes.NewBuffer([]byte(str)),
	}
}

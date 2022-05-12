//gox:build integration || localintegration

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"fmt"
	"io/fs"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestBackendService_Template_Integ(t *testing.T) {
	const (
		appName = "my-app"
		envName = "my-env"
		// accountID = "123456789123"
		// region    = "us-west-2"
		manifestSuffix = "-manifest.yml"
		templateSuffix = "-stack.yml"
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

	for name, pathPrefix := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// parse files
			manifestBytes, err := ioutil.ReadFile(pathPrefix + manifestSuffix)
			require.NoError(t, err)
			tmplBytes, err := ioutil.ReadFile(pathPrefix + templateSuffix)
			require.NoError(t, err)

			mft, err := manifest.UnmarshalWorkload([]byte(manifestBytes))
			require.NoError(t, err)
			require.NoError(t, mft.Validate())

			serializer, err := NewBackendService(mft.(*manifest.BackendService), envName, appName,
				RuntimeConfig{
					ServiceDiscoveryEndpoint: fmt.Sprintf("%s.%s.local", envName, appName),
					// AccountID:                accountID,
					// Region:                   region,
				})
			require.NoError(t, err)

			// mock parser, but use real parser for parsing backend service
			realParser := serializer.parser
			mockParser := mocks.NewMockbackendSvcReadParser(ctrl)
			mockParser.EXPECT().Read(albRulePriorityGeneratorPath).Return(newTemplateContent("albRulePriorityGenerator"), nil)
			mockParser.EXPECT().Read(desiredCountGeneratorPath).Return(newTemplateContent("desiredCountGenerator"), nil)
			mockParser.EXPECT().Read(envControllerPath).Return(newTemplateContent("envController"), nil)
			mockParser.EXPECT().ParseBackendService(gomock.Any()).DoAndReturn(func(data template.WorkloadOpts) (*template.Content, error) {
				return realParser.ParseBackendService(data)
			})
			serializer.parser = mockParser

			// generate cf template
			tmpl, err := serializer.Template()
			require.NoError(t, err)
			var actualTmpl map[any]any
			require.NoError(t, yaml.Unmarshal([]byte(tmpl), &actualTmpl))

			var expectedTmpl map[any]any
			require.NoError(t, yaml.Unmarshal(tmplBytes, &expectedTmpl))

			require.Equal(t, expectedTmpl, actualTmpl)
		})
	}
}

func newTemplateContent(str string) *template.Content {
	return &template.Content{
		Buffer: bytes.NewBuffer([]byte(str)),
	}
}

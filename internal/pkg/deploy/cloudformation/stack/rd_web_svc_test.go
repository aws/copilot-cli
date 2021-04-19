// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack/mocks"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var testRDWebServiceManifest = &manifest.RequestDrivenWebService{
	Workload: manifest.Workload{
		Name: aws.String(testServiceName),
		Type: aws.String(manifest.RequestDrivenWebServiceType),
	},
	RequestDrivenWebServiceConfig: manifest.RequestDrivenWebServiceConfig{
		ImageConfig: manifest.ImageWithPort{
			Port: aws.Uint16(80),
		},
		InstanceConfig: manifest.AppRunnerInstanceConfig{
			CPU:    aws.Int(256),
			Memory: aws.Int(512),
		},
		Variables: map[string]string{
			"LOG_LEVEL": "info",
			"NODE_ENV":  "development",
		},
		Tags: map[string]string{
			"owner": "jeff",
		},
	},
}

func TestRequestDrivenWebService_NewRequestDrivenWebService(t *testing.T) {
	type testInput struct {
		mft *manifest.RequestDrivenWebService
		env string
		app string
		rc  RuntimeConfig
	}

	testCases := map[string]struct {
		input            testInput
		mockDependencies func(t *testing.T, ctrl *gomock.Controller, c *RequestDrivenWebService)

		wantedStack *RequestDrivenWebService
		wantedError error
	}{
		"should return RequestDrivenWebService": {
			input: testInput{
				mft: testRDWebServiceManifest,
				env: testEnvName,
				app: testAppName,
				rc:  RuntimeConfig{},
			},

			wantedStack: &RequestDrivenWebService{
				appRunnerWkld: &appRunnerWkld{
					wkld: &wkld{
						name:  aws.StringValue(testRDWebServiceManifest.Name),
						env:   testEnvName,
						app:   testAppName,
						rc:    RuntimeConfig{},
						image: testRDWebServiceManifest.ImageConfig,
					},
					instanceConfig: testRDWebServiceManifest.InstanceConfig,
					imageConfig:    testRDWebServiceManifest.ImageConfig,
				},
				manifest: testRDWebServiceManifest,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			stack, err := NewRequestDrivenWebService(
				tc.input.mft,
				tc.input.env,
				tc.input.app,
				tc.input.rc,
			)

			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedStack.name, stack.name)
			require.Equal(t, tc.wantedStack.env, stack.env)
			require.Equal(t, tc.wantedStack.app, stack.app)
			require.Equal(t, tc.wantedStack.rc, stack.rc)
			require.Equal(t, tc.wantedStack.image, stack.image)
			require.Equal(t, tc.wantedStack.manifest, stack.manifest)
			require.Equal(t, tc.wantedStack.instanceConfig, stack.instanceConfig)
			require.Equal(t, tc.wantedStack.imageConfig, stack.imageConfig)
			require.NotNil(t, stack.addons)
			require.NotNil(t, stack.parser)
		})
	}
}

func TestRequestDrivenWebService_Template(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(t *testing.T, ctrl *gomock.Controller, c *RequestDrivenWebService)
		wantedTemplate   string
		wantedError      error
	}{
		"should parse template": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *RequestDrivenWebService) {
				mockParser := mocks.NewMockrequestDrivenWebSvcReadParser(ctrl)
				mockParser.EXPECT().ParseRequestDrivenWebService(template.WorkloadOpts{
					Variables: c.manifest.Variables,
					Tags:      c.manifest.Tags,
				}).Return(&template.Content{Buffer: bytes.NewBufferString("template")}, nil)
				c.parser = mockParser
			},
			wantedTemplate: "template",
		},
		"should return parsing error": {
			mockDependencies: func(t *testing.T, ctrl *gomock.Controller, c *RequestDrivenWebService) {
				mockParser := mocks.NewMockrequestDrivenWebSvcReadParser(ctrl)
				mockParser.EXPECT().ParseRequestDrivenWebService(template.WorkloadOpts{
					Variables: c.manifest.Variables,
					Tags:      c.manifest.Tags,
				}).Return(nil, errors.New("parsing error"))
				c.parser = mockParser
			},
			wantedError: errors.New("parsing error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			conf := &RequestDrivenWebService{
				appRunnerWkld: &appRunnerWkld{
					wkld: &wkld{
						name: testServiceName,
						env:  testEnvName,
						app:  testAppName,
						rc: RuntimeConfig{
							Image: &ECRImage{
								RepoURL:  testImageRepoURL,
								ImageTag: testImageTag,
							},
						},
					},
				},
				manifest: testRDWebServiceManifest,
			}
			tc.mockDependencies(t, ctrl, conf)

			// WHEN
			template, err := conf.Template()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
				require.Equal(t, "", template)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedTemplate, template)
			}
		})
	}
}

func TestRequestDrivenWebService_SerializedParameters(t *testing.T) {
	testCases := map[string]struct {
		mockDependencies func(ctrl *gomock.Controller, c *RequestDrivenWebService)

		wantedParams string
		wantedError  error
	}{
		"unavailable template": {
			mockDependencies: func(ctrl *gomock.Controller, c *RequestDrivenWebService) {
				m := mocks.NewMockrequestDrivenWebSvcReadParser(ctrl)
				m.EXPECT().Parse(wkldParamsTemplatePath, gomock.Any(), gomock.Any()).Return(nil, errors.New("serialization error"))
				c.wkld.parser = m
			},
			wantedParams: "",
			wantedError:  errors.New("serialization error"),
		},
		"render params template": {
			mockDependencies: func(ctrl *gomock.Controller, c *RequestDrivenWebService) {
				m := mocks.NewMockrequestDrivenWebSvcReadParser(ctrl)
				m.EXPECT().Parse(wkldParamsTemplatePath, gomock.Any(), gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("params")}, nil)
				c.wkld.parser = m
			},
			wantedParams: "params",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := &RequestDrivenWebService{
				appRunnerWkld: &appRunnerWkld{
					wkld: &wkld{
						name: aws.StringValue(testRDWebServiceManifest.Name),
						env:  testEnvName,
						app:  testAppName,
						rc: RuntimeConfig{
							Image: &ECRImage{
								RepoURL:  testImageRepoURL,
								ImageTag: testImageTag,
							},
						},
					},
					instanceConfig: testRDWebServiceManifest.InstanceConfig,
					imageConfig:    testRDWebServiceManifest.ImageConfig,
				},
				manifest: testRDWebServiceManifest,
			}
			tc.mockDependencies(ctrl, c)

			// WHEN
			params, err := c.SerializedParameters()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedParams, params)
		})
	}
}

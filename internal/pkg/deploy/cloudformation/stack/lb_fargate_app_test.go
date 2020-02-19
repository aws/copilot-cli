// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package stack

import (
	"bytes"
	"errors"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestLBFargateStackConfig_StackName(t *testing.T) {
	testCases := map[string]struct {
		inAppName     string
		inEnvName     string
		inProjectName string

		wantedStackName string
	}{
		"valid stack name": {
			inAppName:     "frontend",
			inEnvName:     "test",
			inProjectName: "phonetool",

			wantedStackName: "phonetool-test-frontend",
		},
		"longer than 128 characters": {
			inAppName:     "whatisthishorriblylongapplicationnamethatcantfitintocloudformationwhatarewesupposedtodoaboutthisaaaaaaaaaaaaaaaaaaaa",
			inEnvName:     "test",
			inProjectName: "phonetool",

			wantedStackName: "phonetool-test-whatisthishorriblylongapplicationnamethatcantfitintocloudformationwhatarewesupposedtodoaboutthisaaaaaaaaaaaaaaaaa",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			conf := &LBFargateStackConfig{
				CreateLBFargateAppInput: &deploy.CreateLBFargateAppInput{
					App: &manifest.LBFargateManifest{
						AppManifest: manifest.AppManifest{
							Name: tc.inAppName,
						},
					},
					Env: &archer.Environment{
						Project: tc.inProjectName,
						Name:    tc.inEnvName,
					},
				},
			}

			// WHEN
			n := conf.StackName()

			// THEN
			require.Equal(t, tc.wantedStackName, n, "expected stack names to be equal")
		})
	}
}

func TestLBFargateStackConfig_Template(t *testing.T) {
	testCases := map[string]struct {
		in               *deploy.CreateLBFargateAppInput
		mockDependencies func(ctrl *gomock.Controller, c *LBFargateStackConfig)

		wantedTemplate string
		wantedError    error
	}{
		"unavailable rule priority lambda template": {
			mockDependencies: func(ctrl *gomock.Controller, c *LBFargateStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Read(lbFargateAppRulePriorityGeneratorPath).Return(nil, errors.New("some error"))
				c.parser = m
			},
			wantedTemplate: "",
			wantedError:    errors.New("some error"),
		},
		"failed parsing app template": {
			in: &deploy.CreateLBFargateAppInput{
				App: manifest.NewLoadBalancedFargateManifest(&manifest.LBFargateManifestProps{
					AppManifestProps: &manifest.AppManifestProps{
						AppName:    "frontend",
						Dockerfile: "frontend/Dockerfile",
					},
					Path: "frontend",
				}),
				Env: &archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					Region:    "us-west-2",
					AccountID: "12345",
					Prod:      false,
				},
				ImageRepoURL: "12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend",
				ImageTag:     "manual-bf3678c",
			},

			mockDependencies: func(ctrl *gomock.Controller, c *LBFargateStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Read(lbFargateAppRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("something")}, nil)
				m.EXPECT().Parse(lbFargateAppTemplatePath, gomock.Any()).Return(nil, errors.New("some error"))
				c.parser = m
			},

			wantedTemplate: "",
			wantedError:    errors.New("some error"),
		},
		"render default template": {
			in: &deploy.CreateLBFargateAppInput{
				App: manifest.NewLoadBalancedFargateManifest(&manifest.LBFargateManifestProps{
					AppManifestProps: &manifest.AppManifestProps{
						AppName:    "frontend",
						Dockerfile: "frontend/Dockerfile",
					},
					Path: "frontend",
				}),
				Env: &archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					Region:    "us-west-2",
					AccountID: "12345",
					Prod:      false,
				},
				ImageRepoURL: "12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend",
				ImageTag:     "manual-bf3678c",
			},
			mockDependencies: func(ctrl *gomock.Controller, c *LBFargateStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Read(lbFargateAppRulePriorityGeneratorPath).Return(&template.Content{Buffer: bytes.NewBufferString("lambda")}, nil)
				m.EXPECT().Parse(lbFargateAppTemplatePath, struct {
					RulePriorityLambda string
					*lbFargateTemplateParams
				}{
					RulePriorityLambda:      "lambda",
					lbFargateTemplateParams: c.toTemplateParams(),
				}).Return(&template.Content{Buffer: bytes.NewBufferString("template")}, nil)
				c.parser = m
			},

			wantedTemplate: "template",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			conf := &LBFargateStackConfig{
				CreateLBFargateAppInput: tc.in,
			}
			tc.mockDependencies(ctrl, conf)

			// WHEN
			template, err := conf.Template()

			// THEN
			require.Equal(t, tc.wantedError, err)
			require.Equal(t, tc.wantedTemplate, template)
		})
	}
}

func TestLBFargateStackConfig_Parameters(t *testing.T) {
	testCases := map[string]struct {
		httpsEnabled bool
		expectedHTTP string
	}{
		"HTTPS Enabled": {
			httpsEnabled: true,
			expectedHTTP: "true",
		},
		"HTTPS Not Enabled": {
			httpsEnabled: false,
			expectedHTTP: "false",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			// GIVEN
			conf := &LBFargateStackConfig{
				CreateLBFargateAppInput: &deploy.CreateLBFargateAppInput{
					App: manifest.NewLoadBalancedFargateManifest(&manifest.LBFargateManifestProps{
						AppManifestProps: &manifest.AppManifestProps{
							AppName:    "frontend",
							Dockerfile: "frontend/Dockerfile",
						},
						Path: "frontend",
					}),
					Env: &archer.Environment{
						Project:   "phonetool",
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "12345",
						Prod:      false,
					},
					ImageRepoURL: "12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend",
					ImageTag:     "manual-bf3678c",
				},
				httpsEnabled: tc.httpsEnabled,
			}

			// WHEN
			params := conf.Parameters()

			// THEN
			require.Equal(t, []*cloudformation.Parameter{
				{
					ParameterKey:   aws.String(LBFargateParamProjectNameKey),
					ParameterValue: aws.String("phonetool"),
				},
				{
					ParameterKey:   aws.String(LBFargateParamEnvNameKey),
					ParameterValue: aws.String("test"),
				},
				{
					ParameterKey:   aws.String(LBFargateParamAppNameKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(LBFargateParamContainerImageKey),
					ParameterValue: aws.String("12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend:manual-bf3678c"),
				},
				{
					ParameterKey:   aws.String(LBFargateParamContainerPortKey),
					ParameterValue: aws.String("80"),
				},
				{
					ParameterKey:   aws.String(LBFargateRulePathKey),
					ParameterValue: aws.String("frontend"),
				},
				{
					ParameterKey:   aws.String(LBFargateTaskCPUKey),
					ParameterValue: aws.String("256"),
				},
				{
					ParameterKey:   aws.String(LBFargateTaskMemoryKey),
					ParameterValue: aws.String("512"),
				},
				{
					ParameterKey:   aws.String(LBFargateTaskCountKey),
					ParameterValue: aws.String("1"),
				},
				{
					ParameterKey:   aws.String(LBFargateParamHTTPSKey),
					ParameterValue: aws.String(tc.expectedHTTP),
				},
			}, params)
		})
	}
}

func TestLBFargateStackConfig_SerializedParameters(t *testing.T) {
	testCases := map[string]struct {
		in               *deploy.CreateLBFargateAppInput
		mockDependencies func(ctrl *gomock.Controller, c *LBFargateStackConfig)

		wantedParams string
		wantedError  error
	}{
		"unavailable template": {
			in: &deploy.CreateLBFargateAppInput{
				App: manifest.NewLoadBalancedFargateManifest(&manifest.LBFargateManifestProps{
					AppManifestProps: &manifest.AppManifestProps{
						AppName:    "frontend",
						Dockerfile: "frontend/Dockerfile",
					},
					Path: "frontend",
				}),
				Env: &archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					Region:    "us-west-2",
					AccountID: "12345",
					Prod:      false,
				},
				ImageRepoURL: "12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend",
				ImageTag:     "manual-bf3678c",
			},
			mockDependencies: func(ctrl *gomock.Controller, c *LBFargateStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Parse(lbFargateAppParamsPath, gomock.Any()).Return(nil, errors.New("some error"))
				c.parser = m
			},
			wantedParams: "",
			wantedError:  errors.New("some error"),
		},
		"render params template": {
			in: &deploy.CreateLBFargateAppInput{
				App: manifest.NewLoadBalancedFargateManifest(&manifest.LBFargateManifestProps{
					AppManifestProps: &manifest.AppManifestProps{
						AppName:    "frontend",
						Dockerfile: "frontend/Dockerfile",
					},
					Path: "frontend",
				}),
				Env: &archer.Environment{
					Project:   "phonetool",
					Name:      "test",
					Region:    "us-west-2",
					AccountID: "12345",
					Prod:      false,
				},
				ImageRepoURL: "12345.dkr.ecr.us-west-2.amazonaws.com/phonetool/frontend",
				ImageTag:     "manual-bf3678c",
			},
			mockDependencies: func(ctrl *gomock.Controller, c *LBFargateStackConfig) {
				m := mocks.NewMockReadParser(ctrl)
				m.EXPECT().Parse(lbFargateAppParamsPath, gomock.Any()).Return(&template.Content{Buffer: bytes.NewBufferString("params")}, nil)
				c.parser = m
			},
			wantedParams: "params",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			c := &LBFargateStackConfig{
				CreateLBFargateAppInput: tc.in,
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

func TestLBFargateStackConfig_Tags(t *testing.T) {
	// GIVEN
	conf := &LBFargateStackConfig{
		CreateLBFargateAppInput: &deploy.CreateLBFargateAppInput{
			App: manifest.NewLoadBalancedFargateManifest(&manifest.LBFargateManifestProps{
				AppManifestProps: &manifest.AppManifestProps{
					AppName:    "frontend",
					Dockerfile: "frontend/Dockerfile",
				},
				Path: "frontend",
			}),
			Env: &archer.Environment{
				Project:   "phonetool",
				Name:      "test",
				Region:    "us-west-2",
				AccountID: "12345",
				Prod:      false,
			},
			ImageTag: "manual-bf3678c",
		},
	}

	// WHEN
	tags := conf.Tags()

	// THEN
	require.Equal(t, []*cloudformation.Tag{
		{
			Key:   aws.String(ProjectTagKey),
			Value: aws.String("phonetool"),
		},
		{
			Key:   aws.String(EnvTagKey),
			Value: aws.String("test"),
		},
		{
			Key:   aws.String(AppTagKey),
			Value: aws.String("frontend"),
		},
	}, tags)
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestWebAppURI_String(t *testing.T) {
	testCases := map[string]struct {
		dnsName string
		path    string

		wanted string
	}{
		"http": {
			dnsName: "abc.us-west-1.elb.amazonaws.com",
			path:    "app",

			wanted: "http://abc.us-west-1.elb.amazonaws.com/app",
		},
		"http with / path": {
			dnsName: "jobs.test.phonetool.com",
			path:    "/",

			wanted: "http://jobs.test.phonetool.com",
		},
		"https": {
			dnsName: "jobs.test.phonetool.com",
			path:    "",

			wanted: "https://jobs.test.phonetool.com",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			uri := &WebAppURI{
				DNSName: tc.dnsName,
				Path:    tc.path,
			}

			require.Equal(t, tc.wanted, uri.String())
		})
	}
}

type webAppDescriberMocks struct {
	storeSvc     *mocks.MockstoreSvc
	appDescriber *mocks.MockappDescriber
}

func TestWebAppDescriber_URI(t *testing.T) {
	const (
		testProject      = "phonetool"
		testEnv          = "test"
		testApp          = "jobs"
		testEnvSubdomain = "test.phonetool.com"
		testEnvLBDNSName = "http://abc.us-west-1.elb.amazonaws.com"
		testAppPath      = "*"
	)
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks webAppDescriberMocks)

		wantedURI   string
		wantedError error
	}{
		"fail to get output of environment stack": {
			setupMocks: func(m webAppDescriberMocks) {
				gomock.InOrder(
					m.appDescriber.EXPECT().EnvOutputs().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get output for environment test: some error"),
		},
		"fail to get parameters of application stack": {
			setupMocks: func(m webAppDescriberMocks) {
				gomock.InOrder(
					m.appDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						stack.EnvOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get parameters for application jobs: some error"),
		},
		"https web application": {
			setupMocks: func(m webAppDescriberMocks) {
				gomock.InOrder(
					m.appDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						stack.EnvOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebAppRulePathParamKey: testAppPath,
					}, nil),
				)
			},

			wantedURI: "https://jobs.test.phonetool.com",
		},
		"http web application": {
			setupMocks: func(m webAppDescriberMocks) {
				gomock.InOrder(
					m.appDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebAppRulePathParamKey: testAppPath,
					}, nil),
				)
			},

			wantedURI: "http://http://abc.us-west-1.elb.amazonaws.com/*",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockAppDescriber := mocks.NewMockappDescriber(ctrl)
			mocks := webAppDescriberMocks{
				appDescriber: mockAppDescriber,
			}

			tc.setupMocks(mocks)

			d := &WebAppDescriber{
				app: &archer.Application{
					Project: testProject,
					Name:    testApp,
				},
				appDescriber:     mockAppDescriber,
				initAppDescriber: func(string) error { return nil },
			}

			// WHEN
			actual, err := d.URI(testEnv)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedURI, actual)
			}
		})
	}
}

func TestWebAppDescriber_Describe(t *testing.T) {
	const (
		testProject      = "phonetool"
		testEnv          = "test"
		testApp          = "jobs"
		testEnvLBDNSName = "abc.us-west-1.elb.amazonaws.com"
		testAppPath      = "*"
		prodEnv          = "prod"
		prodEnvLBDNSName = "abc.us-west-1.elb.amazonaws.com"
		prodAppPath      = "*"
	)
	mockErr := errors.New("some error")
	mockNotExistErr := awserr.New("ValidationError", "Stack with id mockID does not exist", nil)
	testEnvironment := archer.Environment{
		Project: testProject,
		Name:    testEnv,
	}
	prodEnvironment := archer.Environment{
		Project: testProject,
		Name:    prodEnv,
	}
	testCases := map[string]struct {
		shouldOutputResources bool

		setupMocks func(mocks webAppDescriberMocks)

		wantedWebApp *WebAppDesc
		wantedError  error
	}{
		"return error if fail to list environment": {
			setupMocks: func(m webAppDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("list environments: some error"),
		},
		"return error if fail to retrieve URI": {
			setupMocks: func(m webAppDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
						&testEnvironment,
					}, nil),
					m.appDescriber.EXPECT().EnvOutputs().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve application URI: get output for environment test: some error"),
		},
		"return error if fail to retrieve application deployment configuration": {
			setupMocks: func(m webAppDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
						&testEnvironment,
					}, nil),
					m.appDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebAppRulePathParamKey: testAppPath,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebAppContainerPortParamKey: "80",
						stack.AppTaskCountParamKey:          "1",
						stack.AppTaskCPUParamKey:            "256",
						stack.AppTaskMemoryParamKey:         "512",
					}, nil),
					m.appDescriber.EXPECT().EnvVars().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve environment variables: some error"),
		},
		"return error if fail to retrieve application resources": {
			shouldOutputResources: true,
			setupMocks: func(m webAppDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
						&testEnvironment,
					}, nil),
					m.appDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebAppRulePathParamKey: testAppPath,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebAppContainerPortParamKey: "80",
						stack.AppTaskCountParamKey:          "1",
						stack.AppTaskCPUParamKey:            "256",
						stack.AppTaskMemoryParamKey:         "512",
					}, nil),
					m.appDescriber.EXPECT().EnvVars().Return(
						map[string]string{
							"ECS_CLI_ENVIRONMENT_NAME": testEnv,
						}, nil),
					m.appDescriber.EXPECT().AppStackResources().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve application resources: some error"),
		},
		"skip if not deployed": {
			shouldOutputResources: true,
			setupMocks: func(m webAppDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
						&testEnvironment,
					}, nil),
					m.appDescriber.EXPECT().EnvOutputs().Return(nil, mockNotExistErr),
					m.appDescriber.EXPECT().AppStackResources().Return(nil, mockNotExistErr),
				)
			},
			wantedWebApp: &WebAppDesc{
				AppName:        testApp,
				Type:           "",
				Project:        testProject,
				Configurations: []*WebAppConfig(nil),
				Routes:         []*WebAppRoute(nil),
				Variables:      []*EnvVars(nil),
				Resources:      make(map[string][]*CfnResource),
			},
		},
		"success": {
			shouldOutputResources: true,
			setupMocks: func(m webAppDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
						&testEnvironment,
						&prodEnvironment,
					}, nil),

					m.appDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebAppRulePathParamKey: testAppPath,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebAppContainerPortParamKey: "80",
						stack.AppTaskCountParamKey:          "1",
						stack.AppTaskCPUParamKey:            "256",
						stack.AppTaskMemoryParamKey:         "512",
					}, nil),
					m.appDescriber.EXPECT().EnvVars().Return(
						map[string]string{
							"ECS_CLI_ENVIRONMENT_NAME": testEnv,
						}, nil),

					m.appDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: prodEnvLBDNSName,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebAppRulePathParamKey: prodAppPath,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebAppContainerPortParamKey: "5000",
						stack.AppTaskCountParamKey:          "2",
						stack.AppTaskCPUParamKey:            "512",
						stack.AppTaskMemoryParamKey:         "1024",
					}, nil),
					m.appDescriber.EXPECT().EnvVars().Return(
						map[string]string{
							"ECS_CLI_ENVIRONMENT_NAME": prodEnv,
						}, nil),

					m.appDescriber.EXPECT().AppStackResources().Return([]*cloudformation.StackResource{
						{
							ResourceType:       aws.String("AWS::EC2::SecurityGroupIngress"),
							PhysicalResourceId: aws.String("ContainerSecurityGroupIngressFromPublicALB"),
						},
					}, nil),
					m.appDescriber.EXPECT().AppStackResources().Return([]*cloudformation.StackResource{
						{
							ResourceType:       aws.String("AWS::EC2::SecurityGroup"),
							PhysicalResourceId: aws.String("sg-0758ed6b233743530"),
						},
					}, nil),
				)
			},
			wantedWebApp: &WebAppDesc{
				AppName: testApp,
				Type:    "",
				Project: testProject,
				Configurations: []*WebAppConfig{
					{
						CPU:         "256",
						Environment: "test",
						Memory:      "512",
						Port:        "80",
						Tasks:       "1",
					},
					{
						CPU:         "512",
						Environment: "prod",
						Memory:      "1024",
						Port:        "5000",
						Tasks:       "2",
					},
				},
				Routes: []*WebAppRoute{
					{
						Environment: "test",
						URL:         "http://abc.us-west-1.elb.amazonaws.com/*",
					},
					{
						Environment: "prod",
						URL:         "http://abc.us-west-1.elb.amazonaws.com/*",
					},
				},
				Variables: []*EnvVars{
					{
						Environment: "prod",
						Name:        "ECS_CLI_ENVIRONMENT_NAME",
						Value:       "prod",
					},
					{
						Environment: "test",
						Name:        "ECS_CLI_ENVIRONMENT_NAME",
						Value:       "test",
					},
				},
				Resources: map[string][]*CfnResource{
					"test": []*CfnResource{
						{
							Type:       "AWS::EC2::SecurityGroupIngress",
							PhysicalID: "ContainerSecurityGroupIngressFromPublicALB",
						},
					},
					"prod": []*CfnResource{
						{
							Type:       "AWS::EC2::SecurityGroup",
							PhysicalID: "sg-0758ed6b233743530",
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstoreSvc(ctrl)
			mockAppDescriber := mocks.NewMockappDescriber(ctrl)
			mocks := webAppDescriberMocks{
				storeSvc:     mockStore,
				appDescriber: mockAppDescriber,
			}

			tc.setupMocks(mocks)

			d := &WebAppDescriber{
				app: &archer.Application{
					Project: testProject,
					Name:    testApp,
				},
				enableResources:  tc.shouldOutputResources,
				store:            mockStore,
				appDescriber:     mockAppDescriber,
				initAppDescriber: func(string) error { return nil },
			}

			// WHEN
			webapp, err := d.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedWebApp, webapp, "expected output content match")
			}
		})
	}
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	CFNStack "github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe/stack"
	"github.com/aws/aws-sdk-go/aws/awserr"
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

type describeMocks struct {
	storeSvc       *mocks.MockstoreSvc
	stackDescriber *mocks.MockstackDescriber
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
	environment := archer.Environment{
		Project: testProject,
		Name:    testEnv,
	}
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks describeMocks)

		wantedURI   *WebAppURI
		wantedError error
	}{
		"environment does not exist in store": {
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetEnvironment(testProject, testEnv).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get environment test: some error"),
		},
		"fail to get output of environment stack": {
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetEnvironment(testProject, testEnv).Return(&environment, nil),
					m.stackDescriber.EXPECT().EnvOutputs(&environment).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get output for environment test: some error"),
		},
		"fail to get parameters of application stack": {
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetEnvironment(testProject, testEnv).Return(&environment, nil),
					m.stackDescriber.EXPECT().EnvOutputs(&environment).Return(map[string]string{
						CFNStack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						CFNStack.EnvOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.stackDescriber.EXPECT().AppParams(&environment, testApp).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get parameters for application jobs: some error"),
		},
		"https web application": {
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetEnvironment(testProject, testEnv).Return(&environment, nil),
					m.stackDescriber.EXPECT().EnvOutputs(&environment).Return(map[string]string{
						CFNStack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						CFNStack.EnvOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.stackDescriber.EXPECT().AppParams(&environment, testApp).Return(map[string]string{
						CFNStack.LBWebAppRulePathParamKey: testAppPath,
					}, nil),
				)
			},

			wantedURI: &WebAppURI{
				DNSName: testApp + "." + testEnvSubdomain,
			},
		},
		"http web application": {
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetEnvironment(testProject, testEnv).Return(&environment, nil),
					m.stackDescriber.EXPECT().EnvOutputs(&environment).Return(map[string]string{
						CFNStack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.stackDescriber.EXPECT().AppParams(&environment, testApp).Return(map[string]string{
						CFNStack.LBWebAppRulePathParamKey: testAppPath,
					}, nil),
				)
			},

			wantedURI: &WebAppURI{
				DNSName: testEnvLBDNSName,
				Path:    testAppPath,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstoreSvc(ctrl)
			mockStackDescriber := mocks.NewMockstackDescriber(ctrl)
			mocks := describeMocks{
				storeSvc:       mockStore,
				stackDescriber: mockStackDescriber,
			}

			tc.setupMocks(mocks)

			d := &WebAppDescriber{
				app: &archer.Application{
					Project: testProject,
					Name:    testApp,
				},
				store:          mockStore,
				stackDescriber: mockStackDescriber,
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

		setupMocks func(mocks describeMocks)

		wantedWebApp *WebApp
		wantedError  error
	}{
		"return error if fail to list environment": {
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("list environments: some error"),
		},
		"return error if fail to retrieve URI": {
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
						&testEnvironment,
					}, nil),
					m.storeSvc.EXPECT().GetEnvironment(testProject, testEnv).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve application URI: get environment test: some error"),
		},
		"return error if fail to retrieve application deployment configuration": {
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
						&testEnvironment,
					}, nil),
					m.storeSvc.EXPECT().GetEnvironment(testProject, testEnv).Return(&testEnvironment, nil),
					m.stackDescriber.EXPECT().EnvOutputs(&testEnvironment).Return(map[string]string{
						CFNStack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.stackDescriber.EXPECT().AppParams(&testEnvironment, testApp).Return(map[string]string{
						CFNStack.LBWebAppRulePathParamKey: testAppPath,
					}, nil),
					m.stackDescriber.EXPECT().AppParams(&testEnvironment, testApp).Return(map[string]string{
						CFNStack.LBWebAppContainerPortParamKey: "80",
						CFNStack.AppTaskCountParamKey:          "1",
						CFNStack.AppTaskCPUParamKey:            "256",
						CFNStack.AppTaskMemoryParamKey:         "512",
					}, nil),
					m.stackDescriber.EXPECT().EnvVars(&testEnvironment, testApp).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve environment variables: some error"),
		},
		"return error if fail to retrieve application resources": {
			shouldOutputResources: true,
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
						&testEnvironment,
					}, nil),
					m.storeSvc.EXPECT().GetEnvironment(testProject, testEnv).Return(&testEnvironment, nil),
					m.stackDescriber.EXPECT().EnvOutputs(&testEnvironment).Return(map[string]string{
						CFNStack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.stackDescriber.EXPECT().AppParams(&testEnvironment, testApp).Return(map[string]string{
						CFNStack.LBWebAppRulePathParamKey: testAppPath,
					}, nil),
					m.stackDescriber.EXPECT().AppParams(&testEnvironment, testApp).Return(map[string]string{
						CFNStack.LBWebAppContainerPortParamKey: "80",
						CFNStack.AppTaskCountParamKey:          "1",
						CFNStack.AppTaskCPUParamKey:            "256",
						CFNStack.AppTaskMemoryParamKey:         "512",
					}, nil),
					m.stackDescriber.EXPECT().EnvVars(&testEnvironment, testApp).Return([]*stack.EnvVars{
						&stack.EnvVars{
							Environment: testEnv,
							Name:        "ECS_CLI_ENVIRONMENT_NAME",
							Value:       testEnv,
						},
					}, nil),
					m.stackDescriber.EXPECT().StackResources(testEnv, testApp).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve application resources: some error"),
		},
		"skip if not deployed": {
			shouldOutputResources: true,
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
						&testEnvironment,
					}, nil),
					m.storeSvc.EXPECT().GetEnvironment(testProject, testEnv).Return(&testEnvironment, nil),
					m.stackDescriber.EXPECT().EnvOutputs(&testEnvironment).Return(nil, mockNotExistErr),
					m.stackDescriber.EXPECT().StackResources(testEnv, testApp).Return(nil, mockNotExistErr),
				)
			},
			wantedWebApp: &WebApp{
				AppName:        testApp,
				Type:           "",
				Project:        testProject,
				Configurations: []*WebAppConfig(nil),
				Routes:         []*WebAppRoute(nil),
				Variables:      []*stack.EnvVars(nil),
				Resources:      make(map[string][]*stack.CfnResource),
			},
		},
		"success": {
			shouldOutputResources: true,
			setupMocks: func(m describeMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
						&testEnvironment,
						&prodEnvironment,
					}, nil),

					m.storeSvc.EXPECT().GetEnvironment(testProject, testEnv).Return(&testEnvironment, nil),
					m.stackDescriber.EXPECT().EnvOutputs(&testEnvironment).Return(map[string]string{
						CFNStack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.stackDescriber.EXPECT().AppParams(&testEnvironment, testApp).Return(map[string]string{
						CFNStack.LBWebAppRulePathParamKey: testAppPath,
					}, nil),
					m.stackDescriber.EXPECT().AppParams(&testEnvironment, testApp).Return(map[string]string{
						CFNStack.LBWebAppContainerPortParamKey: "80",
						CFNStack.AppTaskCountParamKey:          "1",
						CFNStack.AppTaskCPUParamKey:            "256",
						CFNStack.AppTaskMemoryParamKey:         "512",
					}, nil),
					m.stackDescriber.EXPECT().EnvVars(&testEnvironment, testApp).Return([]*stack.EnvVars{
						&stack.EnvVars{
							Environment: testEnv,
							Name:        "ECS_CLI_ENVIRONMENT_NAME",
							Value:       testEnv,
						},
					}, nil),

					m.storeSvc.EXPECT().GetEnvironment(testProject, prodEnv).Return(&prodEnvironment, nil),
					m.stackDescriber.EXPECT().EnvOutputs(&prodEnvironment).Return(map[string]string{
						CFNStack.EnvOutputPublicLoadBalancerDNSName: prodEnvLBDNSName,
					}, nil),
					m.stackDescriber.EXPECT().AppParams(&prodEnvironment, testApp).Return(map[string]string{
						CFNStack.LBWebAppRulePathParamKey: prodAppPath,
					}, nil),
					m.stackDescriber.EXPECT().AppParams(&prodEnvironment, testApp).Return(map[string]string{
						CFNStack.LBWebAppContainerPortParamKey: "5000",
						CFNStack.AppTaskCountParamKey:          "2",
						CFNStack.AppTaskCPUParamKey:            "512",
						CFNStack.AppTaskMemoryParamKey:         "1024",
					}, nil),
					m.stackDescriber.EXPECT().EnvVars(&prodEnvironment, testApp).Return([]*stack.EnvVars{
						&stack.EnvVars{
							Environment: prodEnv,
							Name:        "ECS_CLI_ENVIRONMENT_NAME",
							Value:       prodEnv,
						},
					}, nil),

					m.stackDescriber.EXPECT().StackResources(testEnv, testApp).Return([]*stack.CfnResource{
						&stack.CfnResource{
							Type:       "AWS::EC2::SecurityGroupIngress",
							PhysicalID: "ContainerSecurityGroupIngressFromPublicALB",
						},
					}, nil),
					m.stackDescriber.EXPECT().StackResources(prodEnv, testApp).Return([]*stack.CfnResource{
						&stack.CfnResource{
							Type:       "AWS::EC2::SecurityGroup",
							PhysicalID: "sg-0758ed6b233743530",
						},
					}, nil),
				)
			},
			wantedWebApp: &WebApp{
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
				Variables: []*stack.EnvVars{
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
				Resources: map[string][]*stack.CfnResource{
					"test": []*stack.CfnResource{
						{
							Type:       "AWS::EC2::SecurityGroupIngress",
							PhysicalID: "ContainerSecurityGroupIngressFromPublicALB",
						},
					},
					"prod": []*stack.CfnResource{
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
			mockStackDescriber := mocks.NewMockstackDescriber(ctrl)
			mocks := describeMocks{
				storeSvc:       mockStore,
				stackDescriber: mockStackDescriber,
			}

			tc.setupMocks(mocks)

			d := &WebAppDescriber{
				app: &archer.Application{
					Project: testProject,
					Name:    testApp,
				},
				store:          mockStore,
				stackDescriber: mockStackDescriber,
			}

			// WHEN
			webapp, err := d.Describe(tc.shouldOutputResources)

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

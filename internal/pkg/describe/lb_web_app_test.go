// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
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
						stack.LBWebServiceRulePathParamKey: testAppPath,
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
						stack.LBWebServiceRulePathParamKey: testAppPath,
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
				app: &config.Service{
					App:  testProject,
					Name: testApp,
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
	testEnvironment := config.Environment{
		App:  testProject,
		Name: testEnv,
	}
	prodEnvironment := config.Environment{
		App:  testProject,
		Name: prodEnv,
	}
	testCases := map[string]struct {
		shouldOutputResources bool

		setupMocks func(mocks webAppDescriberMocks)

		wantedWebApp *webAppDesc
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
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*config.Environment{
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
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*config.Environment{
						&testEnvironment,
					}, nil),
					m.appDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey: testAppPath,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "80",
						stack.ServiceTaskCountParamKey:          "1",
						stack.ServiceTaskCPUParamKey:            "256",
						stack.ServiceTaskMemoryParamKey:         "512",
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
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*config.Environment{
						&testEnvironment,
					}, nil),
					m.appDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey: testAppPath,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "80",
						stack.ServiceTaskCountParamKey:          "1",
						stack.ServiceTaskCPUParamKey:            "256",
						stack.ServiceTaskMemoryParamKey:         "512",
					}, nil),
					m.appDescriber.EXPECT().EnvVars().Return(
						map[string]string{
							"COPILOT_ENVIRONMENT_NAME": testEnv,
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
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*config.Environment{
						&testEnvironment,
					}, nil),
					m.appDescriber.EXPECT().EnvOutputs().Return(nil, mockNotExistErr),
					m.appDescriber.EXPECT().AppStackResources().Return(nil, mockNotExistErr),
				)
			},
			wantedWebApp: &webAppDesc{
				AppName:          testApp,
				Type:             "",
				Project:          testProject,
				Configurations:   []*AppConfig(nil),
				Routes:           []*WebAppRoute(nil),
				ServiceDiscovery: []*ServiceDiscovery(nil),
				Variables:        []*EnvVars(nil),
				Resources:        make(map[string][]*CfnResource),
			},
		},
		"success": {
			shouldOutputResources: true,
			setupMocks: func(m webAppDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*config.Environment{
						&testEnvironment,
						&prodEnvironment,
					}, nil),

					m.appDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey: testAppPath,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "5000",
						stack.ServiceTaskCountParamKey:          "1",
						stack.ServiceTaskCPUParamKey:            "256",
						stack.ServiceTaskMemoryParamKey:         "512",
					}, nil),
					m.appDescriber.EXPECT().EnvVars().Return(
						map[string]string{
							"COPILOT_ENVIRONMENT_NAME": testEnv,
						}, nil),

					m.appDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: prodEnvLBDNSName,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey: prodAppPath,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "5000",
						stack.ServiceTaskCountParamKey:          "2",
						stack.ServiceTaskCPUParamKey:            "512",
						stack.ServiceTaskMemoryParamKey:         "1024",
					}, nil),
					m.appDescriber.EXPECT().EnvVars().Return(
						map[string]string{
							"COPILOT_ENVIRONMENT_NAME": prodEnv,
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
			wantedWebApp: &webAppDesc{
				AppName: testApp,
				Type:    "",
				Project: testProject,
				Configurations: []*AppConfig{
					{
						CPU:         "256",
						Environment: "test",
						Memory:      "512",
						Port:        "5000",
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
				ServiceDiscovery: []*ServiceDiscovery{
					{
						Environment: []string{"test", "prod"},
						Namespace:   "jobs.phonetool.local:5000",
					},
				},
				Variables: []*EnvVars{
					{
						Environment: "prod",
						Name:        "COPILOT_ENVIRONMENT_NAME",
						Value:       "prod",
					},
					{
						Environment: "test",
						Name:        "COPILOT_ENVIRONMENT_NAME",
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
				app: &config.Service{
					App:  testProject,
					Name: testApp,
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

func TestWebAppDesc_String(t *testing.T) {
	testCases := map[string]struct {
		wantedHumanString string
		wantedJSONString  string
	}{
		"correct output": {
			wantedHumanString: `About

  Project           my-project
  Name              my-app
  Type              Load Balanced Web Service

Configurations

  Environment       Tasks               CPU (vCPU)          Memory (MiB)        Port
  test              1                   0.25                512                 80
  prod              3                   0.5                 1024                5000

Routes

  Environment       URL
  test              http://my-pr-Publi.us-west-2.elb.amazonaws.com/frontend
  prod              http://my-pr-Publi.us-west-2.elb.amazonaws.com/backend

Service Discovery

  Environment       Namespace
  test, prod        http://my-app.my-project.local:5000

Variables

  Name                      Environment         Value
  COPILOT_ENVIRONMENT_NAME  prod                prod
  -                         test                test

Resources

  test
    AWS::EC2::SecurityGroup  sg-0758ed6b233743530

  prod
    AWS::EC2::SecurityGroupIngress  ContainerSecurityGroupIngressFromPublicALB
`,
			wantedJSONString: "{\"appName\":\"my-app\",\"type\":\"Load Balanced Web Service\",\"project\":\"my-project\",\"configurations\":[{\"environment\":\"test\",\"port\":\"80\",\"tasks\":\"1\",\"cpu\":\"256\",\"memory\":\"512\"},{\"environment\":\"prod\",\"port\":\"5000\",\"tasks\":\"3\",\"cpu\":\"512\",\"memory\":\"1024\"}],\"routes\":[{\"environment\":\"test\",\"url\":\"http://my-pr-Publi.us-west-2.elb.amazonaws.com/frontend\"},{\"environment\":\"prod\",\"url\":\"http://my-pr-Publi.us-west-2.elb.amazonaws.com/backend\"}],\"serviceDiscovery\":[{\"environment\":[\"test\",\"prod\"],\"namespace\":\"http://my-app.my-project.local:5000\"}],\"variables\":[{\"environment\":\"prod\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"prod\"},{\"environment\":\"test\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"test\"}],\"resources\":{\"prod\":[{\"type\":\"AWS::EC2::SecurityGroupIngress\",\"physicalID\":\"ContainerSecurityGroupIngressFromPublicALB\"}],\"test\":[{\"type\":\"AWS::EC2::SecurityGroup\",\"physicalID\":\"sg-0758ed6b233743530\"}]}}\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			config := []*AppConfig{
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
					Tasks:       "3",
				},
			}
			envVars := []*EnvVars{
				{
					Environment: "prod",
					Name:        "COPILOT_ENVIRONMENT_NAME",
					Value:       "prod",
				},
				{
					Environment: "test",
					Name:        "COPILOT_ENVIRONMENT_NAME",
					Value:       "test",
				},
			}
			routes := []*WebAppRoute{
				{
					Environment: "test",
					URL:         "http://my-pr-Publi.us-west-2.elb.amazonaws.com/frontend",
				},
				{
					Environment: "prod",
					URL:         "http://my-pr-Publi.us-west-2.elb.amazonaws.com/backend",
				},
			}
			sds := []*ServiceDiscovery{
				{
					Environment: []string{"test", "prod"},
					Namespace:   "http://my-app.my-project.local:5000",
				},
			}
			resources := map[string][]*CfnResource{
				"test": []*CfnResource{
					{
						PhysicalID: "sg-0758ed6b233743530",
						Type:       "AWS::EC2::SecurityGroup",
					},
				},
				"prod": []*CfnResource{
					{
						Type:       "AWS::EC2::SecurityGroupIngress",
						PhysicalID: "ContainerSecurityGroupIngressFromPublicALB",
					},
				},
			}
			webApp := &webAppDesc{
				AppName:          "my-app",
				Type:             "Load Balanced Web Service",
				Configurations:   config,
				Project:          "my-project",
				Variables:        envVars,
				Routes:           routes,
				ServiceDiscovery: sds,
				Resources:        resources,
			}
			human := webApp.HumanString()
			json, _ := webApp.JSONString()

			require.Equal(t, tc.wantedHumanString, human)
			require.Equal(t, tc.wantedJSONString, json)
		})
	}
}

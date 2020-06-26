// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type backendSvcDescriberMocks struct {
	storeSvc     *mocks.MockstoreSvc
	svcDescriber *mocks.MocksvcDescriber
}

func TestBackendServiceDescriber_Describe(t *testing.T) {
	const (
		testApp     = "phonetool"
		testEnv     = "test"
		testSvc     = "jobs"
		testSvcPath = "*"
		prodEnv     = "prod"
		prodSvcPath = "*"
	)
	mockErr := errors.New("some error")
	mockNotExistErr := awserr.New("ValidationError", "Stack with id mockID does not exist", nil)
	testEnvironment := config.Environment{
		App:  testApp,
		Name: testEnv,
	}
	prodEnvironment := config.Environment{
		App:  testApp,
		Name: prodEnv,
	}
	testCases := map[string]struct {
		shouldOutputResources bool

		setupMocks func(mocks backendSvcDescriberMocks)

		wantedBackendSvc *backendSvcDesc
		wantedError      error
	}{
		"return error if fail to list environment": {
			setupMocks: func(m backendSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testApp).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("list environments for application phonetool: some error"),
		},
		"return error if fail to retrieve service deployment configuration": {
			setupMocks: func(m backendSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testApp).Return([]*config.Environment{
						&testEnvironment,
					}, nil),
					m.svcDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service deployment configuration: some error"),
		},
		"return error if fail to retrieve environment variables": {
			setupMocks: func(m backendSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testApp).Return([]*config.Environment{
						&testEnvironment,
					}, nil),
					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "80",
						stack.ServiceTaskCountParamKey:          "1",
						stack.ServiceTaskCPUParamKey:            "256",
						stack.ServiceTaskMemoryParamKey:         "512",
					}, nil),
					m.svcDescriber.EXPECT().EnvVars().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve environment variables: some error"),
		},
		"skip if not deployed": {
			shouldOutputResources: true,
			setupMocks: func(m backendSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testApp).Return([]*config.Environment{
						&testEnvironment,
					}, nil),
					m.svcDescriber.EXPECT().Params().Return(nil, mockNotExistErr),
					m.svcDescriber.EXPECT().ServiceStackResources().Return(nil, mockNotExistErr),
				)
			},
			wantedBackendSvc: &backendSvcDesc{
				Service:          testSvc,
				Type:             "",
				App:              testApp,
				Configurations:   []*ServiceConfig(nil),
				ServiceDiscovery: []*ServiceDiscovery(nil),
				Variables:        []*EnvVars(nil),
				Resources:        make(map[string][]*CfnResource),
			},
		},
		"success": {
			shouldOutputResources: true,
			setupMocks: func(m backendSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testApp).Return([]*config.Environment{
						&testEnvironment,
						&prodEnvironment,
					}, nil),

					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "5000",
						stack.ServiceTaskCountParamKey:          "1",
						stack.ServiceTaskCPUParamKey:            "256",
						stack.ServiceTaskMemoryParamKey:         "512",
					}, nil),
					m.svcDescriber.EXPECT().EnvVars().Return(
						map[string]string{
							"COPILOT_ENVIRONMENT_NAME": testEnv,
						}, nil),

					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "5000",
						stack.ServiceTaskCountParamKey:          "2",
						stack.ServiceTaskCPUParamKey:            "512",
						stack.ServiceTaskMemoryParamKey:         "1024",
					}, nil),
					m.svcDescriber.EXPECT().EnvVars().Return(
						map[string]string{
							"COPILOT_ENVIRONMENT_NAME": prodEnv,
						}, nil),

					m.svcDescriber.EXPECT().ServiceStackResources().Return([]*cloudformation.StackResource{
						{
							ResourceType:       aws.String("AWS::EC2::SecurityGroupIngress"),
							PhysicalResourceId: aws.String("ContainerSecurityGroupIngressFromPublicALB"),
						},
					}, nil),
					m.svcDescriber.EXPECT().ServiceStackResources().Return([]*cloudformation.StackResource{
						{
							ResourceType:       aws.String("AWS::EC2::SecurityGroup"),
							PhysicalResourceId: aws.String("sg-0758ed6b233743530"),
						},
					}, nil),
				)
			},
			wantedBackendSvc: &backendSvcDesc{
				Service: testSvc,
				Type:    "",
				App:     testApp,
				Configurations: []*ServiceConfig{
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
			mockSvcDescriber := mocks.NewMocksvcDescriber(ctrl)
			mocks := backendSvcDescriberMocks{
				storeSvc:     mockStore,
				svcDescriber: mockSvcDescriber,
			}

			tc.setupMocks(mocks)

			d := &BackendServiceDescriber{
				service: &config.Service{
					App:  testApp,
					Name: testSvc,
				},
				enableResources:      tc.shouldOutputResources,
				store:                mockStore,
				svcDescriber:         mockSvcDescriber,
				initServiceDescriber: func(string) error { return nil },
			}

			// WHEN
			backendsvc, err := d.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedBackendSvc, backendsvc, "expected output content match")
			}
		})
	}
}

func TestBackendSvcDesc_String(t *testing.T) {
	testCases := map[string]struct {
		wantedHumanString string
		wantedJSONString  string
	}{
		"correct output": {
			wantedHumanString: `About

  Application       my-app
  Name              my-svc
  Type              Backend Service

Configurations

  Environment       Tasks               CPU (vCPU)          Memory (MiB)        Port
  test              1                   0.25                512                 80
  prod              3                   0.5                 1024                5000

Service Discovery

  Environment       Namespace
  test, prod        http://my-svc.my-app.local:5000

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
			wantedJSONString: "{\"service\":\"my-svc\",\"type\":\"Backend Service\",\"application\":\"my-app\",\"configurations\":[{\"environment\":\"test\",\"port\":\"80\",\"tasks\":\"1\",\"cpu\":\"256\",\"memory\":\"512\"},{\"environment\":\"prod\",\"port\":\"5000\",\"tasks\":\"3\",\"cpu\":\"512\",\"memory\":\"1024\"}],\"serviceDiscovery\":[{\"environment\":[\"test\",\"prod\"],\"namespace\":\"http://my-svc.my-app.local:5000\"}],\"variables\":[{\"environment\":\"prod\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"prod\"},{\"environment\":\"test\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"test\"}],\"resources\":{\"prod\":[{\"type\":\"AWS::EC2::SecurityGroupIngress\",\"physicalID\":\"ContainerSecurityGroupIngressFromPublicALB\"}],\"test\":[{\"type\":\"AWS::EC2::SecurityGroup\",\"physicalID\":\"sg-0758ed6b233743530\"}]}}\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			config := []*ServiceConfig{
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
			sds := []*ServiceDiscovery{
				{
					Environment: []string{"test", "prod"},
					Namespace:   "http://my-svc.my-app.local:5000",
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
			backendSvc := &backendSvcDesc{
				Service:          "my-svc",
				Type:             "Backend Service",
				Configurations:   config,
				App:              "my-app",
				Variables:        envVars,
				ServiceDiscovery: sds,
				Resources:        resources,
			}
			human := backendSvc.HumanString()
			json, _ := backendSvc.JSONString()

			require.Equal(t, tc.wantedHumanString, human)
			require.Equal(t, tc.wantedJSONString, json)
		})
	}
}

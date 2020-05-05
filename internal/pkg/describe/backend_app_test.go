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

type backendAppDescriberMocks struct {
	storeSvc     *mocks.MockstoreSvc
	appDescriber *mocks.MockappDescriber
}

func TestBackendAppDescriber_Describe(t *testing.T) {
	const (
		testProject = "phonetool"
		testEnv     = "test"
		testApp     = "jobs"
		testAppPath = "*"
		prodEnv     = "prod"
		prodAppPath = "*"
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

		setupMocks func(mocks backendAppDescriberMocks)

		wantedBackendApp *backendAppDesc
		wantedError      error
	}{
		"return error if fail to list environment": {
			setupMocks: func(m backendAppDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("list environments for project phonetool: some error"),
		},
		"return error if fail to retrieve application deployment configuration": {
			setupMocks: func(m backendAppDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
						&testEnvironment,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve application deployment configuration: some error"),
		},
		"return error if fail to retrieve environment variables": {
			setupMocks: func(m backendAppDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
						&testEnvironment,
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
		"skip if not deployed": {
			shouldOutputResources: true,
			setupMocks: func(m backendAppDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
						&testEnvironment,
					}, nil),
					m.appDescriber.EXPECT().Params().Return(nil, mockNotExistErr),
					m.appDescriber.EXPECT().AppStackResources().Return(nil, mockNotExistErr),
				)
			},
			wantedBackendApp: &backendAppDesc{
				AppName:          testApp,
				Type:             "",
				Project:          testProject,
				Configurations:   []*AppConfig(nil),
				ServiceDiscovery: []*ServiceDiscovery(nil),
				Variables:        []*EnvVars(nil),
				Resources:        make(map[string][]*CfnResource),
			},
		},
		"success": {
			shouldOutputResources: true,
			setupMocks: func(m backendAppDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironments(testProject).Return([]*archer.Environment{
						&testEnvironment,
						&prodEnvironment,
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
			wantedBackendApp: &backendAppDesc{
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
			mocks := backendAppDescriberMocks{
				storeSvc:     mockStore,
				appDescriber: mockAppDescriber,
			}

			tc.setupMocks(mocks)

			d := &BackendAppDescriber{
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
			backendapp, err := d.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedBackendApp, backendapp, "expected output content match")
			}
		})
	}
}

func TestBackendAppDesc_String(t *testing.T) {
	testCases := map[string]struct {
		wantedHumanString string
		wantedJSONString  string
	}{
		"correct output": {
			wantedHumanString: `About

  Project           my-project
  Name              my-app
  Type              Backend Service

Configurations

  Environment       Tasks               CPU (vCPU)          Memory (MiB)        Port
  test              1                   0.25                512                 80
  prod              3                   0.5                 1024                5000

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
			wantedJSONString: "{\"appName\":\"my-app\",\"type\":\"Backend Service\",\"project\":\"my-project\",\"configurations\":[{\"environment\":\"test\",\"port\":\"80\",\"tasks\":\"1\",\"cpu\":\"256\",\"memory\":\"512\"},{\"environment\":\"prod\",\"port\":\"5000\",\"tasks\":\"3\",\"cpu\":\"512\",\"memory\":\"1024\"}],\"serviceDiscovery\":[{\"environment\":[\"test\",\"prod\"],\"namespace\":\"http://my-app.my-project.local:5000\"}],\"variables\":[{\"environment\":\"prod\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"prod\"},{\"environment\":\"test\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"test\"}],\"resources\":{\"prod\":[{\"type\":\"AWS::EC2::SecurityGroupIngress\",\"physicalID\":\"ContainerSecurityGroupIngressFromPublicALB\"}],\"test\":[{\"type\":\"AWS::EC2::SecurityGroup\",\"physicalID\":\"sg-0758ed6b233743530\"}]}}\n",
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
			backendApp := &backendAppDesc{
				AppName:          "my-app",
				Type:             "Backend Service",
				Configurations:   config,
				Project:          "my-project",
				Variables:        envVars,
				ServiceDiscovery: sds,
				Resources:        resources,
			}
			human := backendApp.HumanString()
			json, _ := backendApp.JSONString()

			require.Equal(t, tc.wantedHumanString, human)
			require.Equal(t, tc.wantedJSONString, json)
		})
	}
}

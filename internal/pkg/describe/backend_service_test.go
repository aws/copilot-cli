// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type backendSvcDescriberMocks struct {
	storeSvc     *mocks.MockDeployedEnvServicesLister
	svcDescriber *mocks.MocksvcDescriber
}

func TestBackendServiceDescriber_Describe(t *testing.T) {
	const (
		testApp = "phonetool"
		testEnv = "test"
		testSvc = "jobs"
		prodEnv = "prod"
		mockEnv = "mockEnv"
	)
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		shouldOutputResources bool

		setupMocks func(mocks backendSvcDescriberMocks)

		wantedBackendSvc *backendSvcDesc
		wantedError      error
	}{
		"return error if fail to list environment": {
			setupMocks: func(m backendSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("list deployed environments for application phonetool: some error"),
		},
		"return error if fail to retrieve service deployment configuration": {
			setupMocks: func(m backendSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.svcDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service deployment configuration: some error"),
		},
		"return error if fail to retrieve environment variables": {
			setupMocks: func(m backendSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "80",
						stack.WorkloadTaskCountParamKey:         "1",
						stack.WorkloadTaskCPUParamKey:           "256",
						stack.WorkloadTaskMemoryParamKey:        "512",
					}, nil),
					m.svcDescriber.EXPECT().EnvVars().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve environment variables: some error"),
		},
		"return error if fail to retrieve secrets": {
			setupMocks: func(m backendSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),

					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "80",
						stack.WorkloadTaskCountParamKey:         "1",
						stack.WorkloadTaskCPUParamKey:           "256",
						stack.WorkloadTaskMemoryParamKey:        "512",
					}, nil),
					m.svcDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     "prod",
						},
					}, nil),
					m.svcDescriber.EXPECT().Secrets().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve secrets: some error"),
		},
		"success": {
			shouldOutputResources: true,
			setupMocks: func(m backendSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv, prodEnv, mockEnv}, nil),

					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "5000",
						stack.WorkloadTaskCountParamKey:         "1",
						stack.WorkloadTaskCPUParamKey:           "256",
						stack.WorkloadTaskMemoryParamKey:        "512",
					}, nil),
					m.svcDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     testEnv,
						},
					}, nil),
					m.svcDescriber.EXPECT().Secrets().Return([]*ecs.ContainerSecret{
						{
							Name:      "GITHUB_WEBHOOK_SECRET",
							Container: "container",
							ValueFrom: "GH_WEBHOOK_SECRET",
						},
					}, nil),
					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "5000",
						stack.WorkloadTaskCountParamKey:         "2",
						stack.WorkloadTaskCPUParamKey:           "512",
						stack.WorkloadTaskMemoryParamKey:        "1024",
					}, nil),
					m.svcDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     prodEnv,
						},
					}, nil),
					m.svcDescriber.EXPECT().Secrets().Return([]*ecs.ContainerSecret{
						{
							Name:      "SOME_OTHER_SECRET",
							Container: "container",
							ValueFrom: "SHHHHHHHH",
						},
					}, nil),
					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "-1",
						stack.WorkloadTaskCountParamKey:         "2",
						stack.WorkloadTaskCPUParamKey:           "512",
						stack.WorkloadTaskMemoryParamKey:        "1024",
					}, nil),
					m.svcDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     mockEnv,
						},
					}, nil),
					m.svcDescriber.EXPECT().Secrets().Return(
						nil, nil),
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
					m.svcDescriber.EXPECT().ServiceStackResources().Return([]*cloudformation.StackResource{
						{
							ResourceType:       aws.String("AWS::EC2::SecurityGroup"),
							PhysicalResourceId: aws.String("sg-2337435300758ed6b"),
						},
					}, nil),
				)
			},
			wantedBackendSvc: &backendSvcDesc{
				Service: testSvc,
				Type:    "Backend Service",
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
					{
						CPU:         "512",
						Environment: "mockEnv",
						Memory:      "1024",
						Port:        "-",
						Tasks:       "2",
					},
				},
				ServiceDiscovery: []*ServiceDiscovery{
					{
						Environment: []string{"test", "prod"},
						Namespace:   "jobs.phonetool.local:5000",
					},
				},
				Variables: []*envVar{
					{
						Container:   "container",
						Environment: "test",
						Name:        "COPILOT_ENVIRONMENT_NAME",
						Value:       "test",
					},
					{
						Container:   "container",
						Environment: "prod",
						Name:        "COPILOT_ENVIRONMENT_NAME",
						Value:       "prod",
					},
					{
						Container:   "container",
						Environment: "mockEnv",
						Name:        "COPILOT_ENVIRONMENT_NAME",
						Value:       "mockEnv",
					},
				},
				Secrets: []*secret{
					{
						Name:        "GITHUB_WEBHOOK_SECRET",
						Container:   "container",
						Environment: "test",
						ValueFrom:   "GH_WEBHOOK_SECRET",
					},
					{
						Name:        "SOME_OTHER_SECRET",
						Container:   "container",
						Environment: "prod",
						ValueFrom:   "SHHHHHHHH",
					},
				},
				Resources: map[string][]*CfnResource{
					"test": {
						{
							Type:       "AWS::EC2::SecurityGroupIngress",
							PhysicalID: "ContainerSecurityGroupIngressFromPublicALB",
						},
					},
					"prod": {
						{
							Type:       "AWS::EC2::SecurityGroup",
							PhysicalID: "sg-0758ed6b233743530",
						},
					},
					"mockEnv": {
						{
							Type:       "AWS::EC2::SecurityGroup",
							PhysicalID: "sg-2337435300758ed6b",
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

			mockStore := mocks.NewMockDeployedEnvServicesLister(ctrl)
			mockSvcDescriber := mocks.NewMocksvcDescriber(ctrl)
			mocks := backendSvcDescriberMocks{
				storeSvc:     mockStore,
				svcDescriber: mockSvcDescriber,
			}

			tc.setupMocks(mocks)

			d := &BackendServiceDescriber{
				app:             testApp,
				svc:             testSvc,
				enableResources: tc.shouldOutputResources,
				store:           mockStore,
				svcDescriber: map[string]svcDescriber{
					"test":    mockSvcDescriber,
					"prod":    mockSvcDescriber,
					"mockEnv": mockSvcDescriber,
				},
				initServiceDescriber: func(string) error { return nil },
			}

			// WHEN
			backendsvc, err := d.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
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
  -----------       -----               ----------          ------------        ----
  test              1                   0.25                512                 80
  prod              3                   0.5                 1024                5000

Service Discovery

  Environment       Namespace
  -----------       ---------
  test, prod        http://my-svc.my-app.local:5000

Variables

  Name                      Container           Environment         Value
  ----                      ---------           -----------         -----
  COPILOT_ENVIRONMENT_NAME  container           prod                prod
    "                         "                 test                test

Secrets

  Name                   Container           Environment         Value From
  ----                   ---------           -----------         ----------
  GITHUB_WEBHOOK_SECRET  container           test                parameter/GH_WEBHOOK_SECRET
  SOME_OTHER_SECRET        "                 prod                parameter/SHHHHH

Resources

  test
    AWS::EC2::SecurityGroup  sg-0758ed6b233743530

  prod
    AWS::EC2::SecurityGroupIngress  ContainerSecurityGroupIngressFromPublicALB
`,
			wantedJSONString: "{\"service\":\"my-svc\",\"type\":\"Backend Service\",\"application\":\"my-app\",\"configurations\":[{\"environment\":\"test\",\"port\":\"80\",\"tasks\":\"1\",\"cpu\":\"256\",\"memory\":\"512\"},{\"environment\":\"prod\",\"port\":\"5000\",\"tasks\":\"3\",\"cpu\":\"512\",\"memory\":\"1024\"}],\"serviceDiscovery\":[{\"environment\":[\"test\",\"prod\"],\"namespace\":\"http://my-svc.my-app.local:5000\"}],\"variables\":[{\"environment\":\"prod\",\"container\":\"container\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"prod\"},{\"environment\":\"test\",\"container\":\"container\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"test\"}],\"secrets\":[{\"name\":\"GITHUB_WEBHOOK_SECRET\",\"container\":\"container\",\"environment\":\"test\",\"valueFrom\":\"GH_WEBHOOK_SECRET\"},{\"name\":\"SOME_OTHER_SECRET\",\"container\":\"container\",\"environment\":\"prod\",\"valueFrom\":\"SHHHHH\"}],\"resources\":{\"prod\":[{\"type\":\"AWS::EC2::SecurityGroupIngress\",\"physicalID\":\"ContainerSecurityGroupIngressFromPublicALB\"}],\"test\":[{\"type\":\"AWS::EC2::SecurityGroup\",\"physicalID\":\"sg-0758ed6b233743530\"}]}}\n",
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
			envVars := []*envVar{
				{
					Container:   "container",
					Environment: "prod",
					Name:        "COPILOT_ENVIRONMENT_NAME",
					Value:       "prod",
				},
				{
					Container:   "container",
					Environment: "test",
					Name:        "COPILOT_ENVIRONMENT_NAME",
					Value:       "test",
				},
			}
			secrets := []*secret{
				{
					Name:        "GITHUB_WEBHOOK_SECRET",
					Container:   "container",
					Environment: "test",
					ValueFrom:   "GH_WEBHOOK_SECRET",
				},
				{
					Name:        "SOME_OTHER_SECRET",
					Container:   "container",
					Environment: "prod",
					ValueFrom:   "SHHHHH",
				},
			}
			sds := []*ServiceDiscovery{
				{
					Environment: []string{"test", "prod"},
					Namespace:   "http://my-svc.my-app.local:5000",
				},
			}
			resources := map[string][]*CfnResource{
				"test": {
					{
						PhysicalID: "sg-0758ed6b233743530",
						Type:       "AWS::EC2::SecurityGroup",
					},
				},
				"prod": {
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
				Secrets:          secrets,
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

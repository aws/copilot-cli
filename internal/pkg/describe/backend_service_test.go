// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"

	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

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

		setupMocks func(mocks lbWebSvcDescriberMocks)

		wantedBackendSvc *backendSvcDesc
		wantedError      error
	}{
		"return error if fail to list environment": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("list deployed environments for application phonetool: some error"),
		},
		"return error if fail to retrieve service deployment configuration": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsStackDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get stack parameters for environment test: some error"),
		},
		"return error if fail to retrieve svc discovery endpoint": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsStackDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceContainerPortParamKey: "80",
						cfnstack.WorkloadTaskCountParamKey:         "1",
						cfnstack.WorkloadTaskMemoryParamKey:        "512",
						cfnstack.WorkloadTaskCPUParamKey:           "256",
					}, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("", errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("some error"),
		},
		"return error if fail to retrieve platform": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsStackDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceContainerPortParamKey: "5000",
						cfnstack.WorkloadTaskCountParamKey:         "1",
						cfnstack.WorkloadTaskCPUParamKey:           "256",
						cfnstack.WorkloadTaskMemoryParamKey:        "512",
					}, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsStackDescriber.EXPECT().Platform().Return(nil, errors.New("some error")),
					)
			},
			wantedError: fmt.Errorf("retrieve platform: some error"),
		},
		"return error if fail to retrieve environment variables": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsStackDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceContainerPortParamKey: "80",
						cfnstack.WorkloadTaskCountParamKey:         "1",
						cfnstack.WorkloadTaskMemoryParamKey:        "512",
						cfnstack.WorkloadTaskCPUParamKey:           "256",
					}, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsStackDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture: "X86_64",
					}, nil),
					m.ecsStackDescriber.EXPECT().EnvVars().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve environment variables: some error"),
		},
		"return error if fail to retrieve secrets": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),

					m.ecsStackDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceContainerPortParamKey: "80",
						cfnstack.WorkloadTaskCountParamKey:         "1",
						cfnstack.WorkloadTaskCPUParamKey:           "256",
						cfnstack.WorkloadTaskMemoryParamKey:        "512",
					}, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsStackDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture: "X86_64",
					}, nil),
					m.ecsStackDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     "prod",
						},
					}, nil),
					m.ecsStackDescriber.EXPECT().Secrets().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve secrets: some error"),
		},
		"success": {
			shouldOutputResources: true,
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv, prodEnv, mockEnv}, nil),

					m.ecsStackDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceContainerPortParamKey: "5000",
						cfnstack.WorkloadTaskCountParamKey:         "1",
						cfnstack.WorkloadTaskCPUParamKey:           "256",
						cfnstack.WorkloadTaskMemoryParamKey:        "512",
					}, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsStackDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture: "X86_64",
					}, nil),
					m.ecsStackDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     testEnv,
						},
					}, nil),
					m.ecsStackDescriber.EXPECT().Secrets().Return([]*ecs.ContainerSecret{
						{
							Name:      "GITHUB_WEBHOOK_SECRET",
							Container: "container",
							ValueFrom: "GH_WEBHOOK_SECRET",
						},
					}, nil),
					m.ecsStackDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceContainerPortParamKey: "5000",
						cfnstack.WorkloadTaskCountParamKey:         "2",
						cfnstack.WorkloadTaskCPUParamKey:           "512",
						cfnstack.WorkloadTaskMemoryParamKey:        "1024",
					}, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("prod.phonetool.local", nil),
					m.ecsStackDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture: "X86_64",
					}, nil),
					m.ecsStackDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     prodEnv,
						},
					}, nil),
					m.ecsStackDescriber.EXPECT().Secrets().Return([]*ecs.ContainerSecret{
						{
							Name:      "SOME_OTHER_SECRET",
							Container: "container",
							ValueFrom: "SHHHHHHHH",
						},
					}, nil),
					m.ecsStackDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceContainerPortParamKey: "-1",
						cfnstack.WorkloadTaskCountParamKey:         "2",
						cfnstack.WorkloadTaskCPUParamKey:           "512",
						cfnstack.WorkloadTaskMemoryParamKey:        "1024",
					}, nil),
					m.ecsStackDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture: "X86_64",
					}, nil),
					m.ecsStackDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     mockEnv,
						},
					}, nil),
					m.ecsStackDescriber.EXPECT().Secrets().Return(
						nil, nil),
					m.ecsStackDescriber.EXPECT().ServiceStackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::EC2::SecurityGroupIngress",
							PhysicalID: "ContainerSecurityGroupIngressFromPublicALB",
						},
					}, nil),
					m.ecsStackDescriber.EXPECT().ServiceStackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::EC2::SecurityGroup",
							PhysicalID: "sg-0758ed6b233743530",
						},
					}, nil),
					m.ecsStackDescriber.EXPECT().ServiceStackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::EC2::SecurityGroup",
							PhysicalID: "sg-2337435300758ed6b",
						},
					}, nil),
				)
			},
			wantedBackendSvc: &backendSvcDesc{
				Service: testSvc,
				Type:    "Backend Service",
				Platform: "LINUX/X86_64",
				App:     testApp,
				Configurations: []*ECSServiceConfig{
					{
						ServiceConfig: &ServiceConfig{
							CPU:         "256",
							Environment: "test",
							Memory:      "512",
							Port:        "5000",
						},
						Tasks: "1",
					},
					{
						ServiceConfig: &ServiceConfig{
							CPU:         "512",
							Environment: "prod",
							Memory:      "1024",
							Port:        "5000",
						},
						Tasks: "2",
					},
					{
						ServiceConfig: &ServiceConfig{
							CPU:         "512",
							Environment: "mockEnv",
							Memory:      "1024",
							Port:        "-",
						},
						Tasks: "2",
					},
				},
				ServiceDiscovery: []*ServiceDiscovery{
					{
						Environment: []string{"test"},
						Namespace:   "jobs.test.phonetool.local:5000",
					},
					{
						Environment: []string{"prod"},
						Namespace:   "jobs.prod.phonetool.local:5000",
					},
				},
				Variables: []*containerEnvVar{
					{
						envVar: &envVar{
							Environment: "test",
							Name:        "COPILOT_ENVIRONMENT_NAME",
							Value:       "test",
						},
						Container: "container",
					},
					{
						envVar: &envVar{
							Environment: "prod",
							Name:        "COPILOT_ENVIRONMENT_NAME",
							Value:       "prod",
						},
						Container: "container",
					},
					{
						envVar: &envVar{
							Environment: "mockEnv",
							Name:        "COPILOT_ENVIRONMENT_NAME",
							Value:       "mockEnv",
						},
						Container: "container",
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
				Resources: map[string][]*stack.Resource{
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
				environments: []string{"test", "prod", "mockEnv"},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockDeployedEnvServicesLister(ctrl)
			mockSvcDescriber := mocks.NewMockecsStackDescriber(ctrl)
			mockEnvDescriber := mocks.NewMockenvDescriber(ctrl)
			mocks := lbWebSvcDescriberMocks{
				storeSvc:          mockStore,
				ecsStackDescriber: mockSvcDescriber,
				envDescriber:      mockEnvDescriber,
			}

			tc.setupMocks(mocks)

			d := &BackendServiceDescriber{
				ecsServiceDescriber: &ecsServiceDescriber{
					app:             testApp,
					svc:             testSvc,
					enableResources: tc.shouldOutputResources,
					store:           mockStore,
					svcStackDescriber: map[string]ecsStackDescriber{
						"test":    mockSvcDescriber,
						"prod":    mockSvcDescriber,
						"mockEnv": mockSvcDescriber,
					},
					initDescribers: func(string) error { return nil },
				},
				envDescriber: map[string]envDescriber{
					"test":    mockEnvDescriber,
					"prod":    mockEnvDescriber,
					"mockEnv": mockEnvDescriber,
				},
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
  test              http://my-svc.test.my-app.local:5000
  prod              http://my-svc.prod.my-app.local:5000

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
			wantedJSONString: "{\"service\":\"my-svc\",\"type\":\"Backend Service\",\"application\":\"my-app\",\"configurations\":[{\"environment\":\"test\",\"port\":\"80\",\"cpu\":\"256\",\"memory\":\"512\",\"tasks\":\"1\"},{\"environment\":\"prod\",\"port\":\"5000\",\"cpu\":\"512\",\"memory\":\"1024\",\"tasks\":\"3\"}],\"serviceDiscovery\":[{\"environment\":[\"test\"],\"namespace\":\"http://my-svc.test.my-app.local:5000\"},{\"environment\":[\"prod\"],\"namespace\":\"http://my-svc.prod.my-app.local:5000\"}],\"variables\":[{\"environment\":\"prod\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"prod\",\"container\":\"container\"},{\"environment\":\"test\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"test\",\"container\":\"container\"}],\"secrets\":[{\"name\":\"GITHUB_WEBHOOK_SECRET\",\"container\":\"container\",\"environment\":\"test\",\"valueFrom\":\"GH_WEBHOOK_SECRET\"},{\"name\":\"SOME_OTHER_SECRET\",\"container\":\"container\",\"environment\":\"prod\",\"valueFrom\":\"SHHHHH\"}],\"resources\":{\"prod\":[{\"type\":\"AWS::EC2::SecurityGroupIngress\",\"physicalID\":\"ContainerSecurityGroupIngressFromPublicALB\"}],\"test\":[{\"type\":\"AWS::EC2::SecurityGroup\",\"physicalID\":\"sg-0758ed6b233743530\"}]}}\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			config := []*ECSServiceConfig{
				{
					ServiceConfig: &ServiceConfig{
						CPU:         "256",
						Environment: "test",
						Memory:      "512",
						Port:        "80",
					},
					Tasks: "1",
				},
				{
					ServiceConfig: &ServiceConfig{
						CPU:         "512",
						Environment: "prod",
						Memory:      "1024",
						Port:        "5000",
					},
					Tasks: "3",
				},
			}
			envVars := []*containerEnvVar{
				{
					envVar: &envVar{
						Environment: "prod",
						Name:        "COPILOT_ENVIRONMENT_NAME",
						Value:       "prod",
					},
					Container: "container",
				},
				{
					envVar: &envVar{
						Environment: "test",
						Name:        "COPILOT_ENVIRONMENT_NAME",
						Value:       "test",
					},
					Container: "container",
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
					Environment: []string{"test"},
					Namespace:   "http://my-svc.test.my-app.local:5000",
				},
				{
					Environment: []string{"prod"},
					Namespace:   "http://my-svc.prod.my-app.local:5000",
				},
			}
			resources := map[string][]*stack.Resource{
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
				environments:     []string{"test", "prod"},
			}
			human := backendSvc.HumanString()
			json, _ := backendSvc.JSONString()

			require.Equal(t, tc.wantedHumanString, human)
			require.Equal(t, tc.wantedJSONString, json)
		})
	}
}

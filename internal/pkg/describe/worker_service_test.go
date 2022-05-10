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

func TestWorkerServiceDescriber_Describe(t *testing.T) {
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

		wantedWorkerSvc *workerSvcDesc
		wantedError     error
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
					m.ecsDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get stack parameters for environment test: some error"),
		},
		"return error if fail to retrieve platform": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.WorkloadContainerPortParamKey: "-",
						cfnstack.WorkloadTaskCountParamKey:     "1",
						cfnstack.WorkloadTaskCPUParamKey:       "256",
						cfnstack.WorkloadTaskMemoryParamKey:    "512",
					}, nil),
					m.ecsDescriber.EXPECT().Platform().Return(nil, errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("retrieve platform: some error"),
		},
		"return error if fail to retrieve environment variables": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.WorkloadContainerPortParamKey: "-",
						cfnstack.WorkloadTaskCountParamKey:     "1",
						cfnstack.WorkloadTaskMemoryParamKey:    "512",
						cfnstack.WorkloadTaskCPUParamKey:       "256",
					}, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "X86_64",
					}, nil),
					m.ecsDescriber.EXPECT().EnvVars().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve environment variables: some error"),
		},
		"return error if fail to retrieve secrets": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),

					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.WorkloadContainerPortParamKey: "-",
						cfnstack.WorkloadTaskCountParamKey:     "1",
						cfnstack.WorkloadTaskCPUParamKey:       "256",
						cfnstack.WorkloadTaskMemoryParamKey:    "512",
					}, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "X86_64",
					}, nil),
					m.ecsDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     "prod",
						},
					}, nil),
					m.ecsDescriber.EXPECT().Secrets().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve secrets: some error"),
		},
		"success": {
			shouldOutputResources: true,
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv, prodEnv, mockEnv}, nil),

					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.WorkloadContainerPortParamKey: "-",
						cfnstack.WorkloadTaskCountParamKey:     "1",
						cfnstack.WorkloadTaskCPUParamKey:       "256",
						cfnstack.WorkloadTaskMemoryParamKey:    "512",
					}, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "X86_64",
					}, nil),
					m.ecsDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     testEnv,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Secrets().Return([]*ecs.ContainerSecret{
						{
							Name:      "GITHUB_WEBHOOK_SECRET",
							Container: "container",
							ValueFrom: "GH_WEBHOOK_SECRET",
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.WorkloadContainerPortParamKey: "-",
						cfnstack.WorkloadTaskCountParamKey:     "2",
						cfnstack.WorkloadTaskCPUParamKey:       "512",
						cfnstack.WorkloadTaskMemoryParamKey:    "1024",
					}, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "ARM64",
					}, nil),
					m.ecsDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     prodEnv,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Secrets().Return([]*ecs.ContainerSecret{
						{
							Name:      "A_SECRET",
							Container: "container",
							ValueFrom: "SECRET",
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.WorkloadContainerPortParamKey: "-",
						cfnstack.WorkloadTaskCountParamKey:     "2",
						cfnstack.WorkloadTaskCPUParamKey:       "512",
						cfnstack.WorkloadTaskMemoryParamKey:    "1024",
					}, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "X86_64",
					}, nil),
					m.ecsDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     mockEnv,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Secrets().Return(
						nil, nil),
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::EC2::SecurityGroupIngress",
							PhysicalID: "ContainerSecurityGroupIngressFromPublicALB",
						},
					}, nil),
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::EC2::SecurityGroup",
							PhysicalID: "sg-0758ed6b233743530",
						},
					}, nil),
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::EC2::SecurityGroup",
							PhysicalID: "sg-2337435300758ed6b",
						},
					}, nil),
				)
			},
			wantedWorkerSvc: &workerSvcDesc{
				Service: testSvc,
				Type:    "Worker Service",
				App:     testApp,
				Configurations: []*ECSServiceConfig{
					{
						ServiceConfig: &ServiceConfig{
							CPU:         "256",
							Environment: "test",
							Memory:      "512",
							Platform:    "LINUX/X86_64",
							Port:        "-",
						},
						Tasks: "1",
					},
					{
						ServiceConfig: &ServiceConfig{
							CPU:         "512",
							Environment: "prod",
							Memory:      "1024",
							Platform:    "LINUX/ARM64",
							Port:        "-",
						},
						Tasks: "2",
					},
					{
						ServiceConfig: &ServiceConfig{
							CPU:         "512",
							Environment: "mockEnv",
							Memory:      "1024",
							Platform:    "LINUX/X86_64",
							Port:        "-",
						},
						Tasks: "2",
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
						Name:        "A_SECRET",
						Container:   "container",
						Environment: "prod",
						ValueFrom:   "SECRET",
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
			mockSvcDescriber := mocks.NewMockecsDescriber(ctrl)
			mocks := lbWebSvcDescriberMocks{
				storeSvc:     mockStore,
				ecsDescriber: mockSvcDescriber,
			}

			tc.setupMocks(mocks)

			d := &WorkerServiceDescriber{
				app:              testApp,
				svc:              testSvc,
				enableResources:  tc.shouldOutputResources,
				store:            mockStore,
				initECSDescriber: func(s string) (ecsDescriber, error) { return mockSvcDescriber, nil },
			}

			// WHEN
			workersvc, err := d.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedWorkerSvc, workersvc, "expected output content match")
			}
		})
	}
}

func TestWorkerSvcDesc_String(t *testing.T) {
	testCases := map[string]struct {
		wantedHumanString string
		wantedJSONString  string
	}{
		"correct output": {
			wantedHumanString: `About

  Application  my-app
  Name         my-svc
  Type         Worker Service

Configurations

  Environment  Tasks     CPU (vCPU)  Memory (MiB)  Platform      Port
  -----------  -----     ----------  ------------  --------      ----
  test         1         0.25        512           LINUX/X86_64  -
  prod         3         0.5         1024          LINUX/ARM64     "

Variables

  Name                      Container  Environment  Value
  ----                      ---------  -----------  -----
  COPILOT_ENVIRONMENT_NAME  container  prod         prod
    "                         "        test         test

Secrets

  Name                   Container  Environment  Value From
  ----                   ---------  -----------  ----------
  A_SECRET               container  prod         parameter/SECRET
  GITHUB_WEBHOOK_SECRET    "        test         parameter/GH_WEBHOOK_SECRET

Resources

  test
    AWS::EC2::SecurityGroup  sg-0758ed6b233743530

  prod
    AWS::EC2::SecurityGroupIngress  ContainerSecurityGroupIngressFromPublicALB
`,
			wantedJSONString: "{\"service\":\"my-svc\",\"type\":\"Worker Service\",\"application\":\"my-app\",\"configurations\":[{\"environment\":\"test\",\"port\":\"-\",\"cpu\":\"256\",\"memory\":\"512\",\"platform\":\"LINUX/X86_64\",\"tasks\":\"1\"},{\"environment\":\"prod\",\"port\":\"-\",\"cpu\":\"512\",\"memory\":\"1024\",\"platform\":\"LINUX/ARM64\",\"tasks\":\"3\"}],\"variables\":[{\"environment\":\"prod\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"prod\",\"container\":\"container\"},{\"environment\":\"test\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"test\",\"container\":\"container\"}],\"secrets\":[{\"name\":\"A_SECRET\",\"container\":\"container\",\"environment\":\"prod\",\"valueFrom\":\"SECRET\"},{\"name\":\"GITHUB_WEBHOOK_SECRET\",\"container\":\"container\",\"environment\":\"test\",\"valueFrom\":\"GH_WEBHOOK_SECRET\"}],\"resources\":{\"prod\":[{\"type\":\"AWS::EC2::SecurityGroupIngress\",\"physicalID\":\"ContainerSecurityGroupIngressFromPublicALB\"}],\"test\":[{\"type\":\"AWS::EC2::SecurityGroup\",\"physicalID\":\"sg-0758ed6b233743530\"}]}}\n",
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
						Platform:    "LINUX/X86_64",
						Port:        "-",
					},
					Tasks: "1",
				},
				{
					ServiceConfig: &ServiceConfig{
						CPU:         "512",
						Environment: "prod",
						Memory:      "1024",
						Platform:    "LINUX/ARM64",
						Port:        "-",
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
					Name:        "A_SECRET",
					Container:   "container",
					Environment: "prod",
					ValueFrom:   "SECRET",
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
			workerSvc := &workerSvcDesc{
				Service:        "my-svc",
				Type:           "Worker Service",
				Configurations: config,
				App:            "my-app",
				Variables:      envVars,
				Secrets:        secrets,
				Resources:      resources,
				environments:   []string{"test", "prod"},
			}
			human := workerSvc.HumanString()
			json, _ := workerSvc.JSONString()

			require.Equal(t, tc.wantedHumanString, human)
			require.Equal(t, tc.wantedJSONString, json)
		})
	}
}

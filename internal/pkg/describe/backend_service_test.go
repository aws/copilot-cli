// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatch"

	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	cfnstack "github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	describeStack "github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type backendSvcDescriberMocks struct {
	storeSvc     *mocks.MockDeployedEnvServicesLister
	ecsDescriber *mocks.MockecsDescriber
	cwDescriber  *mocks.MockcwAlarmDescriber
	envDescriber *mocks.MockenvDescriber
	lbDescriber  *mocks.MocklbDescriber
}

func TestBackendServiceDescriber_Describe(t *testing.T) {
	const (
		testApp = "phonetool"
		testEnv = "test"
		testSvc = "jobs"
		prodEnv = "prod"
		mockEnv = "mockEnv"
		alarm1  = "alarm1"
		alarm2  = "alarm2"
		desc1   = "alarm description 1"
		desc2   = "alarm description 2"
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
					m.ecsDescriber.EXPECT().StackResources().Return(nil, nil),
					m.ecsDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service URI: get stack parameters for environment test: some error"),
		},
		"return error if fail to retrieve svc discovery endpoint": {
			setupMocks: func(m backendSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return(nil, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.WorkloadTargetPortParamKey: "80",
						cfnstack.WorkloadTaskCountParamKey:  "1",
						cfnstack.WorkloadTaskMemoryParamKey: "512",
						cfnstack.WorkloadTaskCPUParamKey:    "256",
					}, nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("", mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service URI: retrieve service discovery endpoint for environment test: some error"),
		},
		"return error if fail to retrieve service connect dns names": {
			setupMocks: func(m backendSvcDescriberMocks) {
				params := map[string]string{
					cfnstack.WorkloadTargetPortParamKey: "5000",
					cfnstack.WorkloadTaskCountParamKey:  "1",
					cfnstack.WorkloadTaskCPUParamKey:    "256",
					cfnstack.WorkloadTaskMemoryParamKey: "512",
				}
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return(nil, nil),
					m.ecsDescriber.EXPECT().Params().Return(params, nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().Params().Return(params, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service connect DNS names: some error"),
		},
		"return error if fail to retrieve platform": {
			setupMocks: func(m backendSvcDescriberMocks) {
				params := map[string]string{
					cfnstack.WorkloadTargetPortParamKey: "5000",
					cfnstack.WorkloadTaskCountParamKey:  "1",
					cfnstack.WorkloadTaskCPUParamKey:    "256",
					cfnstack.WorkloadTaskMemoryParamKey: "512",
				}
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return(nil, nil),
					m.ecsDescriber.EXPECT().Params().Return(params, nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().Params().Return(params, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.ecsDescriber.EXPECT().Platform().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve platform: some error"),
		},
		"return error if fail to retrieve rollback alarm names": {
			setupMocks: func(m backendSvcDescriberMocks) {
				params := map[string]string{
					cfnstack.WorkloadTargetPortParamKey: "5000",
					cfnstack.WorkloadTaskCountParamKey:  "1",
					cfnstack.WorkloadTaskCPUParamKey:    "256",
					cfnstack.WorkloadTaskMemoryParamKey: "512",
				}
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return(nil, nil),
					m.ecsDescriber.EXPECT().Params().Return(params, nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().Params().Return(params, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "X86_64",
					}, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve rollback alarm names: some error"),
		},
		"return error if fail to retrieve rollback alarm descriptions": {
			setupMocks: func(m backendSvcDescriberMocks) {
				params := map[string]string{
					cfnstack.WorkloadTargetPortParamKey: "5000",
					cfnstack.WorkloadTaskCountParamKey:  "1",
					cfnstack.WorkloadTaskCPUParamKey:    "256",
					cfnstack.WorkloadTaskMemoryParamKey: "512",
				}
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return(nil, nil),
					m.ecsDescriber.EXPECT().Params().Return(params, nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().Params().Return(params, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "X86_64",
					}, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return([]string{alarm1, alarm2}, nil),
					m.cwDescriber.EXPECT().AlarmDescriptions([]string{alarm1, alarm2}).Return(nil, errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("retrieve alarm descriptions: some error"),
		},
		"return error if fail to retrieve environment variables": {
			setupMocks: func(m backendSvcDescriberMocks) {
				params := map[string]string{
					cfnstack.WorkloadTargetPortParamKey: "5000",
					cfnstack.WorkloadTaskCountParamKey:  "1",
					cfnstack.WorkloadTaskCPUParamKey:    "256",
					cfnstack.WorkloadTaskMemoryParamKey: "512",
				}
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return(nil, nil),
					m.ecsDescriber.EXPECT().Params().Return(params, nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().Params().Return(params, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "X86_64",
					}, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return(nil, nil),
					m.ecsDescriber.EXPECT().EnvVars().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve environment variables: some error"),
		},
		"return error if fail to retrieve secrets": {
			setupMocks: func(m backendSvcDescriberMocks) {
				params := map[string]string{
					cfnstack.WorkloadTargetPortParamKey: "80",
					cfnstack.WorkloadTaskCountParamKey:  "1",
					cfnstack.WorkloadTaskCPUParamKey:    "256",
					cfnstack.WorkloadTaskMemoryParamKey: "512",
				}
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return(nil, nil),
					m.ecsDescriber.EXPECT().Params().Return(params, nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().Params().Return(params, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "X86_64",
					}, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return(nil, nil),
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
		"should not fetch descriptions if no ROLLBACK alarms present": {
			setupMocks: func(m backendSvcDescriberMocks) {
				testParams := map[string]string{
					cfnstack.WorkloadTargetPortParamKey: "5000",
					cfnstack.WorkloadTaskCountParamKey:  "1",
					cfnstack.WorkloadTaskCPUParamKey:    "256",
					cfnstack.WorkloadTaskMemoryParamKey: "512",
				}
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return(nil, nil),
					m.ecsDescriber.EXPECT().Params().Return(testParams, nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().Params().Return(testParams, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "X86_64",
					}, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return([]string{}, nil),
					m.ecsDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{}, nil),
					m.ecsDescriber.EXPECT().Secrets().Return([]*ecs.ContainerSecret{}, nil))
			},
			wantedBackendSvc: &backendSvcDesc{
				ecsSvcDesc: ecsSvcDesc{
					Service: testSvc,
					Type:    "Backend Service",
					App:     testApp,
					Configurations: []*ECSServiceConfig{
						{
							ServiceConfig: &ServiceConfig{
								CPU:         "256",
								Environment: "test",
								Memory:      "512",
								Platform:    "LINUX/X86_64",
								Port:        "5000",
							},
							Tasks: "1",
						},
					},
					ServiceDiscovery: serviceDiscoveries{
						"jobs.test.phonetool.local:5000": []string{"test"},
					},
					ServiceConnect: serviceConnects{},
					Resources:      map[string][]*stack.Resource{},
					environments:   []string{"test"},
				},
			},
		},
		"success": {
			shouldOutputResources: true,
			setupMocks: func(m backendSvcDescriberMocks) {
				testParams := map[string]string{
					cfnstack.WorkloadTargetPortParamKey: "5000",
					cfnstack.WorkloadTaskCountParamKey:  "1",
					cfnstack.WorkloadTaskCPUParamKey:    "256",
					cfnstack.WorkloadTaskMemoryParamKey: "512",
				}
				prodParams := map[string]string{
					cfnstack.WorkloadTargetPortParamKey: "5000",
					cfnstack.WorkloadTaskCountParamKey:  "2",
					cfnstack.WorkloadTaskCPUParamKey:    "512",
					cfnstack.WorkloadTaskMemoryParamKey: "1024",
				}
				mockParams := map[string]string{
					cfnstack.WorkloadTargetPortParamKey: "-1",
					cfnstack.WorkloadTaskCountParamKey:  "2",
					cfnstack.WorkloadTaskCPUParamKey:    "512",
					cfnstack.WorkloadTaskMemoryParamKey: "1024",
				}
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv, prodEnv, mockEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return(nil, nil),
					m.ecsDescriber.EXPECT().Params().Return(testParams, nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().Params().Return(testParams, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "X86_64",
					}, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return(nil, nil),
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
					m.ecsDescriber.EXPECT().StackResources().Return(nil, nil),
					m.ecsDescriber.EXPECT().Params().Return(prodParams, nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("prod.phonetool.local", nil),
					m.ecsDescriber.EXPECT().Params().Return(prodParams, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("prod.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "ARM64",
					}, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return([]string{alarm1}, nil),
					m.cwDescriber.EXPECT().AlarmDescriptions([]string{alarm1}).Return([]*cloudwatch.AlarmDescription{
						{
							Name:        alarm1,
							Description: desc1,
						},
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
							Name:      "SOME_OTHER_SECRET",
							Container: "container",
							ValueFrom: "SHHHHHHHH",
						},
					}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return(nil, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "X86_64",
					}, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return([]string{alarm2}, nil),
					m.cwDescriber.EXPECT().AlarmDescriptions([]string{alarm2}).Return([]*cloudwatch.AlarmDescription{
						{
							Name:        alarm2,
							Description: desc2,
						},
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
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::EC2::SecurityGroupIngress",
							PhysicalID: "ContainerSecurityGroupIngressFromPublicALB",
						},
					}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::EC2::SecurityGroup",
							PhysicalID: "sg-0758ed6b233743530",
						},
					}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::EC2::SecurityGroup",
							PhysicalID: "sg-2337435300758ed6b",
						},
					}, nil),
				)
			},
			wantedBackendSvc: &backendSvcDesc{
				ecsSvcDesc: ecsSvcDesc{
					Service: testSvc,
					Type:    "Backend Service",
					App:     testApp,
					Configurations: []*ECSServiceConfig{
						{
							ServiceConfig: &ServiceConfig{
								CPU:         "256",
								Environment: "test",
								Memory:      "512",
								Platform:    "LINUX/X86_64",
								Port:        "5000",
							},
							Tasks: "1",
						},
						{
							ServiceConfig: &ServiceConfig{
								CPU:         "512",
								Environment: "prod",
								Memory:      "1024",
								Platform:    "LINUX/ARM64",
								Port:        "5000",
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
					ServiceDiscovery: serviceDiscoveries{
						"jobs.test.phonetool.local:5000": []string{"test"},
						"jobs.prod.phonetool.local:5000": []string{"prod"},
					},
					ServiceConnect: serviceConnects{},
					AlarmDescriptions: []*cloudwatch.AlarmDescription{
						{
							Name:        alarm1,
							Description: desc1,
							Environment: prodEnv,
						},
						{
							Name:        alarm2,
							Description: desc2,
							Environment: mockEnv,
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
		},
		"internal alb success http": {
			shouldOutputResources: true,
			setupMocks: func(m backendSvcDescriberMocks) {
				params := map[string]string{
					cfnstack.WorkloadTargetPortParamKey: "5000",
					cfnstack.WorkloadTaskCountParamKey:  "1",
					cfnstack.WorkloadTaskCPUParamKey:    "256",
					cfnstack.WorkloadTaskMemoryParamKey: "512",
					cfnstack.WorkloadRulePathParamKey:   "mySvc",
				}
				resources := []*describeStack.Resource{
					{
						Type:       "AWS::ElasticLoadBalancingV2::TargetGroup",
						LogicalID:  svcStackResourceALBTargetGroupLogicalID,
						PhysicalID: "targetGroupARN",
					},
					{
						Type:       svcStackResourceListenerRuleResourceType,
						LogicalID:  svcStackResourceHTTPListenerRuleLogicalID,
						PhysicalID: "listenerRuleARN",
					},
				}
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return(resources, nil),
					m.ecsDescriber.EXPECT().Params().Return(params, nil),
					m.ecsDescriber.EXPECT().StackResources().Return(resources, nil),
					m.lbDescriber.EXPECT().ListenerRulesHostHeaders([]string{"listenerRuleARN"}).Return([]string{"jobs.test.phonetool.internal"}, nil),
					m.ecsDescriber.EXPECT().Params().Return(params, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return([]string{"jobs"}, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "X86_64",
					}, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return(nil, nil),
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
					m.ecsDescriber.EXPECT().StackResources().Return(resources, nil),
				)
			},
			wantedBackendSvc: &backendSvcDesc{
				ecsSvcDesc: ecsSvcDesc{
					Service: testSvc,
					Type:    "Backend Service",
					App:     testApp,
					Configurations: []*ECSServiceConfig{
						{
							ServiceConfig: &ServiceConfig{
								CPU:         "256",
								Environment: "test",
								Memory:      "512",
								Platform:    "LINUX/X86_64",
								Port:        "5000",
							},
							Tasks: "1",
						},
					},
					Routes: []*WebServiceRoute{
						{
							Environment: "test",
							URL:         "http://jobs.test.phonetool.internal/mySvc",
						},
					},
					ServiceDiscovery: serviceDiscoveries{
						"jobs.test.phonetool.local:5000": []string{"test"},
					},
					ServiceConnect: serviceConnects{
						"jobs": []string{"test"},
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
					},
					Secrets: []*secret{
						{
							Name:        "GITHUB_WEBHOOK_SECRET",
							Container:   "container",
							Environment: "test",
							ValueFrom:   "GH_WEBHOOK_SECRET",
						},
					},
					Resources: map[string][]*stack.Resource{
						"test": {
							{
								Type:       "AWS::ElasticLoadBalancingV2::TargetGroup",
								LogicalID:  svcStackResourceALBTargetGroupLogicalID,
								PhysicalID: "targetGroupARN",
							},
							{
								Type:       svcStackResourceListenerRuleResourceType,
								LogicalID:  svcStackResourceHTTPListenerRuleLogicalID,
								PhysicalID: "listenerRuleARN",
							},
						},
					},
					environments: []string{"test"},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := backendSvcDescriberMocks{
				storeSvc:     mocks.NewMockDeployedEnvServicesLister(ctrl),
				ecsDescriber: mocks.NewMockecsDescriber(ctrl),
				cwDescriber:  mocks.NewMockcwAlarmDescriber(ctrl),
				envDescriber: mocks.NewMockenvDescriber(ctrl),
				lbDescriber:  mocks.NewMocklbDescriber(ctrl),
			}

			tc.setupMocks(mocks)

			d := &BackendServiceDescriber{
				app:                      testApp,
				svc:                      testSvc,
				enableResources:          tc.shouldOutputResources,
				store:                    mocks.storeSvc,
				initECSServiceDescribers: func(s string) (ecsDescriber, error) { return mocks.ecsDescriber, nil },
				initCWDescriber:          func(s string) (cwAlarmDescriber, error) { return mocks.cwDescriber, nil },
				initEnvDescribers:        func(s string) (envDescriber, error) { return mocks.envDescriber, nil },
				initLBDescriber:          func(s string) (lbDescriber, error) { return mocks.lbDescriber, nil },
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

  Application  my-app
  Name         my-svc
  Type         Backend Service

Configurations

  Environment  Tasks     CPU (vCPU)  Memory (MiB)  Platform      Port
  -----------  -----     ----------  ------------  --------      ----
  test         1         0.25        512           LINUX/X86_64  80
  prod         3         0.5         1024          LINUX/ARM64   5000

Rollback Alarms

  Name        Environment  Description
  ----        -----------  -----------
  alarmName1  test         alarm description 1
  alarmName2  prod         alarm description 2

Internal Service Endpoints

  Endpoint                              Environment  Type
  --------                              -----------  ----
  http://my-svc.prod.my-app.local:5000  prod         Service Discovery
  http://my-svc.test.my-app.local:5000  test         Service Discovery

Variables

  Name                      Container  Environment  Value
  ----                      ---------  -----------  -----
  COPILOT_ENVIRONMENT_NAME  container  prod         prod
    "                         "        test         test

Secrets

  Name                   Container  Environment  Value From
  ----                   ---------  -----------  ----------
  GITHUB_WEBHOOK_SECRET  container  test         parameter/GH_WEBHOOK_SECRET
  SOME_OTHER_SECRET        "        prod         parameter/SHHHHH

Resources

  test
    AWS::EC2::SecurityGroup  sg-0758ed6b233743530

  prod
    AWS::EC2::SecurityGroupIngress  ContainerSecurityGroupIngressFromPublicALB
`,
			wantedJSONString: "{\"service\":\"my-svc\",\"type\":\"Backend Service\",\"application\":\"my-app\",\"configurations\":[{\"environment\":\"test\",\"port\":\"80\",\"cpu\":\"256\",\"memory\":\"512\",\"platform\":\"LINUX/X86_64\",\"tasks\":\"1\"},{\"environment\":\"prod\",\"port\":\"5000\",\"cpu\":\"512\",\"memory\":\"1024\",\"platform\":\"LINUX/ARM64\",\"tasks\":\"3\"}],\"rollbackAlarms\":[{\"name\":\"alarmName1\",\"description\":\"alarm description 1\",\"environment\":\"test\"},{\"name\":\"alarmName2\",\"description\":\"alarm description 2\",\"environment\":\"prod\"}],\"routes\":null,\"serviceDiscovery\":[{\"environment\":[\"prod\"],\"endpoint\":\"http://my-svc.prod.my-app.local:5000\"},{\"environment\":[\"test\"],\"endpoint\":\"http://my-svc.test.my-app.local:5000\"}],\"variables\":[{\"environment\":\"prod\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"prod\",\"container\":\"container\"},{\"environment\":\"test\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"test\",\"container\":\"container\"}],\"secrets\":[{\"name\":\"GITHUB_WEBHOOK_SECRET\",\"container\":\"container\",\"environment\":\"test\",\"valueFrom\":\"GH_WEBHOOK_SECRET\"},{\"name\":\"SOME_OTHER_SECRET\",\"container\":\"container\",\"environment\":\"prod\",\"valueFrom\":\"SHHHHH\"}],\"resources\":{\"prod\":[{\"type\":\"AWS::EC2::SecurityGroupIngress\",\"physicalID\":\"ContainerSecurityGroupIngressFromPublicALB\"}],\"test\":[{\"type\":\"AWS::EC2::SecurityGroup\",\"physicalID\":\"sg-0758ed6b233743530\"}]}}\n",
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
						Port:        "80",
					},
					Tasks: "1",
				},
				{
					ServiceConfig: &ServiceConfig{
						CPU:         "512",
						Environment: "prod",
						Memory:      "1024",
						Platform:    "LINUX/ARM64",
						Port:        "5000",
					},
					Tasks: "3",
				},
			}
			alarmDescs := []*cloudwatch.AlarmDescription{
				{
					Name:        "alarmName1",
					Description: "alarm description 1",
					Environment: "test",
				},
				{
					Name:        "alarmName2",
					Description: "alarm description 2",
					Environment: "prod",
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
			sds := serviceDiscoveries{
				"http://my-svc.test.my-app.local:5000": []string{"test"},
				"http://my-svc.prod.my-app.local:5000": []string{"prod"},
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
				ecsSvcDesc: ecsSvcDesc{
					Service:           "my-svc",
					Type:              "Backend Service",
					Configurations:    config,
					App:               "my-app",
					AlarmDescriptions: alarmDescs,
					Variables:         envVars,
					Secrets:           secrets,
					ServiceDiscovery:  sds,
					Resources:         resources,
					environments:      []string{"test", "prod"},
				},
			}
			human := backendSvc.HumanString()
			json, _ := backendSvc.JSONString()

			require.Equal(t, tc.wantedHumanString, human)
			require.Equal(t, tc.wantedJSONString, json)
		})
	}
}

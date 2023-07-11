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
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type lbWebSvcDescriberMocks struct {
	storeSvc     *mocks.MockDeployedEnvServicesLister
	ecsDescriber *mocks.MockecsDescriber
	envDescriber *mocks.MockenvDescriber
	lbDescriber  *mocks.MocklbDescriber
	cwDescriber  *mocks.MockcwAlarmDescriber
}

func TestLBWebServiceDescriber_Describe(t *testing.T) {
	const (
		testApp           = "phonetool"
		testEnv           = "test"
		testSvc           = "jobs"
		testEnvLBDNSName  = "abc.us-west-1.elb.amazonaws.com"
		testSvcPath       = "*"
		testALBAccessible = "true"
		prodEnv           = "prod"
		prodSvcPath       = "*"
		alarm1            = "alarm1"
		alarm2            = "alarm2"
		desc1             = "alarm description 1"
		desc2             = "alarm description 2"
	)
	mockParams := map[string]string{
		cfnstack.WorkloadTargetPortParamKey: "80",
		cfnstack.WorkloadTaskCountParamKey:  "1",
		cfnstack.WorkloadTaskCPUParamKey:    "256",
		cfnstack.WorkloadTaskMemoryParamKey: "512",
		cfnstack.WorkloadRulePathParamKey:   testSvcPath,
	}
	mockProdParams := map[string]string{
		cfnstack.WorkloadTargetPortParamKey: "5000",
		cfnstack.WorkloadTaskCountParamKey:  "2",
		cfnstack.WorkloadTaskCPUParamKey:    "512",
		cfnstack.WorkloadTaskMemoryParamKey: "1024",
		cfnstack.WorkloadRulePathParamKey:   prodSvcPath,
	}
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		shouldOutputResources bool

		setupMocks func(mocks lbWebSvcDescriberMocks)

		wantedWebSvc *webSvcDesc
		wantedError  error
	}{
		"return error if fail to list environment": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("list deployed environments for application phonetool: some error"),
		},
		"return error if fail to retrieve URI for ALB service": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service URI: get stack parameters for service jobs: some error"),
		},
		"return error if fail to retrieve service params": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
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
					m.ecsDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get stack parameters for service jobs: some error"),
		},
		"return error if fail to retrieve service discovery endpoint": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
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
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return(nil, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("", errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("some error"),
		},
		"return error if fail to retrieve platform": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
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
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
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
		"return error if fail to retrieve rollback alarm names": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
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
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return(nil, errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("retrieve rollback alarm names: some error"),
		},
		"return error if fail to retrieve alarm descriptions": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
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
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return([]string{alarm1, alarm2}, nil),
					m.cwDescriber.EXPECT().AlarmDescriptions([]string{alarm1, alarm2}).Return(nil, errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("retrieve alarm descriptions: some error"),
		},
		"return error if fail to retrieve service connect DNS names": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
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
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return(nil, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service connect DNS names: some error"),
		},
		"return error if fail to retrieve secrets": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
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
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return(nil, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.ecsDescriber.EXPECT().Secrets().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve secrets: some error"),
		},
		"return error if fail to retrieve service resources for ALB service": {
			shouldOutputResources: true,
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "X86_64",
					}, nil),
					m.ecsDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     "test",
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return(nil, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil),
					m.ecsDescriber.EXPECT().Secrets().Return([]*ecs.ContainerSecret{
						{
							Name:      "GITHUB_WEBHOOK_SECRET",
							Container: "container",
							ValueFrom: "GH_WEBHOOK_SECRET",
						},
						{
							Name:      "SOME_OTHER_SECRET",
							Container: "container",
							ValueFrom: "SHHHHHHHH",
						},
					}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service resources: some error"),
		},
		"should not try to fetch descriptions if no ROLLBACK alarms present": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						envOutputPublicALBAccessible:       testALBAccessible,
					}, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "X86_64",
					}, nil),
					m.ecsDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return([]string{}, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return([]string{testSvc}, nil),
					m.ecsDescriber.EXPECT().Secrets().Return([]*ecs.ContainerSecret{}, nil))
			},
			wantedWebSvc: &webSvcDesc{
				ecsSvcDesc: ecsSvcDesc{
					Service: testSvc,
					Type:    "Load Balanced Web Service",
					App:     testApp,
					Configurations: []*ECSServiceConfig{
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
					},
					Routes: []*WebServiceRoute{
						{
							Environment: "test",
							URL:         "http://abc.us-west-1.elb.amazonaws.com/*",
						},
					},
					ServiceDiscovery: serviceDiscoveries{
						"jobs.test.phonetool.local:80": []string{"test"},
					},
					ServiceConnect: serviceConnects{
						testSvc: []string{"test"},
					},
					Resources:    map[string][]*stack.Resource{},
					environments: []string{"test"},
				},
			},
		},
		"success for ALB service": {
			shouldOutputResources: true,
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv, prodEnv}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						envOutputPublicALBAccessible:       testALBAccessible,
					}, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "X86_64",
					}, nil),
					m.ecsDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container1",
							Value:     testEnv,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockParams, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return([]string{alarm1}, nil),
					m.cwDescriber.EXPECT().AlarmDescriptions([]string{alarm1}).Return([]*cloudwatch.AlarmDescription{
						{
							Name:        alarm1,
							Description: desc1,
							Environment: testEnv,
						},
					}, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return([]string{testSvc}, nil),
					m.ecsDescriber.EXPECT().Secrets().Return([]*ecs.ContainerSecret{
						{
							Name:      "GITHUB_WEBHOOK_SECRET",
							Container: "container",
							ValueFrom: "GH_WEBHOOK_SECRET",
						},
					}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockProdParams, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						envOutputPublicALBAccessible:       testALBAccessible,
					}, nil),
					m.ecsDescriber.EXPECT().Platform().Return(&ecs.ContainerPlatform{
						OperatingSystem: "LINUX",
						Architecture:    "ARM64",
					}, nil),
					m.ecsDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container2",
							Value:     prodEnv,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(mockProdParams, nil),
					m.ecsDescriber.EXPECT().RollbackAlarmNames().Return([]string{alarm2}, nil),
					m.cwDescriber.EXPECT().AlarmDescriptions([]string{alarm2}).Return([]*cloudwatch.AlarmDescription{
						{
							Name:        alarm2,
							Description: desc2,
							Environment: prodEnv,
						},
					}, nil),
					m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("prod.phonetool.local", nil),
					m.ecsDescriber.EXPECT().ServiceConnectDNSNames().
						Return([]string{testSvc}, nil),
					m.ecsDescriber.EXPECT().Secrets().Return([]*ecs.ContainerSecret{
						{
							Name:      "SOME_OTHER_SECRET",
							Container: "container",
							ValueFrom: "SHHHHHHHH",
						},
					}, nil),
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
				)
			},
			wantedWebSvc: &webSvcDesc{
				ecsSvcDesc: ecsSvcDesc{
					Service: testSvc,
					Type:    "Load Balanced Web Service",
					App:     testApp,
					Configurations: []*ECSServiceConfig{
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
							Tasks: "2",
						},
					},
					Routes: []*WebServiceRoute{
						{
							Environment: "test",
							URL:         "http://abc.us-west-1.elb.amazonaws.com/*",
						},
						{
							Environment: "prod",
							URL:         "http://abc.us-west-1.elb.amazonaws.com/*",
						},
					},
					AlarmDescriptions: []*cloudwatch.AlarmDescription{
						{
							Name:        alarm1,
							Description: desc1,
							Environment: testEnv,
						},
						{
							Name:        alarm2,
							Description: desc2,
							Environment: prodEnv,
						},
					},
					ServiceDiscovery: serviceDiscoveries{
						"jobs.test.phonetool.local:80":   []string{"test"},
						"jobs.prod.phonetool.local:5000": []string{"prod"},
					},
					ServiceConnect: serviceConnects{
						testSvc: []string{"test", "prod"},
					},
					Variables: []*containerEnvVar{
						{
							envVar: &envVar{
								Environment: "test",
								Name:        "COPILOT_ENVIRONMENT_NAME",
								Value:       "test",
							},
							Container: "container1",
						},
						{
							envVar: &envVar{
								Environment: "prod",
								Name:        "COPILOT_ENVIRONMENT_NAME",
								Value:       "prod",
							},
							Container: "container2",
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
					},
					environments: []string{"test", "prod"},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockDeployedEnvServicesLister(ctrl)
			mockSvcStackDescriber := mocks.NewMockecsDescriber(ctrl)
			mockEnvDescriber := mocks.NewMockenvDescriber(ctrl)
			mockCwDescriber := mocks.NewMockcwAlarmDescriber(ctrl)
			mocks := lbWebSvcDescriberMocks{
				storeSvc:     mockStore,
				ecsDescriber: mockSvcStackDescriber,
				envDescriber: mockEnvDescriber,
				cwDescriber:  mockCwDescriber,
			}

			tc.setupMocks(mocks)

			d := &LBWebServiceDescriber{
				app:                      testApp,
				svc:                      testSvc,
				enableResources:          tc.shouldOutputResources,
				store:                    mockStore,
				initECSServiceDescribers: func(s string) (ecsDescriber, error) { return mockSvcStackDescriber, nil },
				initCWDescriber:          func(s string) (cwAlarmDescriber, error) { return mocks.cwDescriber, nil },
				initEnvDescribers:        func(s string) (envDescriber, error) { return mockEnvDescriber, nil },
			}

			// WHEN
			websvc, err := d.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedWebSvc, websvc, "expected output content match")
			}
		})
	}
}

func TestLBWebServiceDesc_String(t *testing.T) {
	testCases := map[string]struct {
		wantedHumanString string
		wantedJSONString  string
	}{
		"correct output including env vars and secrets sorted (name, container, env) and double-quotes for ditto": {
			wantedHumanString: `About

  Application  my-app
  Name         my-svc
  Type         Load Balanced Web Service

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

Routes

  Environment  URL
  -----------  ---
  test         http://my-pr-Publi.us-west-2.elb.amazonaws.com/frontend
  prod         http://my-pr-Publi.us-west-2.elb.amazonaws.com/backend

Internal Service Endpoints

  Endpoint                              Environment  Type
  --------                              -----------  ----
  my-svc                                test, prod   Service Connect
  http://my-svc.prod.my-app.local:5000  prod         Service Discovery
  http://my-svc.test.my-app.local:5000  test         Service Discovery

Variables

  Name                      Container   Environment  Value
  ----                      ---------   -----------  -----
  COPILOT_ENVIRONMENT_NAME  containerA  test         test
    "                       containerB  prod         prod
  DIFFERENT_ENV_VAR           "           "            "

Secrets

  Name                   Container   Environment  Value From
  ----                   ---------   -----------  ----------
  GITHUB_WEBHOOK_SECRET  containerA  test         parameter/GH_WEBHOOK_SECRET
  SOME_OTHER_SECRET      containerB  prod         parameter/SHHHHH

Resources

  test
    AWS::EC2::SecurityGroup  sg-0758ed6b233743530

  prod
    AWS::EC2::SecurityGroupIngress  ContainerSecurityGroupIngressFromPublicALB
`,
			wantedJSONString: "{\"service\":\"my-svc\",\"type\":\"Load Balanced Web Service\",\"application\":\"my-app\",\"configurations\":[{\"environment\":\"test\",\"port\":\"80\",\"cpu\":\"256\",\"memory\":\"512\",\"platform\":\"LINUX/X86_64\",\"tasks\":\"1\"},{\"environment\":\"prod\",\"port\":\"5000\",\"cpu\":\"512\",\"memory\":\"1024\",\"platform\":\"LINUX/ARM64\",\"tasks\":\"3\"}],\"rollbackAlarms\":[{\"name\":\"alarmName1\",\"description\":\"alarm description 1\",\"environment\":\"test\"},{\"name\":\"alarmName2\",\"description\":\"alarm description 2\",\"environment\":\"prod\"}],\"routes\":[{\"environment\":\"test\",\"url\":\"http://my-pr-Publi.us-west-2.elb.amazonaws.com/frontend\"},{\"environment\":\"prod\",\"url\":\"http://my-pr-Publi.us-west-2.elb.amazonaws.com/backend\"}],\"serviceDiscovery\":[{\"environment\":[\"prod\"],\"endpoint\":\"http://my-svc.prod.my-app.local:5000\"},{\"environment\":[\"test\"],\"endpoint\":\"http://my-svc.test.my-app.local:5000\"}],\"serviceConnect\":[{\"environment\":[\"test\",\"prod\"],\"endpoint\":\"my-svc\"}],\"variables\":[{\"environment\":\"test\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"test\",\"container\":\"containerA\"},{\"environment\":\"prod\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"prod\",\"container\":\"containerB\"},{\"environment\":\"prod\",\"name\":\"DIFFERENT_ENV_VAR\",\"value\":\"prod\",\"container\":\"containerB\"}],\"secrets\":[{\"name\":\"GITHUB_WEBHOOK_SECRET\",\"container\":\"containerA\",\"environment\":\"test\",\"valueFrom\":\"GH_WEBHOOK_SECRET\"},{\"name\":\"SOME_OTHER_SECRET\",\"container\":\"containerB\",\"environment\":\"prod\",\"valueFrom\":\"SHHHHH\"}],\"resources\":{\"prod\":[{\"type\":\"AWS::EC2::SecurityGroupIngress\",\"physicalID\":\"ContainerSecurityGroupIngressFromPublicALB\"}],\"test\":[{\"type\":\"AWS::EC2::SecurityGroup\",\"physicalID\":\"sg-0758ed6b233743530\"}]}}\n",
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
			envVars := []*containerEnvVar{
				{
					envVar: &envVar{
						Environment: "prod",
						Name:        "COPILOT_ENVIRONMENT_NAME",
						Value:       "prod",
					},
					Container: "containerB",
				},
				{
					envVar: &envVar{
						Environment: "test",
						Name:        "COPILOT_ENVIRONMENT_NAME",
						Value:       "test",
					},
					Container: "containerA",
				},
				{
					envVar: &envVar{
						Environment: "prod",
						Name:        "DIFFERENT_ENV_VAR",
						Value:       "prod",
					},
					Container: "containerB",
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
			secrets := []*secret{
				{
					Name:        "GITHUB_WEBHOOK_SECRET",
					Container:   "containerA",
					Environment: "test",
					ValueFrom:   "GH_WEBHOOK_SECRET",
				},
				{
					Name:        "SOME_OTHER_SECRET",
					Container:   "containerB",
					Environment: "prod",
					ValueFrom:   "SHHHHH",
				},
			}
			routes := []*WebServiceRoute{
				{
					Environment: "test",
					URL:         "http://my-pr-Publi.us-west-2.elb.amazonaws.com/frontend",
				},
				{
					Environment: "prod",
					URL:         "http://my-pr-Publi.us-west-2.elb.amazonaws.com/backend",
				},
			}
			sds := serviceDiscoveries{
				"http://my-svc.test.my-app.local:5000": []string{"test"},
				"http://my-svc.prod.my-app.local:5000": []string{"prod"},
			}
			scs := serviceConnects{
				"my-svc": []string{"test", "prod"},
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
			webSvc := &webSvcDesc{
				ecsSvcDesc: ecsSvcDesc{
					Service:           "my-svc",
					Type:              "Load Balanced Web Service",
					Configurations:    config,
					App:               "my-app",
					Variables:         envVars,
					AlarmDescriptions: alarmDescs,
					Secrets:           secrets,
					Routes:            routes,
					ServiceDiscovery:  sds,
					ServiceConnect:    scs,
					Resources:         resources,
					environments:      []string{"test", "prod"},
				},
			}
			human := webSvc.HumanString()
			json, _ := webSvc.JSONString()

			require.Equal(t, tc.wantedHumanString, human)
			require.Equal(t, tc.wantedJSONString, json)
		})
	}
}

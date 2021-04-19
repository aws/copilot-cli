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

func TestLBWebServiceURI_String(t *testing.T) {
	testCases := map[string]struct {
		dnsName string
		path    string

		wanted string
	}{
		"http": {
			dnsName: "abc.us-west-1.elb.amazonaws.com",
			path:    "svc",

			wanted: "http://abc.us-west-1.elb.amazonaws.com/svc",
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
			uri := &LBWebServiceURI{
				DNSName: tc.dnsName,
				Path:    tc.path,
			}

			require.Equal(t, tc.wanted, uri.String())
		})
	}
}

type ecsSvcDescriberMocks struct {
	storeSvc     *mocks.MockDeployedEnvServicesLister
	svcDescriber *mocks.MockecsSvcDescriber
}

func TestLBWebServiceDescriber_URI(t *testing.T) {
	const (
		testApp          = "phonetool"
		testEnv          = "test"
		testSvc          = "jobs"
		testEnvSubdomain = "test.phonetool.com"
		testEnvLBDNSName = "http://abc.us-west-1.elb.amazonaws.com"
		testSvcPath      = "*"
	)
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks ecsSvcDescriberMocks)

		wantedURI   string
		wantedError error
	}{
		"fail to get output of environment stack": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.svcDescriber.EXPECT().EnvOutputs().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get output for environment test: some error"),
		},
		"fail to get parameters of service stack": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.svcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						envOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.svcDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get parameters for service jobs: some error"),
		},
		"https web service": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.svcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						envOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey: testSvcPath,
					}, nil),
				)
			},

			wantedURI: "https://jobs.test.phonetool.com",
		},
		"http web service": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.svcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey: testSvcPath,
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

			mockSvcDescriber := mocks.NewMockecsSvcDescriber(ctrl)
			mocks := ecsSvcDescriberMocks{
				svcDescriber: mockSvcDescriber,
			}

			tc.setupMocks(mocks)

			d := &LBWebServiceDescriber{
				app: testApp,
				svc: testSvc,
				svcDescriber: map[string]ecsSvcDescriber{
					"test": mockSvcDescriber,
				},
				initServiceDescriber: func(string) error { return nil },
			}

			// WHEN
			actual, err := d.URI(testEnv)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedURI, actual)
			}
		})
	}
}

func TestLBWebServiceDescriber_Describe(t *testing.T) {
	const (
		testApp          = "phonetool"
		testEnv          = "test"
		testSvc          = "jobs"
		testEnvLBDNSName = "abc.us-west-1.elb.amazonaws.com"
		testSvcPath      = "*"
		prodEnv          = "prod"
		prodEnvLBDNSName = "abc.us-west-1.elb.amazonaws.com"
		prodSvcPath      = "*"
	)
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		shouldOutputResources bool

		setupMocks func(mocks ecsSvcDescriberMocks)

		wantedWebSvc *webSvcDesc
		wantedError  error
	}{
		"return error if fail to list environment": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("list deployed environments for application phonetool: some error"),
		},
		"return error if fail to retrieve URI": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.svcDescriber.EXPECT().EnvOutputs().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service URI: get output for environment test: some error"),
		},
		"return error if fail to retrieve service deployment configuration": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.svcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "80",
						stack.WorkloadTaskCountParamKey:         "1",
						stack.WorkloadTaskCPUParamKey:           "256",
						stack.WorkloadTaskMemoryParamKey:        "512",
						stack.LBWebServiceRulePathParamKey:      testSvcPath,
					}, nil),
					m.svcDescriber.EXPECT().EnvVars().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve environment variables: some error"),
		},
		"return error if fail to retrieve environment variables": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.svcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "80",
						stack.WorkloadTaskCountParamKey:         "1",
						stack.WorkloadTaskCPUParamKey:           "256",
						stack.WorkloadTaskMemoryParamKey:        "512",
						stack.LBWebServiceRulePathParamKey:      testSvcPath,
					}, nil),
					m.svcDescriber.EXPECT().EnvVars().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve environment variables: some error"),
		},
		"return error if fail to retrieve secrets": {
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.svcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "80",
						stack.WorkloadTaskCountParamKey:         "1",
						stack.WorkloadTaskCPUParamKey:           "256",
						stack.WorkloadTaskMemoryParamKey:        "512",
						stack.LBWebServiceRulePathParamKey:      testSvcPath,
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
		"return error if fail to retrieve service resources": {
			shouldOutputResources: true,
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.svcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey:      testSvcPath,
						stack.LBWebServiceContainerPortParamKey: "80",
						stack.WorkloadTaskCountParamKey:         "1",
						stack.WorkloadTaskCPUParamKey:           "256",
						stack.WorkloadTaskMemoryParamKey:        "512",
					}, nil),
					m.svcDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     "test",
						},
					}, nil),
					m.svcDescriber.EXPECT().Secrets().Return([]*ecs.ContainerSecret{
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
					m.svcDescriber.EXPECT().ServiceStackResources().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service resources: some error"),
		},
		"success": {
			shouldOutputResources: true,
			setupMocks: func(m ecsSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv, prodEnv}, nil),

					m.svcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey:      testSvcPath,
						stack.LBWebServiceContainerPortParamKey: "5000",
						stack.WorkloadTaskCountParamKey:         "1",
						stack.WorkloadTaskCPUParamKey:           "256",
						stack.WorkloadTaskMemoryParamKey:        "512",
					}, nil),
					m.svcDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container1",
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
					m.svcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: prodEnvLBDNSName,
					}, nil),
					m.svcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey:      prodSvcPath,
						stack.LBWebServiceContainerPortParamKey: "5000",
						stack.WorkloadTaskCountParamKey:         "2",
						stack.WorkloadTaskCPUParamKey:           "512",
						stack.WorkloadTaskMemoryParamKey:        "1024",
					}, nil),
					m.svcDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container2",
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
			wantedWebSvc: &webSvcDesc{
				Service: testSvc,
				Type:    "Load Balanced Web Service",
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
				ServiceDiscovery: []*ServiceDiscovery{
					{
						Environment: []string{"test", "prod"},
						Namespace:   "jobs.phonetool.local:5000",
					},
				},
				Variables: []*envVar{
					{
						Environment: "test",
						Container:   "container1",
						Name:        "COPILOT_ENVIRONMENT_NAME",
						Value:       "test",
					},
					{
						Environment: "prod",
						Container:   "container2",
						Name:        "COPILOT_ENVIRONMENT_NAME",
						Value:       "prod",
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
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockDeployedEnvServicesLister(ctrl)
			mockSvcDescriber := mocks.NewMockecsSvcDescriber(ctrl)
			mocks := ecsSvcDescriberMocks{
				storeSvc:     mockStore,
				svcDescriber: mockSvcDescriber,
			}

			tc.setupMocks(mocks)

			d := &LBWebServiceDescriber{
				app:             testApp,
				svc:             testSvc,
				enableResources: tc.shouldOutputResources,
				store:           mockStore,
				svcDescriber: map[string]ecsSvcDescriber{
					"test": mockSvcDescriber,
					"prod": mockSvcDescriber,
				},
				initServiceDescriber: func(string) error { return nil },
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

  Application       my-app
  Name              my-svc
  Type              Load Balanced Web Service

Configurations

  Environment       Tasks               CPU (vCPU)          Memory (MiB)        Port
  -----------       -----               ----------          ------------        ----
  test              1                   0.25                512                 80
  prod              3                   0.5                 1024                5000

Routes

  Environment       URL
  -----------       ---
  test              http://my-pr-Publi.us-west-2.elb.amazonaws.com/frontend
  prod              http://my-pr-Publi.us-west-2.elb.amazonaws.com/backend

Service Discovery

  Environment       Namespace
  -----------       ---------
  test, prod        http://my-svc.my-app.local:5000

Variables

  Name                      Container           Environment         Value
  ----                      ---------           -----------         -----
  COPILOT_ENVIRONMENT_NAME  containerA          test                test
    "                       containerB          prod                prod
  DIFFERENT_ENV_VAR           "                   "                   "

Secrets

  Name                   Container           Environment         Value From
  ----                   ---------           -----------         ----------
  GITHUB_WEBHOOK_SECRET  containerA          test                parameter/GH_WEBHOOK_SECRET
  SOME_OTHER_SECRET      containerB          prod                parameter/SHHHHH

Resources

  test
    AWS::EC2::SecurityGroup  sg-0758ed6b233743530

  prod
    AWS::EC2::SecurityGroupIngress  ContainerSecurityGroupIngressFromPublicALB
`,
			wantedJSONString: "{\"service\":\"my-svc\",\"type\":\"Load Balanced Web Service\",\"application\":\"my-app\",\"configurations\":[{\"environment\":\"test\",\"port\":\"80\",\"tasks\":\"1\",\"cpu\":\"256\",\"memory\":\"512\"},{\"environment\":\"prod\",\"port\":\"5000\",\"tasks\":\"3\",\"cpu\":\"512\",\"memory\":\"1024\"}],\"routes\":[{\"environment\":\"test\",\"url\":\"http://my-pr-Publi.us-west-2.elb.amazonaws.com/frontend\"},{\"environment\":\"prod\",\"url\":\"http://my-pr-Publi.us-west-2.elb.amazonaws.com/backend\"}],\"serviceDiscovery\":[{\"environment\":[\"test\",\"prod\"],\"namespace\":\"http://my-svc.my-app.local:5000\"}],\"variables\":[{\"environment\":\"test\",\"container\":\"containerA\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"test\"},{\"environment\":\"prod\",\"container\":\"containerB\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"prod\"},{\"environment\":\"prod\",\"container\":\"containerB\",\"name\":\"DIFFERENT_ENV_VAR\",\"value\":\"prod\"}],\"secrets\":[{\"name\":\"GITHUB_WEBHOOK_SECRET\",\"container\":\"containerA\",\"environment\":\"test\",\"valueFrom\":\"GH_WEBHOOK_SECRET\"},{\"name\":\"SOME_OTHER_SECRET\",\"container\":\"containerB\",\"environment\":\"prod\",\"valueFrom\":\"SHHHHH\"}],\"resources\":{\"prod\":[{\"type\":\"AWS::EC2::SecurityGroupIngress\",\"physicalID\":\"ContainerSecurityGroupIngressFromPublicALB\"}],\"test\":[{\"type\":\"AWS::EC2::SecurityGroup\",\"physicalID\":\"sg-0758ed6b233743530\"}]}}\n",
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
					Environment: "prod",
					Container:   "containerB",
					Name:        "COPILOT_ENVIRONMENT_NAME",
					Value:       "prod",
				},
				{
					Environment: "test",
					Container:   "containerA",
					Name:        "COPILOT_ENVIRONMENT_NAME",
					Value:       "test",
				},
				{
					Environment: "prod",
					Container:   "containerB",
					Name:        "DIFFERENT_ENV_VAR",
					Value:       "prod",
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
			webSvc := &webSvcDesc{
				Service:          "my-svc",
				Type:             "Load Balanced Web Service",
				Configurations:   config,
				App:              "my-app",
				Variables:        envVars,
				Secrets:          secrets,
				Routes:           routes,
				ServiceDiscovery: sds,
				Resources:        resources,
			}
			human := webSvc.HumanString()
			json, _ := webSvc.JSONString()

			require.Equal(t, tc.wantedHumanString, human)
			require.Equal(t, tc.wantedJSONString, json)
		})
	}
}

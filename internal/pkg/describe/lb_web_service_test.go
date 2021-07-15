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

func TestLBWebServiceURI_String(t *testing.T) {
	testCases := map[string]struct {
		dnsNames []string
		path     string
		https    bool

		wanted string
	}{
		"http": {
			dnsNames: []string{"abc.us-west-1.elb.amazonaws.com"},
			path:     "svc",

			wanted: "http://abc.us-west-1.elb.amazonaws.com/svc",
		},
		"http with / path": {
			dnsNames: []string{"jobs.test.phonetool.com"},
			path:     "/",

			wanted: "http://jobs.test.phonetool.com",
		},
		"https": {
			dnsNames: []string{"jobs.test.phonetool.com"},
			path:     "svc",
			https:    true,

			wanted: "https://jobs.test.phonetool.com/svc",
		},
		"https with / path": {
			dnsNames: []string{"jobs.test.phonetool.com"},
			path:     "/",
			https:    true,

			wanted: "https://jobs.test.phonetool.com",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			uri := &LBWebServiceURI{
				DNSNames: tc.dnsNames,
				Path:     tc.path,
				HTTPS:    tc.https,
			}

			require.Equal(t, tc.wanted, uri.String())
		})
	}
}

type lbWebSvcDescriberMocks struct {
	storeSvc        *mocks.MockDeployedEnvServicesLister
	ecsSvcDescriber *mocks.MockecsSvcDescriber
	envDescriber    *mocks.MockenvDescriber
}

func TestLBWebServiceDescriber_URI(t *testing.T) {
	const (
		testApp          = "phonetool"
		testEnv          = "test"
		testSvc          = "jobs"
		testEnvSubdomain = "test.phonetool.com"
		testEnvLBDNSName = "abc.us-west-1.elb.amazonaws.com"
		testSvcPath      = "/"
	)
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks lbWebSvcDescriberMocks)

		wantedURI   string
		wantedError error
	}{
		"fail to get parameters of environment stack": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.envDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get stack parameters for environment test: some error"),
		},
		"fail to get outputs of environment stack": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.envDescriber.EXPECT().Params().Return(map[string]string{}, nil),
					m.envDescriber.EXPECT().Outputs().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get stack outputs for environment test: some error"),
		},
		"fail to get parameters of service stack": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.envDescriber.EXPECT().Params().Return(map[string]string{}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						envOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.ecsSvcDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get stack parameters for service jobs: some error"),
		},
		"https web service": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.envDescriber.EXPECT().Params().Return(map[string]string{}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						envOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.ecsSvcDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceRulePathParamKey: testSvcPath,
					}, nil),
				)
			},

			wantedURI: "https://jobs.test.phonetool.com",
		},
		"http web service": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.envDescriber.EXPECT().Params().Return(map[string]string{}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.ecsSvcDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceRulePathParamKey: "*",
					}, nil),
				)
			},

			wantedURI: "http://abc.us-west-1.elb.amazonaws.com/*",
		},
		"with alias": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.envDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.EnvParamAliasesKey: `{"jobs": ["example.com", "v1.example.com"]}`,
					}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						envOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.ecsSvcDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceRulePathParamKey: testSvcPath,
					}, nil),
				)
			},

			wantedURI: "https://example.com or https://v1.example.com",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSvcDescriber := mocks.NewMockecsSvcDescriber(ctrl)
			mockEnvDescriber := mocks.NewMockenvDescriber(ctrl)
			mocks := lbWebSvcDescriberMocks{
				ecsSvcDescriber: mockSvcDescriber,
				envDescriber:    mockEnvDescriber,
			}

			tc.setupMocks(mocks)

			d := &LBWebServiceDescriber{
				app: testApp,
				svc: testSvc,
				svcDescriber: map[string]ecsSvcDescriber{
					"test": mockSvcDescriber,
				},
				envDescriber: map[string]envDescriber{
					"test": mockEnvDescriber,
				},
				initDescriber: func(string) error { return nil },
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
		"return error if fail to retrieve URI": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.envDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service URI: get stack parameters for environment test: some error"),
		},
		"return error if fail to retrieve service deployment configuration": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.envDescriber.EXPECT().Params().Return(map[string]string{}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.ecsSvcDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceContainerPortParamKey: "80",
						cfnstack.WorkloadTaskCountParamKey:         "1",
						cfnstack.WorkloadTaskCPUParamKey:           "256",
						cfnstack.WorkloadTaskMemoryParamKey:        "512",
						cfnstack.LBWebServiceRulePathParamKey:      testSvcPath,
					}, nil),
					m.ecsSvcDescriber.EXPECT().EnvVars().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve environment variables: some error"),
		},
		"return error if fail to retrieve environment variables": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.envDescriber.EXPECT().Params().Return(map[string]string{}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.ecsSvcDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceContainerPortParamKey: "80",
						cfnstack.WorkloadTaskCountParamKey:         "1",
						cfnstack.WorkloadTaskCPUParamKey:           "256",
						cfnstack.WorkloadTaskMemoryParamKey:        "512",
						cfnstack.LBWebServiceRulePathParamKey:      testSvcPath,
					}, nil),
					m.ecsSvcDescriber.EXPECT().EnvVars().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve environment variables: some error"),
		},
		"return error if fail to retrieve secrets": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.envDescriber.EXPECT().Params().Return(map[string]string{}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.ecsSvcDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceContainerPortParamKey: "80",
						cfnstack.WorkloadTaskCountParamKey:         "1",
						cfnstack.WorkloadTaskCPUParamKey:           "256",
						cfnstack.WorkloadTaskMemoryParamKey:        "512",
						cfnstack.LBWebServiceRulePathParamKey:      testSvcPath,
					}, nil),
					m.ecsSvcDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     "prod",
						},
					}, nil),

					m.ecsSvcDescriber.EXPECT().Secrets().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve secrets: some error"),
		},
		"return error if fail to retrieve service resources": {
			shouldOutputResources: true,
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.envDescriber.EXPECT().Params().Return(map[string]string{}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.ecsSvcDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceContainerPortParamKey: "80",
						cfnstack.WorkloadTaskCountParamKey:         "1",
						cfnstack.WorkloadTaskCPUParamKey:           "256",
						cfnstack.WorkloadTaskMemoryParamKey:        "512",
						cfnstack.LBWebServiceRulePathParamKey:      testSvcPath,
					}, nil),
					m.ecsSvcDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container",
							Value:     "test",
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().Secrets().Return([]*ecs.ContainerSecret{
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
					m.ecsSvcDescriber.EXPECT().ServiceStackResources().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service resources: some error"),
		},
		"success": {
			shouldOutputResources: true,
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv, prodEnv}, nil),

					m.envDescriber.EXPECT().Params().Return(map[string]string{}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.ecsSvcDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceContainerPortParamKey: "5000",
						cfnstack.WorkloadTaskCountParamKey:         "1",
						cfnstack.WorkloadTaskCPUParamKey:           "256",
						cfnstack.WorkloadTaskMemoryParamKey:        "512",
						cfnstack.LBWebServiceRulePathParamKey:      testSvcPath,
					}, nil),
					m.ecsSvcDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container1",
							Value:     testEnv,
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().Secrets().Return([]*ecs.ContainerSecret{
						{
							Name:      "GITHUB_WEBHOOK_SECRET",
							Container: "container",
							ValueFrom: "GH_WEBHOOK_SECRET",
						},
					}, nil),
					m.envDescriber.EXPECT().Params().Return(map[string]string{}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.ecsSvcDescriber.EXPECT().Params().Return(map[string]string{
						cfnstack.LBWebServiceContainerPortParamKey: "5000",
						cfnstack.WorkloadTaskCountParamKey:         "2",
						cfnstack.WorkloadTaskCPUParamKey:           "512",
						cfnstack.WorkloadTaskMemoryParamKey:        "1024",
						cfnstack.LBWebServiceRulePathParamKey:      prodSvcPath,
					}, nil),
					m.ecsSvcDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:      "COPILOT_ENVIRONMENT_NAME",
							Container: "container2",
							Value:     prodEnv,
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().Secrets().Return([]*ecs.ContainerSecret{
						{
							Name:      "SOME_OTHER_SECRET",
							Container: "container",
							ValueFrom: "SHHHHHHHH",
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().ServiceStackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::EC2::SecurityGroupIngress",
							PhysicalID: "ContainerSecurityGroupIngressFromPublicALB",
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().ServiceStackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::EC2::SecurityGroup",
							PhysicalID: "sg-0758ed6b233743530",
						},
					}, nil),
				)
			},
			wantedWebSvc: &webSvcDesc{
				Service: testSvc,
				Type:    "Load Balanced Web Service",
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
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockDeployedEnvServicesLister(ctrl)
			mockSvcDescriber := mocks.NewMockecsSvcDescriber(ctrl)
			mockEnvDescriber := mocks.NewMockenvDescriber(ctrl)
			mocks := lbWebSvcDescriberMocks{
				storeSvc:        mockStore,
				ecsSvcDescriber: mockSvcDescriber,
				envDescriber:    mockEnvDescriber,
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
				envDescriber: map[string]envDescriber{
					"test": mockEnvDescriber,
					"prod": mockEnvDescriber,
				},
				initDescriber: func(string) error { return nil },
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
  test              http://my-svc.test.my-app.local:5000
  prod              http://my-svc.prod.my-app.local:5000

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
			wantedJSONString: "{\"service\":\"my-svc\",\"type\":\"Load Balanced Web Service\",\"application\":\"my-app\",\"configurations\":[{\"environment\":\"test\",\"port\":\"80\",\"cpu\":\"256\",\"memory\":\"512\",\"tasks\":\"1\"},{\"environment\":\"prod\",\"port\":\"5000\",\"cpu\":\"512\",\"memory\":\"1024\",\"tasks\":\"3\"}],\"routes\":[{\"environment\":\"test\",\"url\":\"http://my-pr-Publi.us-west-2.elb.amazonaws.com/frontend\"},{\"environment\":\"prod\",\"url\":\"http://my-pr-Publi.us-west-2.elb.amazonaws.com/backend\"}],\"serviceDiscovery\":[{\"environment\":[\"test\"],\"namespace\":\"http://my-svc.test.my-app.local:5000\"},{\"environment\":[\"prod\"],\"namespace\":\"http://my-svc.prod.my-app.local:5000\"}],\"variables\":[{\"environment\":\"test\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"test\",\"container\":\"containerA\"},{\"environment\":\"prod\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"prod\",\"container\":\"containerB\"},{\"environment\":\"prod\",\"name\":\"DIFFERENT_ENV_VAR\",\"value\":\"prod\",\"container\":\"containerB\"}],\"secrets\":[{\"name\":\"GITHUB_WEBHOOK_SECRET\",\"container\":\"containerA\",\"environment\":\"test\",\"valueFrom\":\"GH_WEBHOOK_SECRET\"},{\"name\":\"SOME_OTHER_SECRET\",\"container\":\"containerB\",\"environment\":\"prod\",\"valueFrom\":\"SHHHHH\"}],\"resources\":{\"prod\":[{\"type\":\"AWS::EC2::SecurityGroupIngress\",\"physicalID\":\"ContainerSecurityGroupIngressFromPublicALB\"}],\"test\":[{\"type\":\"AWS::EC2::SecurityGroup\",\"physicalID\":\"sg-0758ed6b233743530\"}]}}\n",
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
				environments:     []string{"test", "prod"},
			}
			human := webSvc.HumanString()
			json, _ := webSvc.JSONString()

			require.Equal(t, tc.wantedHumanString, human)
			require.Equal(t, tc.wantedJSONString, json)
		})
	}
}

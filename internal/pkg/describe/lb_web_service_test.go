// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestWebServiceURI_String(t *testing.T) {
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
			uri := &WebServiceURI{
				DNSName: tc.dnsName,
				Path:    tc.path,
			}

			require.Equal(t, tc.wanted, uri.String())
		})
	}
}

type webSvcDescriberMocks struct {
	storeSvc         *mocks.MockDeployedEnvServicesLister
	testSvcDescriber *mocks.MocksvcDescriber
	prodSvcDescriber *mocks.MocksvcDescriber
}

func TestWebServiceDescriber_URI(t *testing.T) {
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
		setupMocks func(mocks webSvcDescriberMocks)

		wantedURI   string
		wantedError error
	}{
		"fail to get output of environment stack": {
			setupMocks: func(m webSvcDescriberMocks) {
				gomock.InOrder(
					m.testSvcDescriber.EXPECT().EnvOutputs().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get output for environment test: some error"),
		},
		"fail to get parameters of service stack": {
			setupMocks: func(m webSvcDescriberMocks) {
				gomock.InOrder(
					m.testSvcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						stack.EnvOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.testSvcDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get parameters for service jobs: some error"),
		},
		"https web service": {
			setupMocks: func(m webSvcDescriberMocks) {
				gomock.InOrder(
					m.testSvcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						stack.EnvOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.testSvcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey: testSvcPath,
					}, nil),
				)
			},

			wantedURI: "https://jobs.test.phonetool.com",
		},
		"http web service": {
			setupMocks: func(m webSvcDescriberMocks) {
				gomock.InOrder(
					m.testSvcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.testSvcDescriber.EXPECT().Params().Return(map[string]string{
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

			mockSvcDescriber := mocks.NewMocksvcDescriber(ctrl)
			mocks := webSvcDescriberMocks{
				testSvcDescriber: mockSvcDescriber,
			}

			tc.setupMocks(mocks)

			d := &WebServiceDescriber{
				app: testApp,
				svc: testSvc,
				svcDescriber: map[string]svcDescriber{
					"test": mockSvcDescriber,
				},
				initServiceDescriber: func(string) error { return nil },
				svcParams:            make(map[string]map[string]string),
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

func TestWebServiceDescriber_Describe(t *testing.T) {
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

		setupMocks func(mocks webSvcDescriberMocks)

		wantedWebSvc *webSvcDesc
		wantedError  error
	}{
		"return error if fail to list environment": {
			setupMocks: func(m webSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("list deployed environments for application phonetool: some error"),
		},
		"return error if fail to retrieve URI": {
			setupMocks: func(m webSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.testSvcDescriber.EXPECT().EnvOutputs().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service URI: get output for environment test: some error"),
		},
		"return error if fail to retrieve service deployment configuration": {
			setupMocks: func(m webSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.testSvcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.testSvcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceContainerPortParamKey: "80",
						stack.ServiceTaskCountParamKey:          "1",
						stack.ServiceTaskCPUParamKey:            "256",
						stack.ServiceTaskMemoryParamKey:         "512",
						stack.LBWebServiceRulePathParamKey:      testSvcPath,
					}, nil),
					m.testSvcDescriber.EXPECT().EnvVars().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve environment variables: some error"),
		},
		"return error if fail to retrieve service resources": {
			shouldOutputResources: true,
			setupMocks: func(m webSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.testSvcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
						stack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.testSvcDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey:      testSvcPath,
						stack.LBWebServiceContainerPortParamKey: "80",
						stack.ServiceTaskCountParamKey:          "1",
						stack.ServiceTaskCPUParamKey:            "256",
						stack.ServiceTaskMemoryParamKey:         "512",
					}, nil),
					m.testSvcDescriber.EXPECT().EnvVars().Return(
						map[string]string{
							"COPILOT_ENVIRONMENT_NAME": testEnv,
						}, nil),
					m.testSvcDescriber.EXPECT().ServiceStackResources().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service resources: some error"),
		},
		"success": {
			shouldOutputResources: true,
			setupMocks: func(m webSvcDescriberMocks) {
				m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv, prodEnv}, nil)

				m.testSvcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
					stack.EnvOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
				}, nil)
				m.testSvcDescriber.EXPECT().Params().Return(map[string]string{
					stack.LBWebServiceRulePathParamKey:      testSvcPath,
					stack.LBWebServiceContainerPortParamKey: "5000",
					stack.ServiceTaskCountParamKey:          "1",
					stack.ServiceTaskCPUParamKey:            "256",
					stack.ServiceTaskMemoryParamKey:         "512",
				}, nil)
				m.testSvcDescriber.EXPECT().EnvVars().Return(
					map[string]string{
						"COPILOT_ENVIRONMENT_NAME": testEnv,
					}, nil)

				m.prodSvcDescriber.EXPECT().EnvOutputs().Return(map[string]string{
					stack.EnvOutputPublicLoadBalancerDNSName: prodEnvLBDNSName,
				}, nil)
				m.prodSvcDescriber.EXPECT().Params().Return(map[string]string{
					stack.LBWebServiceRulePathParamKey:      prodSvcPath,
					stack.LBWebServiceContainerPortParamKey: "5000",
					stack.ServiceTaskCountParamKey:          "2",
					stack.ServiceTaskCPUParamKey:            "512",
					stack.ServiceTaskMemoryParamKey:         "1024",
				}, nil)
				m.prodSvcDescriber.EXPECT().EnvVars().Return(
					map[string]string{
						"COPILOT_ENVIRONMENT_NAME": prodEnv,
					}, nil)

				m.testSvcDescriber.EXPECT().ServiceStackResources().Return([]*cloudformation.StackResource{
					{
						ResourceType:       aws.String("AWS::EC2::SecurityGroupIngress"),
						PhysicalResourceId: aws.String("ContainerSecurityGroupIngressFromPublicALB"),
					},
				}, nil)
				m.prodSvcDescriber.EXPECT().ServiceStackResources().Return([]*cloudformation.StackResource{
					{
						ResourceType:       aws.String("AWS::EC2::SecurityGroup"),
						PhysicalResourceId: aws.String("sg-0758ed6b233743530"),
					},
				}, nil)
			},
			wantedWebSvc: &webSvcDesc{
				Service: testSvc,
				Type:    "Load Balanced Web Service",
				App:     testApp,
				Configurations: []*ServiceConfig{
					{
						CPU:         "512",
						Environment: "prod",
						Memory:      "1024",
						Port:        "5000",
						Tasks:       "2",
					},
					{
						CPU:         "256",
						Environment: "test",
						Memory:      "512",
						Port:        "5000",
						Tasks:       "1",
					},
				},
				Routes: []*WebServiceRoute{
					{
						Environment: "prod",
						URL:         "http://abc.us-west-1.elb.amazonaws.com/*",
					},
					{
						Environment: "test",
						URL:         "http://abc.us-west-1.elb.amazonaws.com/*",
					},
				},
				ServiceDiscovery: []*ServiceDiscovery{
					{
						Environment: []string{"prod", "test"},
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
			mockTestSvcDescriber := mocks.NewMocksvcDescriber(ctrl)
			mockProdSvcDescriber := mocks.NewMocksvcDescriber(ctrl)
			mocks := webSvcDescriberMocks{
				storeSvc:         mockStore,
				testSvcDescriber: mockTestSvcDescriber,
				prodSvcDescriber: mockProdSvcDescriber,
			}

			tc.setupMocks(mocks)

			d := &WebServiceDescriber{
				app:             testApp,
				svc:             testSvc,
				enableResources: tc.shouldOutputResources,
				store:           mockStore,
				svcDescriber: map[string]svcDescriber{
					"test": mockTestSvcDescriber,
					"prod": mockProdSvcDescriber,
				},
				initServiceDescriber: func(string) error { return nil },
				svcParams:            make(map[string]map[string]string),
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

func TestWebServiceDesc_String(t *testing.T) {
	testCases := map[string]struct {
		wantedHumanString string
		wantedJSONString  string
	}{
		"correct output": {
			wantedHumanString: `About

  Application       my-app
  Name              my-svc
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
			wantedJSONString: "{\"service\":\"my-svc\",\"type\":\"Load Balanced Web Service\",\"application\":\"my-app\",\"configurations\":[{\"environment\":\"test\",\"port\":\"80\",\"tasks\":\"1\",\"cpu\":\"256\",\"memory\":\"512\"},{\"environment\":\"prod\",\"port\":\"5000\",\"tasks\":\"3\",\"cpu\":\"512\",\"memory\":\"1024\"}],\"routes\":[{\"environment\":\"test\",\"url\":\"http://my-pr-Publi.us-west-2.elb.amazonaws.com/frontend\"},{\"environment\":\"prod\",\"url\":\"http://my-pr-Publi.us-west-2.elb.amazonaws.com/backend\"}],\"serviceDiscovery\":[{\"environment\":[\"test\",\"prod\"],\"namespace\":\"http://my-svc.my-app.local:5000\"}],\"variables\":[{\"environment\":\"prod\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"prod\"},{\"environment\":\"test\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"test\"}],\"resources\":{\"prod\":[{\"type\":\"AWS::EC2::SecurityGroupIngress\",\"physicalID\":\"ContainerSecurityGroupIngressFromPublicALB\"}],\"test\":[{\"type\":\"AWS::EC2::SecurityGroup\",\"physicalID\":\"sg-0758ed6b233743530\"}]}}\n",
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

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

const humanStringWithResources = `About

  Application  testapp
  Name         testsvc
  Type         Request-Driven Web Service

Configurations

  Environment  CPU (vCPU)  Memory (MiB)  Port
  -----------  ----------  ------------  ----
  test         1           2048          80
  prod         2           3072            "

Routes

  Environment  Ingress      URL
  -----------  -------      ---
  test         environment  https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com
  prod         internet     https://tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com

Variables

  Name                      Environment  Value
  ----                      -----------  -----
  COPILOT_ENVIRONMENT_NAME  prod         prod
    "                       test         test

Secrets

  Name           Environment  Value
  ----           -----------  -----
  my-ssm-secret  prod         prod
    "            test         test

Resources

  test
    AWS::AppRunner::Service  arn:aws:apprunner:us-east-1:111111111111:service/testapp-test-testsvc

  prod
    AWS::AppRunner::Service  arn:aws:apprunner:us-east-1:111111111111:service/testapp-prod-testsvc
`

type apprunnerSvcDescriberMocks struct {
	storeSvc        *mocks.MockDeployedEnvServicesLister
	ecsSvcDescriber *mocks.MockapprunnerDescriber
}

func TestRDWebServiceDescriber_Describe(t *testing.T) {
	const (
		testApp = "testapp"
		testSvc = "testsvc"
		testEnv = "test"
		prodEnv = "prod"
	)
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		shouldOutputResources bool

		setupMocks func(mocks apprunnerSvcDescriberMocks)

		wantedSvcDesc *rdWebSvcDesc
		wantedError   error
	}{
		"return error if fail to list environment": {
			setupMocks: func(m apprunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("list deployed environments for application testapp: some error"),
		},
		"return error if fail to retrieve service configuration": {
			setupMocks: func(m apprunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsSvcDescriber.EXPECT().Service().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service configuration: some error"),
		},
		"return error if fail to get service url": {
			shouldOutputResources: true,
			setupMocks: func(m apprunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsSvcDescriber.EXPECT().Service().Return(&apprunner.Service{}, nil),
					m.ecsSvcDescriber.EXPECT().ServiceURL().Return("", mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service url: some error"),
		},
		"return error if fail to check if private": {
			shouldOutputResources: true,
			setupMocks: func(m apprunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsSvcDescriber.EXPECT().Service().Return(&apprunner.Service{}, nil),
					m.ecsSvcDescriber.EXPECT().ServiceURL().Return("", nil),
					m.ecsSvcDescriber.EXPECT().IsPrivate().Return(false, mockErr),
				)
			},
			wantedError: fmt.Errorf("check if service is private: some error"),
		},
		"return error if fail to retrieve service resources": {
			shouldOutputResources: true,
			setupMocks: func(m apprunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsSvcDescriber.EXPECT().Service().Return(&apprunner.Service{}, nil),
					m.ecsSvcDescriber.EXPECT().ServiceURL().Return("", nil),
					m.ecsSvcDescriber.EXPECT().IsPrivate().Return(false, nil),
					m.ecsSvcDescriber.EXPECT().StackResources().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("retrieve service resources: some error"),
		},
		"success": {
			shouldOutputResources: true,
			setupMocks: func(m apprunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv, prodEnv}, nil),
					m.ecsSvcDescriber.EXPECT().Service().Return(&apprunner.Service{
						ServiceARN: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-test-testsvc",
						ServiceURL: "6znxd4ra33.public.us-east-1.apprunner.amazonaws.com",
						CPU:        "1024",
						Memory:     "2048",
						Port:       "80",
						EnvironmentVariables: []*apprunner.EnvironmentVariable{
							{
								Name:  "COPILOT_ENVIRONMENT_NAME",
								Value: "test",
							},
						},
						EnvironmentSecrets: []*apprunner.EnvironmentSecret{
							{
								Name:  "SOME_OTHER_SECRET",
								Value: "arn:aws:ssm:us-east-1:111111111111:parameter/SHHHHH",
							},
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().ServiceURL().Return("https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com", nil),
					m.ecsSvcDescriber.EXPECT().IsPrivate().Return(true, nil),
					m.ecsSvcDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::AppRunner::Service",
							PhysicalID: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-test-testsvc",
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().Service().Return(&apprunner.Service{
						ServiceARN: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-prod-testsvc",
						ServiceURL: "tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com",
						CPU:        "2048",
						Memory:     "3072",
						Port:       "80",
						EnvironmentVariables: []*apprunner.EnvironmentVariable{
							{
								Name:  "COPILOT_ENVIRONMENT_NAME",
								Value: "prod",
							},
						},
						EnvironmentSecrets: []*apprunner.EnvironmentSecret{
							{
								Name:  "my-ssm-secret",
								Value: "arn:aws:ssm:us-east-1:111111111111:parameter/jan11ssm",
							},
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().ServiceURL().Return("https://tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com", nil),
					m.ecsSvcDescriber.EXPECT().IsPrivate().Return(false, nil),
					m.ecsSvcDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::AppRunner::Service",
							PhysicalID: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-prod-testsvc",
						},
					}, nil),
				)
			},
			wantedSvcDesc: &rdWebSvcDesc{
				Service: testSvc,
				Type:    "Request-Driven Web Service",
				App:     testApp,
				AppRunnerConfigurations: []*ServiceConfig{
					{
						CPU:         "1024",
						Environment: "test",
						Memory:      "2048",
						Port:        "80",
					},
					{
						CPU:         "2048",
						Environment: "prod",
						Memory:      "3072",
						Port:        "80",
					},
				},
				Routes: []*RDWSRoute{
					{
						Environment: "test",
						URL:         "https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com",
						Ingress:     rdwsIngressEnvironment,
					},
					{
						Environment: "prod",
						URL:         "https://tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com",
						Ingress:     rdwsIngressInternet,
					},
				},
				Variables: []*envVar{
					{
						Environment: "test",
						Name:        "COPILOT_ENVIRONMENT_NAME",
						Value:       "test",
					},
					{
						Environment: "prod",
						Name:        "COPILOT_ENVIRONMENT_NAME",
						Value:       "prod",
					},
				},
				Secrets: []*rdwsSecret{
					{
						Environment: "test",
						Name:        "SOME_OTHER_SECRET",
						ValueFrom:   "arn:aws:ssm:us-east-1:111111111111:parameter/SHHHHH",
					},
					{
						Environment: "prod",
						Name:        "my-ssm-secret",
						ValueFrom:   "arn:aws:ssm:us-east-1:111111111111:parameter/jan11ssm",
					},
				},
				Resources: map[string][]*stack.Resource{
					"test": {
						{
							Type:       "AWS::AppRunner::Service",
							PhysicalID: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-test-testsvc",
						},
					},
					"prod": {
						{
							Type:       "AWS::AppRunner::Service",
							PhysicalID: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-prod-testsvc",
						},
					},
				},
				environments: []string{"test", "prod"},
			},
		},
		"success with observability": {
			shouldOutputResources: true,
			setupMocks: func(m apprunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv, prodEnv}, nil),
					m.ecsSvcDescriber.EXPECT().Service().Return(&apprunner.Service{
						ServiceARN: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-test-testsvc",
						ServiceURL: "6znxd4ra33.public.us-east-1.apprunner.amazonaws.com",
						CPU:        "1024",
						Memory:     "2048",
						Port:       "80",
						EnvironmentVariables: []*apprunner.EnvironmentVariable{
							{
								Name:  "COPILOT_ENVIRONMENT_NAME",
								Value: "test",
							},
						},
						EnvironmentSecrets: []*apprunner.EnvironmentSecret{
							{
								Name:  "SOME_OTHER_SECRET",
								Value: "arn:aws:ssm:us-east-1:111111111111:parameter/SHHHHH",
							},
						},
						Observability: apprunner.ObservabilityConfiguration{
							TraceConfiguration: &apprunner.TraceConfiguration{
								Vendor: aws.String("mockVendor"),
							},
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().ServiceURL().Return("https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com", nil),
					m.ecsSvcDescriber.EXPECT().IsPrivate().Return(true, nil),
					m.ecsSvcDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::AppRunner::Service",
							PhysicalID: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-test-testsvc",
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().Service().Return(&apprunner.Service{
						ServiceARN: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-prod-testsvc",
						ServiceURL: "tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com",
						CPU:        "2048",
						Memory:     "3072",
						Port:       "80",
						EnvironmentVariables: []*apprunner.EnvironmentVariable{
							{
								Name:  "COPILOT_ENVIRONMENT_NAME",
								Value: "prod",
							},
						},
						EnvironmentSecrets: []*apprunner.EnvironmentSecret{
							{
								Name:  "my-ssm-secret",
								Value: "arn:aws:ssm:us-east-1:111111111111:parameter/jan11ssm",
							},
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().ServiceURL().Return("https://tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com", nil),
					m.ecsSvcDescriber.EXPECT().IsPrivate().Return(false, nil),
					m.ecsSvcDescriber.EXPECT().StackResources().Return([]*stack.Resource{
						{
							Type:       "AWS::AppRunner::Service",
							PhysicalID: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-prod-testsvc",
						},
					}, nil),
				)
			},
			wantedSvcDesc: &rdWebSvcDesc{
				Service: testSvc,
				Type:    "Request-Driven Web Service",
				App:     testApp,
				AppRunnerConfigurations: []*ServiceConfig{
					{
						CPU:         "1024",
						Environment: "test",
						Memory:      "2048",
						Port:        "80",
					},
					{
						CPU:         "2048",
						Environment: "prod",
						Memory:      "3072",
						Port:        "80",
					},
				},
				Routes: []*RDWSRoute{
					{
						Environment: "test",
						URL:         "https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com",
						Ingress:     rdwsIngressEnvironment,
					},
					{
						Environment: "prod",
						URL:         "https://tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com",
						Ingress:     rdwsIngressInternet,
					},
				},
				Variables: []*envVar{
					{
						Environment: "test",
						Name:        "COPILOT_ENVIRONMENT_NAME",
						Value:       "test",
					},
					{
						Environment: "prod",
						Name:        "COPILOT_ENVIRONMENT_NAME",
						Value:       "prod",
					},
				},
				Secrets: []*rdwsSecret{
					{
						Environment: "test",
						Name:        "SOME_OTHER_SECRET",
						ValueFrom:   "arn:aws:ssm:us-east-1:111111111111:parameter/SHHHHH",
					},
					{
						Environment: "prod",
						Name:        "my-ssm-secret",
						ValueFrom:   "arn:aws:ssm:us-east-1:111111111111:parameter/jan11ssm",
					},
				},
				Observability: []observabilityInEnv{
					{
						Environment: "test",
						Tracing: &tracing{
							Vendor: "mockVendor",
						},
					},
					{
						Environment: "prod",
					},
				},
				Resources: map[string][]*stack.Resource{
					"test": {
						{
							Type:       "AWS::AppRunner::Service",
							PhysicalID: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-test-testsvc",
						},
					},
					"prod": {
						{
							Type:       "AWS::AppRunner::Service",
							PhysicalID: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-prod-testsvc",
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
			mockSvcDescriber := mocks.NewMockapprunnerDescriber(ctrl)
			mocks := apprunnerSvcDescriberMocks{
				storeSvc:        mockStore,
				ecsSvcDescriber: mockSvcDescriber,
			}

			tc.setupMocks(mocks)

			d := &RDWebServiceDescriber{
				app:                    testApp,
				svc:                    testSvc,
				enableResources:        tc.shouldOutputResources,
				store:                  mockStore,
				initAppRunnerDescriber: func(string) (apprunnerDescriber, error) { return mockSvcDescriber, nil },
			}

			// WHEN
			svcDesc, err := d.Describe()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedSvcDesc, svcDesc, "expected output content match")
			}
		})
	}
}

func TestRDWebServiceDesc_String(t *testing.T) {
	t.Run("correct output including resources", func(t *testing.T) {
		wantedHumanString := humanStringWithResources
		wantedJSONString := "{\"service\":\"testsvc\",\"type\":\"Request-Driven Web Service\",\"application\":\"testapp\",\"configurations\":[{\"environment\":\"test\",\"port\":\"80\",\"cpu\":\"1024\",\"memory\":\"2048\"},{\"environment\":\"prod\",\"port\":\"80\",\"cpu\":\"2048\",\"memory\":\"3072\"}],\"routes\":[{\"environment\":\"test\",\"url\":\"https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com\",\"ingress\":\"environment\"},{\"environment\":\"prod\",\"url\":\"https://tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com\",\"ingress\":\"internet\"}],\"variables\":[{\"environment\":\"prod\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"prod\"},{\"environment\":\"test\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"test\"}],\"resources\":{\"prod\":[{\"type\":\"AWS::AppRunner::Service\",\"physicalID\":\"arn:aws:apprunner:us-east-1:111111111111:service/testapp-prod-testsvc\"}],\"test\":[{\"type\":\"AWS::AppRunner::Service\",\"physicalID\":\"arn:aws:apprunner:us-east-1:111111111111:service/testapp-test-testsvc\"}]},\"secrets\":[{\"environment\":\"prod\",\"name\":\"my-ssm-secret\",\"value\":\"prod\"},{\"environment\":\"test\",\"name\":\"my-ssm-secret\",\"value\":\"test\"}]}\n"
		svcDesc := &rdWebSvcDesc{
			Service: "testsvc",
			Type:    "Request-Driven Web Service",
			App:     "testapp",
			AppRunnerConfigurations: []*ServiceConfig{
				{
					CPU:         "1024",
					Environment: "test",
					Memory:      "2048",
					Port:        "80",
				},
				{
					CPU:         "2048",
					Environment: "prod",
					Memory:      "3072",
					Port:        "80",
				},
			},
			Routes: []*RDWSRoute{
				{
					Environment: "test",
					URL:         "https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com",
					Ingress:     rdwsIngressEnvironment,
				},
				{
					Environment: "prod",
					URL:         "https://tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com",
					Ingress:     rdwsIngressInternet,
				},
			},
			Variables: []*envVar{
				{
					Environment: "test",
					Name:        "COPILOT_ENVIRONMENT_NAME",
					Value:       "test",
				},
				{
					Environment: "prod",
					Name:        "COPILOT_ENVIRONMENT_NAME",
					Value:       "prod",
				},
			},
			Secrets: []*rdwsSecret{
				{
					Environment: "prod",
					Name:        "my-ssm-secret",
					ValueFrom:   "prod",
				},
				{
					Environment: "test",
					Name:        "my-ssm-secret",
					ValueFrom:   "test",
				},
			},
			Resources: map[string][]*stack.Resource{
				"test": {
					{
						Type:       "AWS::AppRunner::Service",
						PhysicalID: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-test-testsvc",
					},
				},
				"prod": {
					{
						Type:       "AWS::AppRunner::Service",
						PhysicalID: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-prod-testsvc",
					},
				},
			},
			environments: []string{"test", "prod"},
		}
		human := svcDesc.HumanString()
		json, _ := svcDesc.JSONString()

		require.Equal(t, wantedHumanString, human)
		require.Equal(t, wantedJSONString, json)
	})

	t.Run("correct output including resources with observability", func(t *testing.T) {
		wantedHumanString := `About

  Application  testapp
  Name         testsvc
  Type         Request-Driven Web Service

Configurations

  Environment  CPU (vCPU)  Memory (MiB)  Port
  -----------  ----------  ------------  ----
  test         1           2048          80
  prod         2           3072            "

Observability

  Environment  Tracing
  -----------  -------
  test         mockVendor
  prod         None

Routes

  Environment  Ingress      URL
  -----------  -------      ---
  test         environment  https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com
  prod         internet     https://tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com

Variables

  Name                      Environment  Value
  ----                      -----------  -----
  COPILOT_ENVIRONMENT_NAME  prod         prod
    "                       test         test

Secrets

  Name           Environment  Value
  ----           -----------  -----
  my-ssm-secret  prod         prod
    "            test         test

Resources

  test
    AWS::AppRunner::Service  arn:aws:apprunner:us-east-1:111111111111:service/testapp-test-testsvc

  prod
    AWS::AppRunner::Service  arn:aws:apprunner:us-east-1:111111111111:service/testapp-prod-testsvc
`
		wantedJSONString := "{\"service\":\"testsvc\",\"type\":\"Request-Driven Web Service\",\"application\":\"testapp\",\"configurations\":[{\"environment\":\"test\",\"port\":\"80\",\"cpu\":\"1024\",\"memory\":\"2048\"},{\"environment\":\"prod\",\"port\":\"80\",\"cpu\":\"2048\",\"memory\":\"3072\"}],\"routes\":[{\"environment\":\"test\",\"url\":\"https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com\",\"ingress\":\"environment\"},{\"environment\":\"prod\",\"url\":\"https://tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com\",\"ingress\":\"internet\"}],\"variables\":[{\"environment\":\"prod\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"prod\"},{\"environment\":\"test\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"test\"}],\"resources\":{\"prod\":[{\"type\":\"AWS::AppRunner::Service\",\"physicalID\":\"arn:aws:apprunner:us-east-1:111111111111:service/testapp-prod-testsvc\"}],\"test\":[{\"type\":\"AWS::AppRunner::Service\",\"physicalID\":\"arn:aws:apprunner:us-east-1:111111111111:service/testapp-test-testsvc\"}]},\"observability\":[{\"environment\":\"test\",\"tracing\":{\"vendor\":\"mockVendor\"}},{\"environment\":\"prod\"}],\"secrets\":[{\"environment\":\"prod\",\"name\":\"my-ssm-secret\",\"value\":\"prod\"},{\"environment\":\"test\",\"name\":\"my-ssm-secret\",\"value\":\"test\"}]}\n"
		svcDesc := &rdWebSvcDesc{
			Service: "testsvc",
			Type:    "Request-Driven Web Service",
			App:     "testapp",
			AppRunnerConfigurations: []*ServiceConfig{
				{
					CPU:         "1024",
					Environment: "test",
					Memory:      "2048",
					Port:        "80",
				},
				{
					CPU:         "2048",
					Environment: "prod",
					Memory:      "3072",
					Port:        "80",
				},
			},
			Routes: []*RDWSRoute{
				{
					Environment: "test",
					URL:         "https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com",
					Ingress:     rdwsIngressEnvironment,
				},
				{
					Environment: "prod",
					URL:         "https://tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com",
					Ingress:     rdwsIngressInternet,
				},
			},
			Variables: []*envVar{
				{
					Environment: "test",
					Name:        "COPILOT_ENVIRONMENT_NAME",
					Value:       "test",
				},
				{
					Environment: "prod",
					Name:        "COPILOT_ENVIRONMENT_NAME",
					Value:       "prod",
				},
			},
			Secrets: []*rdwsSecret{
				{
					Environment: "prod",
					Name:        "my-ssm-secret",
					ValueFrom:   "prod",
				},
				{
					Environment: "test",
					Name:        "my-ssm-secret",
					ValueFrom:   "test",
				},
			},
			Observability: []observabilityInEnv{
				{
					Environment: "test",
					Tracing: &tracing{
						Vendor: "mockVendor",
					},
				},
				{
					Environment: "prod",
				},
			},
			Resources: map[string][]*stack.Resource{
				"test": {
					{
						Type:       "AWS::AppRunner::Service",
						PhysicalID: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-test-testsvc",
					},
				},
				"prod": {
					{
						Type:       "AWS::AppRunner::Service",
						PhysicalID: "arn:aws:apprunner:us-east-1:111111111111:service/testapp-prod-testsvc",
					},
				},
			},
			environments: []string{"test", "prod"},
		}
		human := svcDesc.HumanString()
		json, _ := svcDesc.JSONString()

		require.Equal(t, wantedHumanString, human)
		require.Equal(t, wantedJSONString, json)
	})
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

const humanStringWithResources = `About

  Application       testapp
  Name              testsvc
  Type              Request-Driven Web Service

Configurations

  Environment       CPU (vCPU)          Memory (MiB)        Port
  -----------       ----------          ------------        ----
  test              1                   2048                80
  prod              2                   3072                  "

Routes

  Environment       URL
  -----------       ---
  test              https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com
  prod              https://tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com

Variables

  Name                      Environment         Value
  ----                      -----------         -----
  COPILOT_ENVIRONMENT_NAME  prod                prod
    "                       test                test

Resources

  test
    AWS::AppRunner::Service  arn:aws:apprunner:us-east-1:111111111111:service/testapp-test-testsvc

  prod
    AWS::AppRunner::Service  arn:aws:apprunner:us-east-1:111111111111:service/testapp-prod-testsvc
`

type apprunnerSvcDescriberMocks struct {
	storeSvc        *mocks.MockDeployedEnvServicesLister
	ecsSvcDescriber *mocks.MockapprunnerSvcDescriber
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
		"return error if fail to retrieve service resources": {
			shouldOutputResources: true,
			setupMocks: func(m apprunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().ListEnvironmentsDeployedTo(testApp, testSvc).Return([]string{testEnv}, nil),
					m.ecsSvcDescriber.EXPECT().Service().Return(&apprunner.Service{}, nil),
					m.ecsSvcDescriber.EXPECT().ServiceStackResources().Return(nil, mockErr),
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
					}, nil),
					m.ecsSvcDescriber.EXPECT().ServiceStackResources().Return([]*stack.Resource{
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
					}, nil),
					m.ecsSvcDescriber.EXPECT().ServiceStackResources().Return([]*stack.Resource{
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
				Configurations: []*ServiceConfig{
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
				Routes: []*WebServiceRoute{
					{
						Environment: "test",
						URL:         "https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com",
					},
					{
						Environment: "prod",
						URL:         "https://tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com",
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
			mockSvcDescriber := mocks.NewMockapprunnerSvcDescriber(ctrl)
			mocks := apprunnerSvcDescriberMocks{
				storeSvc:        mockStore,
				ecsSvcDescriber: mockSvcDescriber,
			}

			tc.setupMocks(mocks)

			d := &RDWebServiceDescriber{
				app:             testApp,
				svc:             testSvc,
				enableResources: tc.shouldOutputResources,
				store:           mockStore,
				envSvcDescribers: map[string]apprunnerSvcDescriber{
					"test": mockSvcDescriber,
					"prod": mockSvcDescriber,
				},
				initServiceDescriber: func(string) error { return nil },
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
		wantedJSONString := "{\"service\":\"testsvc\",\"type\":\"Request-Driven Web Service\",\"application\":\"testapp\",\"configurations\":[{\"environment\":\"test\",\"port\":\"80\",\"cpu\":\"1024\",\"memory\":\"2048\"},{\"environment\":\"prod\",\"port\":\"80\",\"cpu\":\"2048\",\"memory\":\"3072\"}],\"routes\":[{\"environment\":\"test\",\"url\":\"https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com\"},{\"environment\":\"prod\",\"url\":\"https://tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com\"}],\"variables\":[{\"environment\":\"prod\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"prod\"},{\"environment\":\"test\",\"name\":\"COPILOT_ENVIRONMENT_NAME\",\"value\":\"test\"}],\"resources\":{\"prod\":[{\"type\":\"AWS::AppRunner::Service\",\"physicalID\":\"arn:aws:apprunner:us-east-1:111111111111:service/testapp-prod-testsvc\"}],\"test\":[{\"type\":\"AWS::AppRunner::Service\",\"physicalID\":\"arn:aws:apprunner:us-east-1:111111111111:service/testapp-test-testsvc\"}]}}\n"
		svcDesc := &rdWebSvcDesc{
			Service: "testsvc",
			Type:    "Request-Driven Web Service",
			App:     "testapp",
			Configurations: []*ServiceConfig{
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
			Routes: []*WebServiceRoute{
				{
					Environment: "test",
					URL:         "https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com",
				},
				{
					Environment: "prod",
					URL:         "https://tumkjmvjjf.public.us-east-1.apprunner.amazonaws.com",
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

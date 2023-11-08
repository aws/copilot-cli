// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/aws/copilot-cli/internal/pkg/template"

	describeStack "github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestLBWebServiceDescriber_URI(t *testing.T) {
	const (
		testApp                  = "phonetool"
		testEnv                  = "test"
		testSvc                  = "jobs"
		testEnvSubdomain         = "test.phonetool.com"
		testEnvLBDNSName         = "abc.us-west-1.elb.amazonaws.com"
		testSvcPath              = "/"
		testALBAccessible        = "true"
		testALBInaccessible      = "false"
		testCloudFrontDomainName = "test.cloudfront.com"

		testNLBDNSName = "def.us-west-2.elb.amazonaws.com"
	)
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks lbWebSvcDescriberMocks)

		wantedURI   string
		wantedError error
	}{
		"fail to get stack resources of service stack": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get stack resources for service jobs: some error"),
		},
		"fail to get params of the service stack when fetching ALB uris": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get stack parameters for service jobs: some error"),
		},
		"fail to get outputs of environment stack when fetching ALB uris": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.WorkloadRulePathParamKey: testSvcPath,
					}, nil),
					m.envDescriber.EXPECT().Outputs().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get stack outputs for environment test: some error"),
		},
		"fail to get listener rule host-header": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.WorkloadRulePathParamKey: testSvcPath,
						stack.WorkloadHTTPSParamKey:    "true",
					}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID:  svcStackResourceHTTPSListenerRuleLogicalID,
							Type:       svcStackResourceListenerRuleResourceType,
							PhysicalID: "mockRuleARN",
						},
					}, nil),
					m.lbDescriber.EXPECT().ListenerRulesHostHeaders([]string{"mockRuleARN"}).
						Return(nil, mockErr),
				)
			},

			wantedError: fmt.Errorf("get host headers for listener rules mockRuleARN: some error"),
		},
		"https web service": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.WorkloadRulePathParamKey: testSvcPath,
						stack.WorkloadHTTPSParamKey:    "true",
					}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID:  "HTTPSListenerRule",
							Type:       svcStackResourceListenerRuleResourceType,
							PhysicalID: "mockRuleARN1",
						},
						{
							LogicalID:  "HTTPSListenerRule1",
							Type:       svcStackResourceListenerRuleResourceType,
							PhysicalID: "mockRuleARN2",
						},
					}, nil),
					m.lbDescriber.EXPECT().ListenerRulesHostHeaders([]string{"mockRuleARN1", "mockRuleARN2"}).
						Return([]string{"jobs.test.phonetool.com", "phonetool.com", "v1.phonetool.com"}, nil),
				)
			},
			wantedURI: "https://jobs.test.phonetool.com, https://phonetool.com, or https://v1.phonetool.com",
		},
		"http web service": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.WorkloadRulePathParamKey: "mySvc",
					}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
				)
			},

			wantedURI: "http://abc.us-west-1.elb.amazonaws.com/mySvc",
		},
		"swallow error if fail to get env vars with imported ALB": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceListenerRuleForImportedALBLogicalID,
						},
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil).Times(1),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.WorkloadRulePathParamKey: "/",
					}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceListenerRuleForImportedALBLogicalID,
						},
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil).Times(1),
					m.lbDescriber.EXPECT().ListenerRulesHostHeaders(nil).
						Return(nil, nil),
					m.ecsDescriber.EXPECT().EnvVars().Return(nil, errors.New("some error")),
					m.envDescriber.EXPECT().Outputs().Return(nil, nil),
				)
			},

			wantedURI: "http://",
		},
		"http web service with imported ALB": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceListenerRuleForImportedALBLogicalID,
						},
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil).Times(1),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.WorkloadRulePathParamKey: "/",
					}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceListenerRuleForImportedALBLogicalID,
						},
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil).Times(1),
					m.lbDescriber.EXPECT().ListenerRulesHostHeaders(nil).
						Return(nil, nil),
					m.ecsDescriber.EXPECT().EnvVars().Return([]*ecs.ContainerEnvVar{
						{
							Name:  LBDNS,
							Value: "mockDNSName",
						},
					}, nil),
				)
			},

			wantedURI: "http://mockDNSName",
		},
		"http web service with cloudfront": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.WorkloadRulePathParamKey: "mySvc",
					}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						envOutputPublicALBAccessible:       testALBAccessible,
						envOutputCloudFrontDomainName:      testCloudFrontDomainName,
					}, nil),
				)
			},

			wantedURI: "http://abc.us-west-1.elb.amazonaws.com/mySvc or http://test.cloudfront.com/mySvc",
		},
		"http web service with cloudfront and alb disabled": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.WorkloadRulePathParamKey: "mySvc",
					}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						envOutputPublicALBAccessible:       testALBInaccessible,
						envOutputCloudFrontDomainName:      testCloudFrontDomainName,
					}, nil),
				)
			},
			wantedURI: "http://test.cloudfront.com/mySvc",
		},
		"fail to get parameters of service stack when fetching NLB uris": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
					{
						LogicalID: svcStackResourceNLBTargetGroupLogicalID,
					},
				}, nil)
				m.ecsDescriber.EXPECT().Params().Return(nil, mockErr)
			},
			wantedError: fmt.Errorf("get stack parameters for service jobs: some error"),
		},
		"fail to get outputs of service stack when fetching NLB uris": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
					{
						LogicalID: svcStackResourceNLBTargetGroupLogicalID,
					},
				}, nil)
				m.ecsDescriber.EXPECT().Params().Return(map[string]string{
					stack.LBWebServiceNLBPortParamKey:      "443",
					stack.LBWebServiceDNSDelegatedParamKey: "false",
				}, nil)
				m.ecsDescriber.EXPECT().Outputs().Return(nil, mockErr)
			},
			wantedError: fmt.Errorf("get stack outputs for service jobs: some error"),
		},
		"fail to get outputs of environment stack when fetching NLB uris": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
					{
						LogicalID: svcStackResourceNLBTargetGroupLogicalID,
					},
				}, nil)
				m.ecsDescriber.EXPECT().Params().Return(map[string]string{
					stack.LBWebServiceNLBPortParamKey:      "443",
					stack.LBWebServiceDNSDelegatedParamKey: "true",
				}, nil)
				m.envDescriber.EXPECT().Outputs().Return(nil, mockErr)
			},
			wantedError: fmt.Errorf("get stack outputs for environment test: some error"),
		},
		"nlb web service": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceNLBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceNLBPortParamKey:      "443",
						stack.LBWebServiceDNSDelegatedParamKey: "false",
					}, nil),
					m.ecsDescriber.EXPECT().Outputs().Return(map[string]string{
						svcOutputPublicNLBDNSName: testNLBDNSName,
					}, nil),
				)
			},
			wantedURI: "def.us-west-2.elb.amazonaws.com:443",
		},
		"nlb web service with default DNS name": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceNLBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceNLBPortParamKey:      "443",
						stack.LBWebServiceDNSDelegatedParamKey: "true",
					}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputSubdomain: testEnvSubdomain,
					}, nil),
				)
			},
			wantedURI: "jobs-nlb.test.phonetool.com:443",
		},
		"nlb web service with alias": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceNLBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceNLBPortParamKey:      "443",
						stack.LBWebServiceDNSDelegatedParamKey: "true",
						stack.LBWebServiceNLBAliasesParamKey:   "alias1.phonetool.com,alias2.phonetool.com",
					}, nil),
				)
			},
			wantedURI: "alias1.phonetool.com:443 or alias2.phonetool.com:443",
		},
		"both http and nlb with alias": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceNLBTargetGroupLogicalID,
						},
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.WorkloadRulePathParamKey: testSvcPath,
						stack.WorkloadHTTPSParamKey:    "true",
					}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID:  svcStackResourceHTTPSListenerRuleLogicalID,
							Type:       svcStackResourceListenerRuleResourceType,
							PhysicalID: "mockRuleARN",
						},
					}, nil),
					m.lbDescriber.EXPECT().ListenerRulesHostHeaders([]string{"mockRuleARN"}).
						Return([]string{"example.com", "v1.example.com"}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceNLBPortParamKey:      "443",
						stack.LBWebServiceDNSDelegatedParamKey: "true",
						stack.LBWebServiceNLBAliasesParamKey:   "alias1.phonetool.com,alias2.phonetool.com",
					}, nil),
				)
			},
			wantedURI: "https://example.com, https://v1.example.com, alias1.phonetool.com:443, or alias2.phonetool.com:443",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSvcDescriber := mocks.NewMockecsDescriber(ctrl)
			mockEnvDescriber := mocks.NewMockenvDescriber(ctrl)
			mockLBDescriber := mocks.NewMocklbDescriber(ctrl)
			mocks := lbWebSvcDescriberMocks{
				ecsDescriber: mockSvcDescriber,
				envDescriber: mockEnvDescriber,
				lbDescriber:  mockLBDescriber,
			}

			tc.setupMocks(mocks)

			d := &LBWebServiceDescriber{
				app:                      testApp,
				svc:                      testSvc,
				initECSServiceDescribers: func(s string) (ecsDescriber, error) { return mockSvcDescriber, nil },
				initEnvDescribers:        func(s string) (envDescriber, error) { return mockEnvDescriber, nil },
				initLBDescriber:          func(s string) (lbDescriber, error) { return mockLBDescriber, nil },
			}

			// WHEN
			actual, err := d.URI(testEnv)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedURI, actual.URI)
			}
		})
	}
}

func TestBackendServiceDescriber_URI(t *testing.T) {
	const (
		testApp                = "phonetool"
		testEnv                = "test"
		testSvc                = "my-svc"
		testEnvInternalDNSName = "abc.us-west-1.elb.amazonaws.internal"
	)
	testCases := map[string]struct {
		setupMocks func(mocks lbWebSvcDescriberMocks)

		wantedURI   string
		wantedError error
	}{
		"should return a blank service discovery URI if there is no port exposed": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				m.ecsDescriber.EXPECT().StackResources().Return(nil, nil)
				m.ecsDescriber.EXPECT().Params().Return(map[string]string{
					stack.WorkloadTargetPortParamKey: template.NoExposedContainerPort, // No port is set for the backend service.
				}, nil)
			},
			wantedURI: BlankServiceDiscoveryURI,
		},
		"should return service connect endpoint if port is exposed": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				m.ecsDescriber.EXPECT().StackResources().Return(nil, nil)
				m.ecsDescriber.EXPECT().Params().Return(map[string]string{
					stack.WorkloadTargetPortParamKey: "8080",
				}, nil)
				m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return([]string{"my-svc:8080"}, nil)
			},
			wantedURI: "my-svc:8080",
		},
		"should return service discovery endpoint if port is exposed": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				m.ecsDescriber.EXPECT().StackResources().Return(nil, nil)
				m.ecsDescriber.EXPECT().Params().Return(map[string]string{
					stack.WorkloadTargetPortParamKey: "8080",
				}, nil)
				m.ecsDescriber.EXPECT().ServiceConnectDNSNames().Return(nil, nil)
				m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.app.local", nil)
			},
			wantedURI: "my-svc.test.app.local:8080",
		},
		"internal url http": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				resources := []*describeStack.Resource{
					{
						Type:       "AWS::ElasticLoadBalancingV2::TargetGroup",
						LogicalID:  svcStackResourceALBTargetGroupLogicalID,
						PhysicalID: "targetGroupARN",
					},
					{
						Type:       svcStackResourceListenerRuleResourceType,
						LogicalID:  svcStackResourceHTTPListenerRuleLogicalID,
						PhysicalID: "mockRuleARN",
					},
				}
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return(resources, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.WorkloadRulePathParamKey: "mySvc",
					}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return(resources, nil),
					m.lbDescriber.EXPECT().ListenerRulesHostHeaders([]string{"mockRuleARN"}).
						Return([]string{"jobs.test.phonetool.internal", "1234.us-west-2.internal.aws.com"}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputInternalLoadBalancerDNSName: "1234.us-west-2.internal.aws.com",
					}, nil),
				)
			},
			wantedURI: "http://jobs.test.phonetool.internal/mySvc",
		},
		"internal url https": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.WorkloadRulePathParamKey: "/",
						stack.WorkloadHTTPSParamKey:    "true",
					}, nil),
					m.ecsDescriber.EXPECT().StackResources().Return([]*describeStack.Resource{
						{
							LogicalID:  "HTTPSListenerRuleGroup0",
							Type:       svcStackResourceListenerRuleResourceType,
							PhysicalID: "mockRuleARN1",
						},
						{
							LogicalID:  "HTTPSListenerRuleGroup1",
							Type:       svcStackResourceListenerRuleResourceType,
							PhysicalID: "mockRuleARN2",
						},
					}, nil),
					m.lbDescriber.EXPECT().ListenerRulesHostHeaders([]string{"mockRuleARN1", "mockRuleARN2"}).
						Return([]string{"jobs.test.phonetool.com", "phonetool.com", "v1.phonetool.com"}, nil),
				)
			},
			wantedURI: "https://jobs.test.phonetool.com, https://phonetool.com, or https://v1.phonetool.com",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSvcDescriber := mocks.NewMockecsDescriber(ctrl)
			mockEnvDescriber := mocks.NewMockenvDescriber(ctrl)
			mockLBDescriber := mocks.NewMocklbDescriber(ctrl)
			mocks := lbWebSvcDescriberMocks{
				ecsDescriber: mockSvcDescriber,
				envDescriber: mockEnvDescriber,
				lbDescriber:  mockLBDescriber,
			}

			tc.setupMocks(mocks)

			d := &BackendServiceDescriber{
				app:                      testApp,
				svc:                      testSvc,
				initECSServiceDescribers: func(s string) (ecsDescriber, error) { return mockSvcDescriber, nil },
				initEnvDescribers:        func(s string) (envDescriber, error) { return mockEnvDescriber, nil },
				initLBDescriber:          func(s string) (lbDescriber, error) { return mockLBDescriber, nil },
			}

			// WHEN
			actual, err := d.URI(testEnv)

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedURI, actual.URI)
			}
		})
	}
}

func TestRDWebServiceDescriber_URI(t *testing.T) {
	const (
		testApp    = "phonetool"
		testEnv    = "test"
		testSvc    = "frontend"
		testSvcURL = "https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com"
	)
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks apprunnerSvcDescriberMocks)

		wantedURI   URI
		wantedError error
	}{
		"fail to get outputs of service stack": {
			setupMocks: func(m apprunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsSvcDescriber.EXPECT().ServiceURL().Return("", mockErr),
				)
			},
			wantedError: fmt.Errorf(`get outputs for service "frontend": some error`),
		},
		"fail to check if private": {
			setupMocks: func(m apprunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsSvcDescriber.EXPECT().ServiceURL().Return(testSvcURL, nil),
					m.ecsSvcDescriber.EXPECT().IsPrivate().Return(false, mockErr),
				)
			},
			wantedError: fmt.Errorf(`check if service "frontend" is private: some error`),
		},
		"succeed in getting public service uri": {
			setupMocks: func(m apprunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsSvcDescriber.EXPECT().ServiceURL().Return(testSvcURL, nil),
					m.ecsSvcDescriber.EXPECT().IsPrivate().Return(false, nil),
				)
			},
			wantedURI: URI{
				URI:        "https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com",
				AccessType: URIAccessTypeInternet,
			},
		},
		"succeed in getting private service uri": {
			setupMocks: func(m apprunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsSvcDescriber.EXPECT().ServiceURL().Return(testSvcURL, nil),
					m.ecsSvcDescriber.EXPECT().IsPrivate().Return(true, nil),
				)
			},
			wantedURI: URI{
				URI:        "https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com",
				AccessType: URIAccessTypeInternal,
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSvcDescriber := mocks.NewMockapprunnerDescriber(ctrl)
			mocks := apprunnerSvcDescriberMocks{
				ecsSvcDescriber: mockSvcDescriber,
			}

			tc.setupMocks(mocks)

			d := &RDWebServiceDescriber{
				app:                    testApp,
				svc:                    testSvc,
				initAppRunnerDescriber: func(string) (apprunnerDescriber, error) { return mockSvcDescriber, nil },
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

func TestLBWebServiceURI_String(t *testing.T) {
	testCases := map[string]struct {
		accessDNSNames []string
		accessPath     string
		accessHTTPS    bool

		wanted string
	}{
		"http": {
			accessDNSNames: []string{"abc.us-west-1.elb.amazonaws.com"},
			accessPath:     "svc",

			wanted: "http://abc.us-west-1.elb.amazonaws.com/svc",
		},
		"http with / path": {
			accessDNSNames: []string{"jobs.test.phonetool.com"},
			accessPath:     "/",

			wanted: "http://jobs.test.phonetool.com",
		},
		"cloudfront": {
			accessDNSNames: []string{"abc.cloudfront.net"},
			accessPath:     "svc",

			wanted: "http://abc.cloudfront.net/svc",
		},
		"cloudfront with https": {
			accessDNSNames: []string{"abc.cloudfront.net"},
			accessPath:     "svc",
			accessHTTPS:    true,

			wanted: "https://abc.cloudfront.net/svc",
		},
		"https": {
			accessDNSNames: []string{"jobs.test.phonetool.com"},
			accessPath:     "svc",
			accessHTTPS:    true,

			wanted: "https://jobs.test.phonetool.com/svc",
		},
		"https with / path": {
			accessDNSNames: []string{"jobs.test.phonetool.com"},
			accessPath:     "/",
			accessHTTPS:    true,

			wanted: "https://jobs.test.phonetool.com",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			uri := &LBWebServiceURI{
				access: accessURI{
					DNSNames: tc.accessDNSNames,
					Path:     tc.accessPath,
					HTTPS:    tc.accessHTTPS,
				},
			}

			require.Equal(t, tc.wanted, uri.String())
		})
	}
}

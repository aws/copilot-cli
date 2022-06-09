// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"

	describeStack "github.com/aws/copilot-cli/internal/pkg/describe/stack"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestLBWebServiceDescriber_URI(t *testing.T) {
	const (
		testApp          = "phonetool"
		testEnv          = "test"
		testSvc          = "jobs"
		testEnvSubdomain = "test.phonetool.com"
		testEnvLBDNSName = "abc.us-west-1.elb.amazonaws.com"
		testSvcPath      = "/"

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
					m.ecsDescriber.EXPECT().ServiceStackResources().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get stack resources for service jobs: some error"),
		},
		"fail to get params of the service stack when fetching ALB uris": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
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
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
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
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.WorkloadRulePathParamKey: testSvcPath,
						stack.WorkloadHTTPSParamKey:    "true",
					}, nil),
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
						{
							LogicalID:  svcStackResourceHTTPSListenerRuleLogicalID,
							Type:       svcStackResourceHTTPSListenerRuleResourceType,
							PhysicalID: "mockRuleARN",
						},
					}, nil),
					m.lbDescriber.EXPECT().ListenerRuleHostHeaders("mockRuleARN").
						Return(nil, mockErr),
				)
			},

			wantedError: fmt.Errorf("get host headers for listener rule mockRuleARN: some error"),
		},
		"https web service": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.WorkloadRulePathParamKey: testSvcPath,
						stack.WorkloadHTTPSParamKey:    "true",
					}, nil),
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
						{
							LogicalID:  svcStackResourceHTTPSListenerRuleLogicalID,
							Type:       svcStackResourceHTTPSListenerRuleResourceType,
							PhysicalID: "mockRuleARN",
						},
					}, nil),
					m.lbDescriber.EXPECT().ListenerRuleHostHeaders("mockRuleARN").
						Return([]string{"jobs.test.phonetool.com", "phonetool.com"}, nil),
				)
			},
			wantedURI: "https://jobs.test.phonetool.com or https://phonetool.com",
		},
		"http web service": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
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
		"fail to get parameters of service stack when fetching NLB uris": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
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
				m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
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
				m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
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
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
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
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
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
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
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
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
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
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
						{
							LogicalID:  svcStackResourceHTTPSListenerRuleLogicalID,
							Type:       svcStackResourceHTTPSListenerRuleResourceType,
							PhysicalID: "mockRuleARN",
						},
					}, nil),
					m.lbDescriber.EXPECT().ListenerRuleHostHeaders("mockRuleARN").
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
			actual, _, err := d.URI(testEnv)

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
				m.ecsDescriber.EXPECT().ServiceStackResources().Return(nil, nil)
				m.ecsDescriber.EXPECT().Params().Return(map[string]string{
					stack.WorkloadContainerPortParamKey: stack.NoExposedContainerPort, // No port is set for the backend service.
				}, nil)
			},
			wantedURI: BlankServiceDiscoveryURI,
		},
		"should return service discovery endpoint if port is exposed": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				m.ecsDescriber.EXPECT().ServiceStackResources().Return(nil, nil)
				m.ecsDescriber.EXPECT().Params().Return(map[string]string{
					stack.WorkloadContainerPortParamKey: "8080",
				}, nil)
				m.envDescriber.EXPECT().ServiceDiscoveryEndpoint().Return("test.app.local", nil)
			},
			wantedURI: "my-svc.test.app.local:8080",
		},
		"internal url http": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.WorkloadRulePathParamKey: "mySvc",
					}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputInternalLoadBalancerDNSName: testEnvInternalDNSName,
					}, nil),
				)
			},
			wantedURI: "http://abc.us-west-1.elb.amazonaws.internal/mySvc",
		},
		"internal url https": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.WorkloadRulePathParamKey: "/",
						stack.WorkloadHTTPSParamKey:    "true",
					}, nil),
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
						{
							LogicalID:  svcStackResourceHTTPSListenerRuleLogicalID,
							Type:       svcStackResourceHTTPSListenerRuleResourceType,
							PhysicalID: "mockRuleARN",
						},
					}, nil),
					m.lbDescriber.EXPECT().ListenerRuleHostHeaders("mockRuleARN").
						Return([]string{"jobs.test.phonetool.com", "phonetool.com"}, nil),
				)
			},
			wantedURI: "https://jobs.test.phonetool.com or https://phonetool.com",
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
			actual, _, err := d.URI(testEnv)

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

		wantedURI   string
		wantedError error
	}{
		"fail to get outputs of service stack": {
			setupMocks: func(m apprunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsSvcDescriber.EXPECT().ServiceURL().Return("", mockErr),
				)
			},
			wantedError: fmt.Errorf("get outputs for service frontend: some error"),
		},
		"succeed in getting outputs of service stack": {
			setupMocks: func(m apprunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsSvcDescriber.EXPECT().ServiceURL().Return(testSvcURL, nil),
				)
			},

			wantedURI: "https://6znxd4ra33.public.us-east-1.apprunner.amazonaws.com",
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
			actual, _, err := d.URI(testEnv)

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
		albDNSNames []string
		albPath     string
		albHTTPS    bool

		wanted string
	}{
		"http": {
			albDNSNames: []string{"abc.us-west-1.elb.amazonaws.com"},
			albPath:     "svc",

			wanted: "http://abc.us-west-1.elb.amazonaws.com/svc",
		},
		"http with / path": {
			albDNSNames: []string{"jobs.test.phonetool.com"},
			albPath:     "/",

			wanted: "http://jobs.test.phonetool.com",
		},
		"https": {
			albDNSNames: []string{"jobs.test.phonetool.com"},
			albPath:     "svc",
			albHTTPS:    true,

			wanted: "https://jobs.test.phonetool.com/svc",
		},
		"https with / path": {
			albDNSNames: []string{"jobs.test.phonetool.com"},
			albPath:     "/",
			albHTTPS:    true,

			wanted: "https://jobs.test.phonetool.com",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			uri := &LBWebServiceURI{
				albURI: albURI{
					DNSNames: tc.albDNSNames,
					Path:     tc.albPath,
					HTTPS:    tc.albHTTPS,
				},
			}

			require.Equal(t, tc.wanted, uri.String())
		})
	}
}

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
		"fail to get parameters of environment stack when fetching ALB uris": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.envDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get stack parameters for environment test: some error"),
		},
		"fail to get outputs of environment stack when fetching ALB uris": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.envDescriber.EXPECT().Params().Return(map[string]string{}, nil),
					m.envDescriber.EXPECT().Outputs().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get stack outputs for environment test: some error"),
		},
		"fail to get parameters of service stack when fetching ALB uris": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.envDescriber.EXPECT().Params().Return(map[string]string{}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						envOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get stack parameters for service jobs: some error"),
		},
		"https web service": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.envDescriber.EXPECT().Params().Return(map[string]string{}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						envOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey: testSvcPath,
						stack.LBWebServiceHTTPSParamKey:    "true",
					}, nil),
				)
			},

			wantedURI: "https://jobs.test.phonetool.com",
		},
		"http web service": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.envDescriber.EXPECT().Params().Return(map[string]string{}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey: "*",
					}, nil),
				)
			},

			wantedURI: "http://abc.us-west-1.elb.amazonaws.com/*",
		},
		"with alias": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.ecsDescriber.EXPECT().ServiceStackResources().Return([]*describeStack.Resource{
						{
							LogicalID: svcStackResourceALBTargetGroupLogicalID,
						},
					}, nil),
					m.envDescriber.EXPECT().Params().Return(map[string]string{
						stack.EnvParamAliasesKey: `{"jobs": ["example.com", "v1.example.com"]}`,
					}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						envOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey: testSvcPath,
						stack.LBWebServiceHTTPSParamKey:    "true",
					}, nil),
				)
			},

			wantedURI: "https://example.com or https://v1.example.com",
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
					m.envDescriber.EXPECT().Params().Return(map[string]string{
						stack.EnvParamAliasesKey: `{"jobs": ["example.com", "v1.example.com"]}`,
					}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						envOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.ecsDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey: testSvcPath,
						stack.LBWebServiceHTTPSParamKey:    "true",
					}, nil),
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
			mocks := lbWebSvcDescriberMocks{
				ecsDescriber: mockSvcDescriber,
				envDescriber: mockEnvDescriber,
			}

			tc.setupMocks(mocks)

			d := &LBWebServiceDescriber{
				app:         testApp,
				svc:         testSvc,
				initClients: func(string) error { return nil },

				ecsServiceDescribers: map[string]ecsDescriber{
					"test": mockSvcDescriber,
				},
				envDescriber: map[string]envDescriber{
					"test": mockEnvDescriber,
				},
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

func TestBackendServiceDescriber_URI(t *testing.T) {
	t.Run("should return a blank service discovery URI if there is no port exposed", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		m := mocks.NewMockecsDescriber(ctrl)
		m.EXPECT().Params().Return(map[string]string{
			stack.LBWebServiceContainerPortParamKey: stack.NoExposedContainerPort, // No port is set for the backend service.
		}, nil)

		d := &BackendServiceDescriber{
			initClients: func(string) error { return nil },

			ecsServiceDescribers: map[string]ecsDescriber{
				"test": m,
			},
		}

		// WHEN
		actual, err := d.URI("test")

		// THEN
		require.NoError(t, err)
		require.Equal(t, BlankServiceDiscoveryURI, actual)
	})
	t.Run("should return service discovery endpoint if port is exposed", func(t *testing.T) {
		// GIVEN
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mockSvcStack := mocks.NewMockecsDescriber(ctrl)
		mockSvcStack.EXPECT().Params().Return(map[string]string{
			stack.LBWebServiceContainerPortParamKey: "8080",
		}, nil)
		mockEnvStack := mocks.NewMockenvDescriber(ctrl)
		mockEnvStack.EXPECT().ServiceDiscoveryEndpoint().Return("test.app.local", nil)

		d := &BackendServiceDescriber{
			svc:         "hello",
			initClients: func(string) error { return nil },

			ecsServiceDescribers: map[string]ecsDescriber{
				"test": mockSvcStack,
			},
			envStackDescriber: map[string]envDescriber{
				"test": mockEnvStack,
			},
		}

		// WHEN
		actual, err := d.URI("test")

		// THEN
		require.NoError(t, err)
		require.Equal(t, "hello.test.app.local:8080", actual)
	})
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
				app:         testApp,
				svc:         testSvc,
				initClients: func(string) error { return nil },

				envSvcDescribers: map[string]apprunnerDescriber{
					"test": mockSvcDescriber,
				},
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

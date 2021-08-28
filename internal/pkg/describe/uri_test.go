// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"

	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
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
					m.ecsStackDescriber.EXPECT().Params().Return(nil, mockErr),
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
					m.ecsStackDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey: testSvcPath,
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
					m.ecsStackDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey: "*",
					}, nil),
				)
			},

			wantedURI: "http://abc.us-west-1.elb.amazonaws.com/*",
		},
		"with alias": {
			setupMocks: func(m lbWebSvcDescriberMocks) {
				gomock.InOrder(
					m.envDescriber.EXPECT().Params().Return(map[string]string{
						stack.EnvParamAliasesKey: `{"jobs": ["example.com", "v1.example.com"]}`,
					}, nil),
					m.envDescriber.EXPECT().Outputs().Return(map[string]string{
						envOutputPublicLoadBalancerDNSName: testEnvLBDNSName,
						envOutputSubdomain:                 testEnvSubdomain,
					}, nil),
					m.ecsStackDescriber.EXPECT().Params().Return(map[string]string{
						stack.LBWebServiceRulePathParamKey: testSvcPath,
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

			mockSvcDescriber := mocks.NewMockecsStackDescriber(ctrl)
			mockEnvDescriber := mocks.NewMockenvDescriber(ctrl)
			mocks := lbWebSvcDescriberMocks{
				ecsStackDescriber: mockSvcDescriber,
				envDescriber:      mockEnvDescriber,
			}

			tc.setupMocks(mocks)

			d := &LBWebServiceDescriber{
				ecsServiceDescriber: &ecsServiceDescriber{
					app: testApp,
					svc: testSvc,
					svcStackDescriber: map[string]ecsStackDescriber{
						"test": mockSvcDescriber,
					},
					initDescribers: func(string) error { return nil },
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
		m := mocks.NewMockecsStackDescriber(ctrl)
		m.EXPECT().Params().Return(map[string]string{
			stack.LBWebServiceContainerPortParamKey: stack.NoExposedContainerPort, // No port is set for the backend service.
		}, nil)

		d := &BackendServiceDescriber{
			ecsServiceDescriber: &ecsServiceDescriber{
				svcStackDescriber: map[string]ecsStackDescriber{
					"test": m,
				},
				initDescribers: func(string) error { return nil },
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
		mockSvcStack := mocks.NewMockecsStackDescriber(ctrl)
		mockSvcStack.EXPECT().Params().Return(map[string]string{
			stack.LBWebServiceContainerPortParamKey: "8080",
		}, nil)
		mockEnvStack := mocks.NewMockenvDescriber(ctrl)
		mockEnvStack.EXPECT().ServiceDiscoveryEndpoint().Return("test.app.local", nil)

		d := &BackendServiceDescriber{
			ecsServiceDescriber: &ecsServiceDescriber{
				svc: "hello",
				svcStackDescriber: map[string]ecsStackDescriber{
					"test": mockSvcStack,
				},
				initDescribers: func(string) error { return nil },
			},
			envDescriber: map[string]envDescriber{
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

			mockSvcDescriber := mocks.NewMockapprunnerSvcDescriber(ctrl)
			mocks := apprunnerSvcDescriberMocks{
				ecsSvcDescriber: mockSvcDescriber,
			}

			tc.setupMocks(mocks)

			d := &RDWebServiceDescriber{
				app: testApp,
				svc: testSvc,
				envSvcDescribers: map[string]apprunnerSvcDescriber{
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

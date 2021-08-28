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

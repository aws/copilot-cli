// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/describe/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type appRunnerSvcDescriberMocks struct {
	storeSvc     *mocks.MockDeployedEnvServicesLister
	svcDescriber *mocks.MockappRunnerSvcDescriber
}

func TestRDWebServiceDescriber_URI(t *testing.T) {
	const (
		testApp    = "phonetool"
		testEnv    = "test"
		testSvc    = "frontend"
		testSvcURL = "6znxd4ra33.public.us-east-1.apprunner.amazonaws.com"
	)
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		setupMocks func(mocks appRunnerSvcDescriberMocks)

		wantedURI   string
		wantedError error
	}{
		"fail to get outputs of service stack": {
			setupMocks: func(m appRunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.svcDescriber.EXPECT().SvcOutputs().Return(nil, mockErr),
				)
			},
			wantedError: fmt.Errorf("get outputs for service frontend: some error"),
		},
		"succeed in getting outputs of service stack": {
			setupMocks: func(m appRunnerSvcDescriberMocks) {
				gomock.InOrder(
					m.svcDescriber.EXPECT().SvcOutputs().Return(map[string]string{
						svcOutputURL: testSvcURL,
					}, nil),
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

			mockSvcDescriber := mocks.NewMockappRunnerSvcDescriber(ctrl)
			mocks := appRunnerSvcDescriberMocks{
				svcDescriber: mockSvcDescriber,
			}

			tc.setupMocks(mocks)

			d := &RDWebServiceDescriber{
				app: testApp,
				svc: testSvc,
				svcDescriber: map[string]appRunnerSvcDescriber{
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

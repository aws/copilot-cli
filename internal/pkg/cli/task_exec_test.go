// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type execTaskMocks struct {
	storeSvc *mocks.Mockstore
}

func TestTaskExec_Validate(t *testing.T) {
	const (
		mockApp       = "my-app"
		mockEnv       = "my-env"
		mockTaskGroup = "my-task-group"
		mockCluster   = "my-cluster"
	)
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		inApp       string
		inEnv       string
		inTaskGroup string
		inCluster   string
		setupMocks  func(mocks execTaskMocks)

		wantedError error
	}{
		"should bubble error if specify both cluster and app": {
			inApp:      mockApp,
			inCluster:  mockCluster,
			setupMocks: func(m execTaskMocks) {},

			wantedError: fmt.Errorf("cannot specify both cluster flag and app or env flags"),
		},
		"should bubble error if specify both cluster and env": {
			inEnv:      mockEnv,
			inCluster:  mockCluster,
			setupMocks: func(m execTaskMocks) {},

			wantedError: fmt.Errorf("cannot specify both cluster flag and app or env flags"),
		},
		"should bubble error if failed to get app": {
			inApp: mockApp,
			setupMocks: func(m execTaskMocks) {
				m.storeSvc.EXPECT().GetApplication(mockApp).Return(nil, mockErr)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"should bubble error if failed to get env": {
			inApp: mockApp,
			inEnv: mockEnv,
			setupMocks: func(m execTaskMocks) {
				m.storeSvc.EXPECT().GetApplication(mockApp).Return(&config.Application{}, nil)
				m.storeSvc.EXPECT().GetEnvironment(mockApp, mockEnv).Return(nil, mockErr)
			},

			wantedError: fmt.Errorf("some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			mocks := execTaskMocks{
				storeSvc: mockStoreReader,
			}

			tc.setupMocks(mocks)

			execTasks := &taskExecOpts{
				taskExecVars: taskExecVars{
					execVars: execVars{
						name:    tc.inTaskGroup,
						appName: tc.inApp,
						envName: tc.inEnv,
					},
					cluster: tc.inCluster,
				},
				store: mockStoreReader,
			}

			// WHEN
			err := execTasks.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

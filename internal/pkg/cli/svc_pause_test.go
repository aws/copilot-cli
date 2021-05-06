// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestSvcPause_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputApp         string
		inputSvc         string
		inputEnvironment string
		mockStoreReader  func(m *mocks.Mockstore)

		wantedError error
	}{
		"invalid app name": {
			inputApp: "my-app",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"invalid service name": {
			inputApp: "my-app",
			inputSvc: "my-svc",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name: "my-app",
				}, nil)
				m.EXPECT().GetService("my-app", "my-svc").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"invalid environment name": {
			inputApp:         "my-app",
			inputEnvironment: "test",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name: "my-app",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "test").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"success": {
			inputApp:         "my-app",
			inputSvc:         "my-svc",
			inputEnvironment: "test",

			mockStoreReader: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name: "my-app",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name: "test",
				}, nil)
				m.EXPECT().GetService("my-app", "my-svc").Return(&config.Workload{
					Name: "my-svc",
				}, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			tc.mockStoreReader(mockStoreReader)

			svcPause := &svcPauseOpts{
				svcPauseVars: svcPauseVars{
					svcName: tc.inputSvc,
					envName: tc.inputEnvironment,
					appName: tc.inputApp,
				},
				store: mockStoreReader,
			}

			// WHEN
			err := svcPause.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSvcPause_Ask(t *testing.T) {
	mockError := errors.New("some error")
	testCases := map[string]struct {
		inputApp         string
		inputSvc         string
		inputEnvironment string
		mockSelector     func(m *mocks.MockdeploySelector)

		wantedError error
	}{
		"errors if failed to select application": {
			mockSelector: func(m *mocks.MockdeploySelector) {
				m.EXPECT().Application(svcPauseAppNamePrompt, svcAppNameHelpPrompt).Return("", mockError)
			},

			wantedError: fmt.Errorf("select application: some error"),
		},
		"errors if failed to select deployed service": {
			inputApp: "mockApp",

			mockSelector: func(m *mocks.MockdeploySelector) {
				m.EXPECT().DeployedService(svcPauseNamePrompt, svcPauseSvcNameHelpPrompt, "mockApp", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, mockError)
			},

			wantedError: fmt.Errorf("select deployed services for application mockApp: some error"),
		},
		"success": {
			inputApp:         "mockApp",
			inputSvc:         "mockSvc",
			inputEnvironment: "mockEnv",

			mockSelector: func(m *mocks.MockdeploySelector) {
				m.EXPECT().DeployedService(svcPauseNamePrompt, svcPauseSvcNameHelpPrompt, "mockApp", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env: "mockEnv",
						Svc: "mockSvc",
					}, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSelector := mocks.NewMockdeploySelector(ctrl)
			tc.mockSelector(mockSelector)

			svcPause := &svcPauseOpts{
				svcPauseVars: svcPauseVars{
					svcName: tc.inputSvc,
					envName: tc.inputEnvironment,
					appName: tc.inputApp,
				},
				sel: mockSelector,
			}

			// WHEN
			err := svcPause.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSvcPause_Execute(t *testing.T) {
	mockError := errors.New("some error")
	testCases := map[string]struct {
		mocking     func(t *testing.T, mockPauser *mocks.MockservicePauser, mockProgress *mocks.Mockprogress)
		wantedError error
	}{
		"errors if failed to pause the service": {
			mocking: func(t *testing.T, mockPauser *mocks.MockservicePauser, mockProgress *mocks.Mockprogress) {
				mockProgress.EXPECT().Start(fmt.Sprintf(fmtSvcPauseStart, "mock-svc"))
				mockPauser.EXPECT().PauseService("mock-svc-arn").Return(mockError)
				mockProgress.EXPECT().Stop(log.Serrorf(fmtsvcPauseFailed, "mock-svc"))
			},
			wantedError: fmt.Errorf("some error"),
		},
		"success": {
			mocking: func(t *testing.T, mockPauser *mocks.MockservicePauser, mockProgress *mocks.Mockprogress) {
				mockProgress.EXPECT().Start(fmt.Sprintf(fmtSvcPauseStart, "mock-svc"))
				mockPauser.EXPECT().PauseService("mock-svc-arn").Return(nil)
				mockProgress.EXPECT().Stop(log.Ssuccessf(fmtSvcPauseSucceed, "mock-svc"))
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstore(ctrl)
			mockServicePauser := mocks.NewMockservicePauser(ctrl)
			mockProgress := mocks.NewMockprogress(ctrl)

			tc.mocking(t, mockServicePauser, mockProgress)

			svcPause := &svcPauseOpts{
				svcPauseVars: svcPauseVars{
					svcName: "mock-svc",
					envName: "mock-env",
					appName: "mock-app",
				},
				svcARN:       "mock-svc-arn",
				store:        mockStore,
				client:       mockServicePauser,
				prog:         mockProgress,
				initSvcPause: func() error { return nil },
			}

			// WHEN
			err := svcPause.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

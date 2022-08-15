// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
)

func TestSvcPause_Validate(t *testing.T) {
	// NOTE: No optional flag to `copilot svc pause` needs to be validated.
}

type svcPauseAskMock struct {
	store  *mocks.Mockstore
	sel    *mocks.MockdeploySelector
	prompt *mocks.Mockprompter
}

func TestSvcPause_Ask(t *testing.T) {
	const (
		inputApp = "my-app"
		inputEnv = "my-env"
		inputSvc = "my-svc"
	)
	mockError := errors.New("some error")
	testCases := map[string]struct {
		inputApp         string
		inputSvc         string
		inputEnvironment string
		skipConfirmation bool

		setupMocks   func(m svcPauseAskMock)
		mockSelector func(m *mocks.MockdeploySelector)
		mockPrompt   func(m *mocks.Mockprompter)

		wantedApp   string
		wantedEnv   string
		wantedSvc   string
		wantedError error
	}{
		"validate app env and svc with all flags passed in": {
			inputApp:         inputApp,
			inputSvc:         inputSvc,
			inputEnvironment: inputEnv,
			skipConfirmation: true,
			setupMocks: func(m svcPauseAskMock) {
				gomock.InOrder(
					m.store.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil),
					m.store.EXPECT().GetEnvironment("my-app", "my-env").Return(&config.Environment{Name: "my-env"}, nil),
					m.store.EXPECT().GetService("my-app", "my-svc").Return(&config.Workload{}, nil),
				)
				m.sel.EXPECT().DeployedService(fmt.Sprintf(svcPauseNamePrompt, inputApp), svcPauseSvcNameHelpPrompt, "my-app", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env:  "my-env",
						Name: "my-svc",
					}, nil) // Let prompter handles the case when svc(env) is definite.
				m.prompt.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedSvc: inputSvc,
		},
		"prompt for app name": {
			inputSvc:         inputSvc,
			inputEnvironment: inputEnv,
			skipConfirmation: true,
			setupMocks: func(m svcPauseAskMock) {
				m.sel.EXPECT().Application(svcPauseAppNamePrompt, wkldAppNameHelpPrompt).Return("my-app", nil)
				m.store.EXPECT().GetApplication(gomock.Any()).Times(0)
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).AnyTimes()
				m.sel.EXPECT().DeployedService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env:  "my-env",
						Name: "my-svc",
					}, nil).AnyTimes()
				m.prompt.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedSvc: inputSvc,
		},
		"errors if failed to select application": {
			skipConfirmation: true,
			setupMocks: func(m svcPauseAskMock) {
				m.sel.EXPECT().Application(svcPauseAppNamePrompt, wkldAppNameHelpPrompt).Return("", errors.New("some error"))
			},
			wantedError: fmt.Errorf("select application: some error"),
		},
		"prompt for svc and env": {
			inputApp:         "my-app",
			skipConfirmation: true,
			setupMocks: func(m svcPauseAskMock) {
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().DeployedService(fmt.Sprintf(svcPauseNamePrompt, inputApp), svcPauseSvcNameHelpPrompt, "my-app", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env:  "my-env",
						Name: "my-svc",
					}, nil)
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedSvc: inputSvc,
		},
		"errors if failed to select deployed service": {
			inputApp:         inputApp,
			skipConfirmation: true,
			setupMocks: func(m svcPauseAskMock) {
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().DeployedService(fmt.Sprintf(svcPauseNamePrompt, inputApp), svcPauseSvcNameHelpPrompt, inputApp, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("select deployed services for application my-app: some error"),
		},
		"should wrap error returned from prompter confirmation": {
			inputApp:         "mockApp",
			inputSvc:         "mockSvc",
			inputEnvironment: "mockEnv",
			skipConfirmation: false,
			setupMocks: func(m svcPauseAskMock) {
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).AnyTimes()
				m.sel.EXPECT().DeployedService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env:  "my-env",
						Name: "my-svc",
					}, nil)
				m.prompt.EXPECT().Confirm("Are you sure you want to stop processing requests for service my-svc?", "", gomock.Any()).
					Times(1).Return(true, mockError)
			},
			wantedError: fmt.Errorf("svc pause confirmation prompt: %w", mockError),
		},
		"should return error if user doesn't confirm svc pause": {
			inputApp:         "mockApp",
			inputSvc:         "mockSvc",
			inputEnvironment: "mockEnv",
			skipConfirmation: false,
			setupMocks: func(m svcPauseAskMock) {
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).AnyTimes()
				m.sel.EXPECT().DeployedService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env:  "my-env",
						Name: "my-svc",
					}, nil)
				m.prompt.EXPECT().Confirm("Are you sure you want to stop processing requests for service my-svc?", "", gomock.Any()).
					Times(1).Return(false, nil)
			},
			wantedError: errors.New("svc pause cancelled - no changes made"),
		},
		"user confirms svc pause": {
			inputApp:         inputApp,
			inputSvc:         inputSvc,
			inputEnvironment: inputEnv,
			skipConfirmation: false,
			setupMocks: func(m svcPauseAskMock) {
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).AnyTimes()
				m.sel.EXPECT().DeployedService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env:  "my-env",
						Name: "my-svc",
					}, nil)
				m.prompt.EXPECT().Confirm("Are you sure you want to stop processing requests for service my-svc?", "", gomock.Any()).
					Times(1).Return(true, nil)
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedSvc: inputSvc,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := svcPauseAskMock{
				store:  mocks.NewMockstore(ctrl),
				sel:    mocks.NewMockdeploySelector(ctrl),
				prompt: mocks.NewMockprompter(ctrl),
			}

			tc.setupMocks(m)
			svcPause := &svcPauseOpts{
				svcPauseVars: svcPauseVars{
					skipConfirmation: tc.skipConfirmation,
					svcName:          tc.inputSvc,
					envName:          tc.inputEnvironment,
					appName:          tc.inputApp,
				},
				sel:    m.sel,
				prompt: m.prompt,
				store:  m.store,
			}

			// WHEN
			err := svcPause.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedApp, svcPause.appName, "expected app name to match")
				require.Equal(t, tc.wantedSvc, svcPause.svcName, "expected service name to match")
				require.Equal(t, tc.wantedEnv, svcPause.envName, "expected service name to match")
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
				mockProgress.EXPECT().Start("Pausing service mock-svc in environment mock-env.")
				mockPauser.EXPECT().PauseService("mock-svc-arn").Return(mockError)
				mockProgress.EXPECT().Stop(log.Serrorf("Failed to pause service mock-svc in environment mock-env.\n"))
			},
			wantedError: fmt.Errorf("some error"),
		},
		"success": {
			mocking: func(t *testing.T, mockPauser *mocks.MockservicePauser, mockProgress *mocks.Mockprogress) {
				mockProgress.EXPECT().Start("Pausing service mock-svc in environment mock-env.")
				mockPauser.EXPECT().PauseService("mock-svc-arn").Return(nil)
				mockProgress.EXPECT().Stop(log.Ssuccessf("Paused service mock-svc in environment mock-env.\n"))
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

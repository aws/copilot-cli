// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
)

func TestSvcStatus_Validate(t *testing.T) {
	// NOTE: No optional flag to `copilot svc pause` needs to be validated.
}

type svcStatusAskMock struct {
	store *mocks.Mockstore
	sel   *mocks.MockdeploySelector
}

func TestSvcStatus_Ask(t *testing.T) {
	const (
		testAppName = "phonetool"
		testEnvName = "test"
		testSvcName = "api"
	)
	mockError := errors.New("some error")
	testCases := map[string]struct {
		inputApp string
		inputSvc string
		inputEnv string

		setupMocks func(m svcStatusAskMock)

		wantedApp   string
		wantedEnv   string
		wantedSvc   string
		wantedError error
	}{
		"validate app env and svc with all flags passed in": {
			inputApp: testAppName,
			inputSvc: testSvcName,
			inputEnv: testEnvName,
			setupMocks: func(m svcStatusAskMock) {
				gomock.InOrder(
					m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil),
					m.store.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{Name: "test"}, nil),
					m.store.EXPECT().GetService("phonetool", "api").Return(&config.Workload{}, nil),
				)
				m.sel.EXPECT().DeployedService(svcStatusNamePrompt, svcStatusNameHelpPrompt, "phonetool", gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env:  "test",
						Name: "api",
					}, nil) // Let prompter handles the case when svc(env) is definite.
			},
			wantedApp: testAppName,
			wantedEnv: testEnvName,
			wantedSvc: testSvcName,
		},
		"prompt for app name": {
			inputEnv: testEnvName,
			inputSvc: testSvcName,
			setupMocks: func(m svcStatusAskMock) {
				m.sel.EXPECT().Application(svcAppNamePrompt, wkldAppNameHelpPrompt).Return("phonetool", nil)
				m.store.EXPECT().GetApplication(gomock.Any()).Times(0)
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).AnyTimes()
				m.sel.EXPECT().DeployedService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env:  testEnvName,
						Name: testSvcName,
					}, nil).AnyTimes()
			},
			wantedApp: testAppName,
			wantedEnv: testEnvName,
			wantedSvc: testSvcName,
		},
		"errors if failed to select application": {
			setupMocks: func(m svcStatusAskMock) {
				m.sel.EXPECT().Application(svcAppNamePrompt, wkldAppNameHelpPrompt).Return("", errors.New("some error"))
			},
			wantedError: fmt.Errorf("select application: some error"),
		},
		"prompt for service and env": {
			inputApp: testAppName,
			setupMocks: func(m svcStatusAskMock) {
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().DeployedService(svcStatusNamePrompt, svcStatusNameHelpPrompt, testAppName, gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env:  testEnvName,
						Name: testSvcName,
					}, nil)
			},
			wantedApp: testAppName,
			wantedEnv: testEnvName,
			wantedSvc: testSvcName,
		},
		"errors if failed to select deployed service": {
			inputApp: "mockApp",
			setupMocks: func(m svcStatusAskMock) {
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().DeployedService(svcStatusNamePrompt, svcStatusNameHelpPrompt, "mockApp", gomock.Any(), gomock.Any()).Return(nil, mockError)
			},

			wantedError: fmt.Errorf("select deployed services for application mockApp: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := svcStatusAskMock{
				store: mocks.NewMockstore(ctrl),
				sel:   mocks.NewMockdeploySelector(ctrl),
			}
			tc.setupMocks(m)
			svcStatus := &svcStatusOpts{
				svcStatusVars: svcStatusVars{
					svcName: tc.inputSvc,
					envName: tc.inputEnv,
					appName: tc.inputApp,
				},
				sel:   m.sel,
				store: m.store,
			}

			// WHEN
			err := svcStatus.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedApp, svcStatus.appName, "expected app name to match")
				require.Equal(t, tc.wantedSvc, svcStatus.svcName, "expected service name to match")
				require.Equal(t, tc.wantedEnv, svcStatus.envName, "expected service name to match")
			}
		})
	}
}

func TestSvcStatus_Execute(t *testing.T) {
	mockError := errors.New("some error")
	testCases := map[string]struct {
		shouldOutputJSON    bool
		mockStatusDescriber func(m *mocks.MockstatusDescriber)
		wantedError         error
	}{
		"errors if failed to describe the status of the service": {
			mockStatusDescriber: func(m *mocks.MockstatusDescriber) {
				m.EXPECT().Describe().Return(nil, mockError)
			},
			wantedError: fmt.Errorf("describe status of service mockSvc: some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockStatusDescriber := mocks.NewMockstatusDescriber(ctrl)
			tc.mockStatusDescriber(mockStatusDescriber)

			svcStatus := &svcStatusOpts{
				svcStatusVars: svcStatusVars{
					svcName:          "mockSvc",
					envName:          "mockEnv",
					shouldOutputJSON: tc.shouldOutputJSON,
					appName:          "mockApp",
				},
				statusDescriber:     mockStatusDescriber,
				initStatusDescriber: func(*svcStatusOpts) error { return nil },
				w:                   b,
			}

			// WHEN
			err := svcStatus.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, b.String(), "expected output content to not be empty")
			}
		})
	}
}

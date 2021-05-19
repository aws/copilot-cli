// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestSvcStatus_Validate(t *testing.T) {
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

			svcStatus := &svcStatusOpts{
				svcStatusVars: svcStatusVars{
					svcName: tc.inputSvc,
					envName: tc.inputEnvironment,
					appName: tc.inputApp,
				},
				store: mockStoreReader,
			}

			// WHEN
			err := svcStatus.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSvcStatus_Ask(t *testing.T) {
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
				m.EXPECT().Application(svcAppNamePrompt, svcAppNameHelpPrompt).Return("", mockError)
			},

			wantedError: fmt.Errorf("select application: some error"),
		},
		"errors if failed to select deployed service": {
			inputApp: "mockApp",

			mockSelector: func(m *mocks.MockdeploySelector) {
				m.EXPECT().DeployedService(svcStatusNamePrompt, svcStatusNameHelpPrompt, "mockApp", gomock.Any(), gomock.Any()).
					Return(nil, mockError)
			},

			wantedError: fmt.Errorf("select deployed services for application mockApp: some error"),
		},
		"success": {
			inputApp:         "mockApp",
			inputSvc:         "mockSvc",
			inputEnvironment: "mockEnv",

			mockSelector: func(m *mocks.MockdeploySelector) {
				m.EXPECT().DeployedService(svcStatusNamePrompt, svcStatusNameHelpPrompt, "mockApp", gomock.Any(), gomock.Any()).
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

			svcStatus := &svcStatusOpts{
				svcStatusVars: svcStatusVars{
					svcName: tc.inputSvc,
					envName: tc.inputEnvironment,
					appName: tc.inputApp,
				},
				sel: mockSelector,
			}

			// WHEN
			err := svcStatus.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
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

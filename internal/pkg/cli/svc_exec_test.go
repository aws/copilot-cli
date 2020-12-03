// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type execSvcMocks struct {
	storeSvc *mocks.Mockstore
	sel      *mocks.MockdeploySelector
}

func TestSvcExec_Validate(t *testing.T) {
	const (
		inputApp = "my-app"
		inputEnv = "my-env"
		inputSvc = "my-svc"
	)
	testCases := map[string]struct {
		setupMocks func(mocks execSvcMocks)

		wantedError error
	}{
		"valid app name and service name": {
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
						Name: "my-app",
					}, nil),
					m.storeSvc.EXPECT().GetEnvironment("my-app", "my-env").Return(&config.Environment{
						Name: "my-env",
					}, nil),
					m.storeSvc.EXPECT().GetService("my-app", "my-svc").Return(&config.Workload{
						Name: "my-svc",
					}, nil),
				)
			},

			wantedError: nil,
		},
		"should bubble error if cannot get application configuration": {
			setupMocks: func(m execSvcMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"should bubble error if cannot get environment configuration": {
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
						Name: "my-app",
					}, nil),
					m.storeSvc.EXPECT().GetEnvironment("my-app", "my-env").Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"should bubble error if cannot get service configuration": {
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
						Name: "my-app",
					}, nil),
					m.storeSvc.EXPECT().GetEnvironment("my-app", "my-env").Return(&config.Environment{
						Name: "my-env",
					}, nil),
					m.storeSvc.EXPECT().GetService("my-app", "my-svc").Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)

			mocks := execSvcMocks{
				storeSvc: mockStoreReader,
			}

			tc.setupMocks(mocks)

			execSvcs := &svcExecOpts{
				execVars: execVars{
					name:    inputSvc,
					appName: inputApp,
					envName: inputEnv,
				},
				store: mockStoreReader,
			}

			// WHEN
			err := execSvcs.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSvcExec_Ask(t *testing.T) {
	const (
		inputApp = "my-app"
		inputEnv = "my-env"
		inputSvc = "my-svc"
	)
	testCases := map[string]struct {
		inputApp string
		inputEnv string
		inputSvc string

		setupMocks func(mocks execSvcMocks)

		wantedApp   string
		wantedEnv   string
		wantedSvc   string
		wantedError error
	}{
		"with all flags": {
			inputApp: inputApp,
			inputEnv: inputEnv,
			inputSvc: inputSvc,
			setupMocks: func(m execSvcMocks) {
				m.sel.EXPECT().DeployedService(svcExecNamePrompt, svcExecNameHelpPrompt, "my-app", gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env: "my-env",
						Svc: "my-svc",
					}, nil)
			},

			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedSvc: inputSvc,
		},
		"success": {
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.sel.EXPECT().Application(svcAppNamePrompt, svcAppNameHelpPrompt).Return("my-app", nil),
					m.sel.EXPECT().DeployedService(svcExecNamePrompt, svcExecNameHelpPrompt, "my-app", gomock.Any(), gomock.Any()).
						Return(&selector.DeployedService{
							Env: "my-env",
							Svc: "my-svc",
						}, nil),
				)
			},

			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedSvc: inputSvc,
		},
		"returns error when fail to select apps": {
			setupMocks: func(m execSvcMocks) {
				m.sel.EXPECT().Application(svcAppNamePrompt, svcAppNameHelpPrompt).Return("", errors.New("some error"))
			},

			wantedError: fmt.Errorf("select application: some error"),
		},
		"returns error when fail to select services": {
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.sel.EXPECT().Application(svcAppNamePrompt, svcAppNameHelpPrompt).Return("my-app", nil),
					m.sel.EXPECT().DeployedService(svcExecNamePrompt, svcExecNameHelpPrompt, "my-app", gomock.Any(), gomock.Any()).
						Return(nil, fmt.Errorf("some error")),
				)
			},

			wantedError: fmt.Errorf("select deployed service for application my-app: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			mockSelector := mocks.NewMockdeploySelector(ctrl)

			mocks := execSvcMocks{
				storeSvc: mockStoreReader,
				sel:      mockSelector,
			}

			tc.setupMocks(mocks)

			execSvcs := &svcExecOpts{
				execVars: execVars{
					name:    tc.inputSvc,
					envName: tc.inputEnv,
					appName: tc.inputApp,
				},
				store: mockStoreReader,
				sel:   mockSelector,
			}

			// WHEN
			err := execSvcs.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedApp, execSvcs.appName, "expected app name to match")
				require.Equal(t, tc.wantedSvc, execSvcs.name, "expected service name to match")
				require.Equal(t, tc.wantedEnv, execSvcs.envName, "expected service name to match")
			}
		})
	}
}

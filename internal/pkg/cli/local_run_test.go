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

var testError = errors.New("some error")

type localRunAskMocks struct {
	store       *mocks.Mockstore
	ws          *mocks.MockwsWlDirReader
	prompt      *mocks.Mockprompter
	deployStore *mocks.MockdeployedEnvironmentLister
}

func TestLocalRunOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputAppName  string
		setupMocks    func(m *localRunAskMocks)
		wantedAppName string
		wantedError   error
	}{
		"no app in workspace": {
			wantedError: errNoAppInWorkspace,
		},
		"fail to read the application from SSM store": {
			inputAppName: "testApp",
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetApplication("testApp").Return(nil, testError)
			},
			wantedError: fmt.Errorf("get application testApp: %w", testError),
		},
		"successful validation": {
			inputAppName: "testApp",
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetApplication("testApp").Return(&config.Application{Name: "testApp"}, nil)
			},
			wantedAppName: "testApp",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &localRunAskMocks{
				store: mocks.NewMockstore(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(m)
			}
			opts := localRunOpts{
				localRunVars: localRunVars{
					appName: tc.inputAppName,
				},
				store: m.store,
			}
			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLocalRunOpts_Ask(t *testing.T) {
	const (
		testAppName  = "testApp"
		testEnvName  = "testEnv"
		testWkldName = "testWkld"
	)

	testCases := map[string]struct {
		inputAppName  string
		inputEnvName  string
		inputWkldName string

		setupMocks     func(m *localRunAskMocks)
		mockEnvChecker func(ctrl *gomock.Controller) versionCompatibilityChecker
		wantedWkldName string
		wantedEnvName  string
		wantedError    error
	}{
		"error while getting all the workloads in the workspace": {
			inputAppName: testAppName,
			setupMocks: func(m *localRunAskMocks) {
				m.ws.EXPECT().ListWorkloads().Return([]string{}, testError)
			},
			wantedError: fmt.Errorf("list workloads in the workspace %s: %w", testAppName, testError),
		},
		"error while returning list of environments a workload is deployed in": {
			inputAppName: testAppName,
			setupMocks: func(m *localRunAskMocks) {
				m.ws.EXPECT().ListWorkloads().Return([]string{"testWkld"}, nil)
				m.deployStore.EXPECT().ListEnvironmentsDeployedTo(testAppName, testWkldName).Return(nil, testError)
			},
			wantedError: fmt.Errorf("list deployed environments for application %s: %w", testAppName, testError),
		},
		"return error if provided workload not exists in workspace": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			setupMocks: func(m *localRunAskMocks) {
				m.ws.EXPECT().ListWorkloads().Return([]string{"testSvc"}, nil)
			},
			wantedError: fmt.Errorf("workload %q does not exist in the workspace", testWkldName),
		},
		"return error if provided workload not exists in ssm": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			setupMocks: func(m *localRunAskMocks) {
				m.ws.EXPECT().ListWorkloads().Return([]string{"testWkld"}, nil)
				m.store.EXPECT().GetWorkload(testAppName, testWkldName).Return(nil, testError)
			},
			wantedError: fmt.Errorf("retrieve %s from application %s: %w", testWkldName, testAppName, testError),
		},
		"return error if provided workload is not deployed": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			setupMocks: func(m *localRunAskMocks) {
				m.ws.EXPECT().ListWorkloads().Return([]string{"testWkld"}, nil)
				m.store.EXPECT().GetWorkload(testAppName, testWkldName).Return(&config.Workload{Name: "testWkld"}, nil)
				m.deployStore.EXPECT().ListEnvironmentsDeployedTo("testApp", "testWkld").Return([]string{}, nil)
			},
			wantedError: fmt.Errorf("workload %q is not deployed in any environment", testWkldName),
		},
		"successfully select environment if only workload name is given as flag": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			setupMocks: func(m *localRunAskMocks) {
				m.ws.EXPECT().ListWorkloads().Return([]string{"testWkld"}, nil)
				m.store.EXPECT().GetWorkload(testAppName, testWkldName).Return(&config.Workload{
					Name: "testWkld",
				}, nil)
				m.deployStore.EXPECT().ListEnvironmentsDeployedTo("testApp", "testWkld").Return([]string{"testEnv", "mockEnv"}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("testEnv", nil)
			},
			wantedWkldName: testWkldName,
			wantedEnvName:  testEnvName,
		},
		"return error while selecting environment": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			setupMocks: func(m *localRunAskMocks) {
				m.ws.EXPECT().ListWorkloads().Return([]string{"testWkld"}, nil)
				m.store.EXPECT().GetWorkload(testAppName, testWkldName).Return(&config.Workload{
					Name: "testWkld",
				}, nil)
				m.deployStore.EXPECT().ListEnvironmentsDeployedTo(testAppName, testWkldName).Return([]string{"testEnv", "mockEnv"}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", testError)
			},
			wantedError: fmt.Errorf("select environment: %w", testError),
		},
		"return error while selecting workload": {
			inputAppName: testAppName,
			setupMocks: func(m *localRunAskMocks) {
				m.ws.EXPECT().ListWorkloads().Return([]string{"testWkld", "testSvc"}, nil)
				m.deployStore.EXPECT().ListEnvironmentsDeployedTo(testAppName, testWkldName).Return([]string{"testEnv"}, nil)
				m.deployStore.EXPECT().ListEnvironmentsDeployedTo(testAppName, "testSvc").Return([]string{"mockEnv"}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", testError)

			},
			wantedError: fmt.Errorf("select a Workload: %w", testError),
		},
		"validate if flags are provided": {
			inputAppName:  testAppName,
			inputEnvName:  testEnvName,
			inputWkldName: testWkldName,
			setupMocks: func(m *localRunAskMocks) {
				m.ws.EXPECT().ListWorkloads().Return([]string{"testWkld"}, nil)
				m.deployStore.EXPECT().ListEnvironmentsDeployedTo(testAppName, testWkldName).Return([]string{"testEnv"}, nil).Times(2)
				m.store.EXPECT().GetWorkload("testApp", "testWkld").Return(&config.Workload{
					Name: "testWkld",
				}, nil)
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("v1.12.1", nil)
				return m
			},
			wantedWkldName: testWkldName,
			wantedEnvName:  testEnvName,
		},
		"return error if no workload is deployed in app": {
			inputAppName: testAppName,
			setupMocks: func(m *localRunAskMocks) {
				m.ws.EXPECT().ListWorkloads().Return([]string{"testWkld", "testSvc"}, nil)
				m.deployStore.EXPECT().ListEnvironmentsDeployedTo(testAppName, testWkldName).Return([]string{}, nil)
				m.deployStore.EXPECT().ListEnvironmentsDeployedTo(testAppName, "testSvc").Return([]string{}, nil)
			},
			wantedError: fmt.Errorf("no workload is deployed in app %s", testAppName),
		},
		"defaults to environment and workload if only one of them is present": {
			inputAppName: testAppName,
			setupMocks: func(m *localRunAskMocks) {
				m.ws.EXPECT().ListWorkloads().Return([]string{"testWkld"}, nil)
				m.deployStore.EXPECT().ListEnvironmentsDeployedTo(testAppName, testWkldName).Return([]string{"testEnv"}, nil)
			},
			wantedWkldName: testWkldName,
			wantedEnvName:  testEnvName,
		},
		"successfully select workload if only environment name is given as flag": {
			inputAppName: testAppName,
			inputEnvName: testEnvName,
			setupMocks: func(m *localRunAskMocks) {
				m.deployStore.EXPECT().ListEnvironmentsDeployedTo(testAppName, testWkldName).Return([]string{"testEnv"}, nil).Times(2)
				m.ws.EXPECT().ListWorkloads().Return([]string{"testWkld"}, nil)
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("v1.12.1", nil)
				return m
			},
			wantedWkldName: testWkldName,
			wantedEnvName:  testEnvName,
		},
		"return error if provided environment is not deployed": {
			inputAppName: testAppName,
			inputEnvName: testEnvName,
			setupMocks: func(m *localRunAskMocks) {
				m.deployStore.EXPECT().ListEnvironmentsDeployedTo(testAppName, testWkldName).Return([]string{"testEnv"}, nil).Times(2)
				m.ws.EXPECT().ListWorkloads().Return([]string{"testWkld"}, nil)
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("bootstrap", nil)
				return m
			},
			wantedError: fmt.Errorf(`cannot use an environment which is not deployed Please run "copilot env deploy, --name %s" to deploy the environment first`, testEnvName),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			m := &localRunAskMocks{
				store:       mocks.NewMockstore(ctrl),
				ws:          mocks.NewMockwsWlDirReader(ctrl),
				prompt:      mocks.NewMockprompter(ctrl),
				deployStore: mocks.NewMockdeployedEnvironmentLister(ctrl),
			}
			tc.setupMocks(m)
			opts := localRunOpts{
				localRunVars: localRunVars{
					appName:  tc.inputAppName,
					wkldName: tc.inputWkldName,
					envName:  tc.inputEnvName,
				},
				store:       m.store,
				ws:          m.ws,
				prompt:      m.prompt,
				deployStore: m.deployStore,
				envVersionGetter: func(string) (versionGetter, error) {
					return tc.mockEnvChecker(ctrl), nil
				},
				wkldDeployedToEnvs: make(map[string][]string),
				deployedWkld:       make([]string, 0),
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedWkldName, opts.wkldName)
				require.Equal(t, tc.wantedEnvName, opts.envName)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

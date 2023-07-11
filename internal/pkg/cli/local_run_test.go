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
		wantedError error
	}{
		"no app in workspace": {
			wantedError: errNoAppInWorkspace,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := localRunOpts{}

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
		region       = "us-west-2"
		accountID    = "123456789012"
	)

	mockEnv := &config.Environment{
		Name:             testEnvName,
		App:              testAppName,
		Region:           region,
		ExecutionRoleARN: "ARN",
		AccountID:        accountID,
	}
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
		"error while getting environemnt belonging to an application by name": {
			inputAppName: testAppName,
			inputEnvName: testEnvName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetEnvironment("testApp", "testEnv").Return(nil, testError)
			},
			wantedError: fmt.Errorf("get environment %s: %w", testEnvName, testError),
		},
		"error while returning all environments belonging to a application": {
			inputAppName: testAppName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().ListEnvironments(testAppName).Return(nil, testError)
			},
			wantedError: fmt.Errorf("get environments for the app %s: %w", testAppName, testError),
		},
		"return error if no deployed environemnts found": {
			inputAppName: testAppName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().ListEnvironments(testAppName).Return([]*config.Environment{}, nil)
			},
			wantedError: fmt.Errorf("no deployed environments found in the app %s", testAppName),
		},
		"return error while selecting environment": {
			inputAppName: testAppName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().ListEnvironments(testAppName).Return([]*config.Environment{{
					Name:      "testEnv",
					Region:    region,
					AccountID: accountID,
				}, {
					Name:      "mockEnv",
					Region:    region,
					AccountID: "123456712",
				}}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", testError)
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("v1.12.1", nil)
				return m
			},
			wantedError: fmt.Errorf("select environment: %w", testError),
		},
		"return error while selecting workload": {
			inputAppName: testAppName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().ListEnvironments(testAppName).Return([]*config.Environment{{
					Name:      "testEnv",
					Region:    region,
					AccountID: accountID,
				}, {
					Name:      "mockEnv",
					Region:    region,
					AccountID: "123456712",
				}}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("testEnv", nil)
				m.deployStore.EXPECT().ListDeployedServices(testAppName, testEnvName).Return([]string{"testWkld"}, nil)
				m.deployStore.EXPECT().ListDeployedJobs(testAppName, testEnvName).Return([]string{"testJob"}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", testError)
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("v1.12.1", nil)
				return m
			},
			wantedError: fmt.Errorf("select Workload: %w", testError),
		},
		"return error if provided environment is not deployed": {
			inputAppName: testAppName,
			inputEnvName: testEnvName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetEnvironment("testApp", "testEnv").Return(mockEnv, nil)
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("bootstrap", nil)
				return m
			},
			wantedError: fmt.Errorf(`cannot use an environment which is not deployed Please run "copilot env deploy, --name %s" to deploy the environment first`, testEnvName),
		},
		"return error if no workloads are found in the environment": {
			inputAppName: testAppName,
			inputEnvName: testEnvName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetEnvironment("testApp", "testEnv").Return(mockEnv, nil)
				m.deployStore.EXPECT().ListDeployedServices(testAppName, testEnvName).Return([]string{}, nil)
				m.deployStore.EXPECT().ListDeployedJobs(testAppName, testEnvName).Return([]string{}, nil)
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("v1.12.1", nil)
				return m
			},
			wantedError: fmt.Errorf("no workloads found in this environment %s", testEnvName),
		},
		"return error while getting the services": {
			inputAppName: testAppName,
			inputEnvName: testEnvName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetEnvironment("testApp", "testEnv").Return(mockEnv, nil)
				m.deployStore.EXPECT().ListDeployedServices(testAppName, testEnvName).Return(nil, testError)
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("v1.12.1", nil)
				return m
			},
			wantedError: fmt.Errorf("Get services: %w", testError),
		},
		"return error while getting the jobs": {
			inputAppName: testAppName,
			inputEnvName: testEnvName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetEnvironment("testApp", "testEnv").Return(mockEnv, nil)
				m.deployStore.EXPECT().ListDeployedServices(testAppName, testEnvName).Return([]string{"testWkld"}, nil)
				m.deployStore.EXPECT().ListDeployedJobs(testAppName, testEnvName).Return(nil, testError)
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("v1.12.1", nil)
				return m
			},
			wantedError: fmt.Errorf("Get jobs: %w", testError),
		},
		"validate if flags are provided": {
			inputAppName:  testAppName,
			inputEnvName:  testEnvName,
			inputWkldName: testWkldName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetEnvironment("testApp", "testEnv").Return(mockEnv, nil)
				m.ws.EXPECT().ListWorkloads().Return([]string{"testWkld"}, nil)
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
		"defaults to environment and workload if only one of them is present": {
			inputAppName: testAppName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().ListEnvironments(testAppName).Return([]*config.Environment{{
					Name:      "testEnv",
					Region:    region,
					AccountID: accountID,
				}}, nil)
				m.deployStore.EXPECT().ListDeployedServices(testAppName, testEnvName).Return([]string{"testWkld"}, nil)
				m.deployStore.EXPECT().ListDeployedJobs(testAppName, testEnvName).Return([]string{}, nil)
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("v1.12.1", nil)
				return m
			},
			wantedWkldName: testWkldName,
			wantedEnvName:  testEnvName,
		},
		"prompts if more than one environment is present": {
			inputAppName: testAppName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().ListEnvironments(testAppName).Return([]*config.Environment{{
					Name:      "testEnv",
					Region:    region,
					AccountID: accountID,
				}, {
					Name:      "mockEnv",
					Region:    region,
					AccountID: "123456712",
				}}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("testEnv", nil)
				m.deployStore.EXPECT().ListDeployedServices(testAppName, testEnvName).Return([]string{"testWkld"}, nil)
				m.deployStore.EXPECT().ListDeployedJobs(testAppName, testEnvName).Return([]string{"testJob"}, nil)
				m.prompt.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("testWkld", nil)
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("v1.12.1", nil)
				return m
			},
			wantedWkldName: testWkldName,
			wantedEnvName:  testEnvName,
		},
		"if only environment name is given as flag": {
			inputAppName: testAppName,
			inputEnvName: testEnvName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().GetEnvironment("testApp", "testEnv").Return(mockEnv, nil)
				m.deployStore.EXPECT().ListDeployedServices(testAppName, testEnvName).Return([]string{"testWkld"}, nil)
				m.deployStore.EXPECT().ListDeployedJobs(testAppName, testEnvName).Return([]string{}, nil)

			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("v1.12.1", nil)
				return m
			},
			wantedWkldName: testWkldName,
			wantedEnvName:  testEnvName,
		},
		"if only workload name is given as flag": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().ListEnvironments(testAppName).Return([]*config.Environment{{
					Name:      "testEnv",
					Region:    region,
					AccountID: accountID,
				}}, nil)
				m.ws.EXPECT().ListWorkloads().Return([]string{"testWkld"}, nil)
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
		"should return a wrapped error when environment version cannot be retrieved": {
			inputAppName:  testAppName,
			inputWkldName: testWkldName,
			setupMocks: func(m *localRunAskMocks) {
				m.store.EXPECT().ListEnvironments(testAppName).Return([]*config.Environment{{
					Name:      "testEnv",
					Region:    region,
					AccountID: accountID,
				}}, nil)
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("", errors.New("some error"))
				return m
			},
			wantedError: fmt.Errorf("get environment %q version: %w", testEnvName, testError),
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
				envFeaturesDescriber: func(string) (versionCompatibilityChecker, error) {
					return tc.mockEnvChecker(ctrl), nil
				},
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

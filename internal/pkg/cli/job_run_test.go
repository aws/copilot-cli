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

type jobRunMock struct {
	configStore *mocks.Mockstore
	sel         *mocks.MockconfigSelector
}

func TestJobRun_Ask(t *testing.T) {
	const (
		inputApp = "my-app"
		inputEnv = "my-env"
		inputJob = "my-job"
	)

	testCases := map[string]struct {
		inputApp     string
		inputJob     string
		inputEnvName string

		setupMocks func(mocks jobRunMock)

		wantedApp   string
		wantedEnv   string
		wantedJob   string
		wantedError error
	}{
		"validate app env and job with all flags passed in": {
			inputApp:     inputApp,
			inputJob:     inputJob,
			inputEnvName: inputEnv,
			setupMocks: func(m jobRunMock) {
				gomock.InOrder(
					m.configStore.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil),
					m.configStore.EXPECT().GetJob("my-app", "my-job").Return(&config.Workload{}, nil),
					m.configStore.EXPECT().GetEnvironment("my-app", "my-env").Return(&config.Environment{Name: "my-env"}, nil),
				)
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedJob: inputJob,
		},
		"prompt for app name": {
			inputJob:     inputJob,
			inputEnvName: inputEnv,
			setupMocks: func(m jobRunMock) {
				gomock.InOrder(
					m.sel.EXPECT().Application(jobAppNamePrompt, wkldAppNameHelpPrompt).Return("my-app", nil),
					m.configStore.EXPECT().GetApplication(gomock.Any()).Times(0),
					m.configStore.EXPECT().GetJob(gomock.Any(), gomock.Any()).AnyTimes(),
					m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes(),
					m.sel.EXPECT().Job(gomock.Any(), gomock.Any(), gomock.Any()).Return("my-job", nil).AnyTimes(),
					m.sel.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Return("my-env", nil).AnyTimes(),
				)
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedJob: inputJob,
		},
		"returns error if fail to select app": {
			setupMocks: func(m jobRunMock) {
				gomock.InOrder(
					m.sel.EXPECT().Application(jobAppNamePrompt, wkldAppNameHelpPrompt).Return("", errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("select application: some error"),
		},
		"prompt for job and env": {
			inputApp: inputApp,
			setupMocks: func(m jobRunMock) {
				gomock.InOrder(
					m.configStore.EXPECT().GetApplication(gomock.Any()).AnyTimes(),
					m.configStore.EXPECT().GetJob(gomock.Any(), gomock.Any()).Times(0),
					m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0),
					m.sel.EXPECT().Job("Which job would you like to invoke?", "", "my-app").Return("my-job", nil),
					m.sel.EXPECT().Environment("Which environment?", "", "my-app").Return("my-env", nil),
				)
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedJob: inputJob,
		},
		"return error if fail to select environment": {
			inputApp: inputApp,
			inputJob: inputJob,
			setupMocks: func(m jobRunMock) {
				gomock.InOrder(
					m.configStore.EXPECT().GetApplication(gomock.Any()).AnyTimes(),
					m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0),
					m.configStore.EXPECT().GetJob(gomock.Any(), gomock.Any()).AnyTimes(),
					m.sel.EXPECT().Environment("Which environment?", "", "my-app").Return("", errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("select environment: some error"),
		},
		"return error if fail to select job": {
			inputApp:     inputApp,
			inputEnvName: inputEnv,
			setupMocks: func(m jobRunMock) {
				gomock.InOrder(
					m.configStore.EXPECT().GetApplication(gomock.Any()).AnyTimes(),
					m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes(),
					m.configStore.EXPECT().GetJob(gomock.Any(), gomock.Any()).Times(0),
					m.sel.EXPECT().Job("Which job would you like to invoke?", "", "my-app").Return("", errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("select job: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstore(ctrl)
			mockSel := mocks.NewMockconfigSelector(ctrl)

			tc.setupMocks(jobRunMock{
				configStore: mockStore,
				sel:         mockSel,
			})
			jobRun := &jobRunOpts{
				jobRunVars: jobRunVars{
					envName: tc.inputEnvName,
					appName: tc.inputApp,
					jobName: tc.inputJob,
				},
				configStore: mockStore,
				sel:         mockSel,
			}

			err := jobRun.Ask()

			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedApp, jobRun.appName, "expected app name to match")
				require.Equal(t, tc.wantedJob, jobRun.jobName, "expected job name to match")
				require.Equal(t, tc.wantedEnv, jobRun.envName, "expected environment name to match")
			}
		})
	}
}

func TestJobRun_Execute(t *testing.T) {
	testCases := map[string]struct {
		appName        string
		envName        string
		jobName        string
		mockjobRunner  func(ctrl *gomock.Controller) runner
		mockEnvChecker func(ctrl *gomock.Controller) versionCompatibilityChecker
		wantedError    error
	}{
		"successfully invoke job": {
			jobName: "mockJob",
			mockjobRunner: func(ctrl *gomock.Controller) runner {
				m := mocks.NewMockrunner(ctrl)
				m.EXPECT().Run().Return(nil)
				return m
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("v1.12.1", nil)
				return m
			},
		},
		"should return a wrapped error when state machine cannot be run": {
			jobName: "mockJob",
			mockjobRunner: func(ctrl *gomock.Controller) runner {
				m := mocks.NewMockrunner(ctrl)
				m.EXPECT().Run().Return(errors.New("some error"))
				return m
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("v1.12.0", nil)
				return m
			},
			wantedError: fmt.Errorf(`execute job "mockJob": some error`),
		},
		"should return a wrapped error when environment version cannot be retrieved": {
			appName: "finance",
			envName: "test",
			jobName: "report",
			mockjobRunner: func(ctrl *gomock.Controller) runner {
				return nil
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("", errors.New("some error"))
				return m
			},
			wantedError: errors.New(`retrieve version of environment stack "test" in application "finance": some error`),
		},
		"should return an error when environment template version is below v1.12.0": {
			appName: "finance",
			envName: "test",
			jobName: "report",
			mockjobRunner: func(ctrl *gomock.Controller) runner {
				return nil
			},
			mockEnvChecker: func(ctrl *gomock.Controller) versionCompatibilityChecker {
				m := mocks.NewMockversionCompatibilityChecker(ctrl)
				m.EXPECT().Version().Return("v1.11.0", nil)
				return m
			},
			wantedError: errors.New(`environment "test" is on version "v1.11.0" which does not support the "job run" feature`),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			jobRunOpts := &jobRunOpts{
				jobRunVars: jobRunVars{
					appName: tc.appName,
					envName: tc.envName,
					jobName: tc.jobName,
				},
				newRunner: func() (runner, error) {
					return tc.mockjobRunner(ctrl), nil
				},
				newEnvCompatibilityChecker: func() (versionCompatibilityChecker, error) {
					return tc.mockEnvChecker(ctrl), nil
				},
			}

			err := jobRunOpts.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}

}

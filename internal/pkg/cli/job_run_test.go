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
	configStore    *mocks.Mockstore
	sel            *mocks.MockdeploySelector
	configSelector *mocks.MockconfigSelector
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
		"validate app env and svc with all flags passed in": {
			inputApp:     inputApp,
			inputJob:     inputJob,
			inputEnvName: inputEnv,
			setupMocks: func(m jobRunMock) {
				gomock.InOrder(
					m.configStore.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil),
					m.configStore.EXPECT().GetEnvironment("my-app", "my-env").Return(&config.Environment{Name: "my-env"}, nil),
					m.configStore.EXPECT().GetJob("my-app", "my-job").Return(&config.Workload{}, nil),
					m.configSelector.EXPECT().Environment(envPrompt, envHelpPrompt, "my-app").Return("my-env", nil),
					m.configSelector.EXPECT().Job(jobNamePrompt, jobNameHelpPrompt, "my-app").Return("my-job", nil),
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
					m.sel.EXPECT().Application(jobAppNamePrompt, svcAppNameHelpPrompt).Return("my-app", nil),
					m.configStore.EXPECT().GetApplication(gomock.Any()).Times(0),
					m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes(),
					m.configStore.EXPECT().GetJob(gomock.Any(), gomock.Any()).AnyTimes(),
					m.configSelector.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Return("my-env", nil).AnyTimes(),
					m.configSelector.EXPECT().Job(gomock.Any(), gomock.Any(), gomock.Any()).Return("my-job", nil).AnyTimes(),
				)
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedJob: inputJob,
		},
		"returns error if fail to select app": {
			setupMocks: func(m jobRunMock) {
				gomock.InOrder(
					m.sel.EXPECT().Application(jobAppNamePrompt, svcAppNameHelpPrompt).Return("", errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("select application: some error"),
		},
		"prompt for job and env": {
			inputApp: inputApp,
			setupMocks: func(m jobRunMock) {
				gomock.InOrder(
					m.configStore.EXPECT().GetApplication(gomock.Any()).AnyTimes(),
					m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0),
					m.configStore.EXPECT().GetJob(gomock.Any(), gomock.Any()).Times(0),
					m.configSelector.EXPECT().Environment(envPrompt, envHelpPrompt, "my-app").Return("my-env", nil),
					m.configSelector.EXPECT().Job(jobNamePrompt, jobNameHelpPrompt, "my-app").Return("my-job", nil),
				)
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedJob: inputJob,
		},
		"return error if fail to select environment": {
			inputApp: inputApp,
			setupMocks: func(m jobRunMock) {
				gomock.InOrder(
					m.configStore.EXPECT().GetApplication(gomock.Any()).AnyTimes(),
					m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0),
					m.configStore.EXPECT().GetJob(gomock.Any(), gomock.Any()).Times(0),
					m.configSelector.EXPECT().Environment(envPrompt, envHelpPrompt, "my-app").Return("", errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("select environment for application my-app: some error"),
		},
		"return error if fail to select deployed jobs": {
			inputApp:     inputApp,
			inputEnvName: inputEnv,
			setupMocks: func(m jobRunMock) {
				gomock.InOrder(
					m.configStore.EXPECT().GetApplication(gomock.Any()).AnyTimes(),
					m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes(),
					m.configStore.EXPECT().GetJob(gomock.Any(), gomock.Any()).Times(0),
					m.configSelector.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes(),
					m.configSelector.EXPECT().Job(jobNamePrompt, jobNameHelpPrompt, inputApp).Return("", errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("select job for application my-app: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			mockSel := mocks.NewMockdeploySelector(ctrl)
			mockConfigSel := mocks.NewMockconfigSelector(ctrl)

			mocks := jobRunMock{
				configStore:    mockstore,
				sel:            mockSel,
				configSelector: mockConfigSel,
			}

			tc.setupMocks(mocks)

			jobRun := &jobRunOpts{
				jobRunVars: jobRunVars{
					envName: tc.inputEnvName,
					appName: tc.inputApp,
					jobName: tc.inputJob,
				},
				configStore:    mockstore,
				sel:            mockSel,
				configSelector: mockConfigSel,
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
		jobName       string
		mockjobRunner func(ctrl *gomock.Controller) Runner
		wantedError   error
	}{
		"success": {
			jobName: "mockJob",
			mockjobRunner: func(ctrl *gomock.Controller) Runner {
				m := mocks.NewMockRunner(ctrl)
				m.EXPECT().Run().Return(nil)
				return m
			},
		},
		"fail": {
			jobName: "mockJob",
			mockjobRunner: func(ctrl *gomock.Controller) Runner {
				m := mocks.NewMockRunner(ctrl)
				m.EXPECT().Run().Return(errors.New("some error"))
				return m
			},
			wantedError: fmt.Errorf("job execution mockJob: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			jobRunOpts := &jobRunOpts{
				jobRunVars: jobRunVars{
					jobName: tc.jobName,
				},
				runner: tc.mockjobRunner(ctrl),
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

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
					m.sel.EXPECT().Application(jobAppNamePrompt, svcAppNameHelpPrompt).Return("my-app", nil),
					m.configStore.EXPECT().GetApplication(gomock.Any()).Times(0),
					m.configStore.EXPECT().GetJob(gomock.Any(), gomock.Any()).AnyTimes(),
					m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes(),
					m.configSelector.EXPECT().Job(gomock.Any(), gomock.Any(), gomock.Any()).Return("my-job", nil).AnyTimes(),
					m.configSelector.EXPECT().Environment(gomock.Any(), gomock.Any(), gomock.Any()).Return("my-env", nil).AnyTimes(),
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
					m.configStore.EXPECT().GetJob(gomock.Any(), gomock.Any()).Times(0),
					m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0),
					m.configSelector.EXPECT().Job("Select a job from your workspace", "The job you want to run", "my-app").Return("my-job", nil),
					m.configSelector.EXPECT().Environment("Select an environment", "The environment to run your job in", "my-app").Return("my-env", nil),
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
					m.configSelector.EXPECT().Environment("Select an environment", "The environment to run your job in", "my-app").Return("", errors.New("some error")),
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
					m.configSelector.EXPECT().Job("Select a job from your workspace", "The job you want to run", "my-app").Return("", errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("select job: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			mockSel := mocks.NewMockdeploySelector(ctrl)
			mockconfigSelector := mocks.NewMockconfigSelector(ctrl)

			mocks := jobRunMock{
				configStore:    mockstore,
				sel:            mockSel,
				configSelector: mockconfigSelector,
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
				configSelector: mockconfigSelector,
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
		mockjobRunner func(ctrl *gomock.Controller) runner
		wantedError   error
	}{
		"success": {
			jobName: "mockJob",
			mockjobRunner: func(ctrl *gomock.Controller) runner {
				m := mocks.NewMockrunner(ctrl)
				m.EXPECT().Run().Return(nil)
				return m
			},
		},
		"fail": {
			jobName: "mockJob",
			mockjobRunner: func(ctrl *gomock.Controller) runner {
				m := mocks.NewMockrunner(ctrl)
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
				runner:     tc.mockjobRunner(ctrl),
				initRunner: func() {},
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

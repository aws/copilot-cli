// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/logging"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestJobLogs_Validate(t *testing.T) {
	const (
		mockSince        = 1 * time.Minute
		mockStartTime    = "1970-01-01T01:01:01+00:00"
		mockBadStartTime = "badStartTime"
		mockEndTime      = "1971-01-01T01:01:01+00:00"
		mockBadEndTime   = "badEndTime"
	)
	testCases := map[string]struct {
		inputApp          string
		inputSvc          string
		inputLimit        int
		inputLast         int
		inputFollow       bool
		inputEnvName      string
		inputStartTime    string
		inputEndTime      string
		inputSince        time.Duration
		inputStateMachine bool

		mockstore func(m *mocks.Mockstore)

		wantedError error
	}{
		"with no flag set": {
			mockstore: func(m *mocks.Mockstore) {},
		},
		"skip validation if app flag is not set": {
			inputSvc:     "frontend",
			inputEnvName: "test",
			mockstore:    func(m *mocks.Mockstore) {},
		},
		"invalid app name": {
			inputApp: "my-app",

			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"invalid env name": {
			inputApp:     "my-app",
			inputEnvName: "test",
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{}, nil)
				m.EXPECT().GetEnvironment("my-app", "test").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"invalid svc name": {
			inputApp: "my-app",
			inputSvc: "frontend",

			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{}, nil)
				m.EXPECT().GetJob("my-app", "frontend").Return(nil, errors.New("some error"))
			},

			wantedError: fmt.Errorf("some error"),
		},
		"returns error if since and startTime flags are set together": {
			inputSince:     mockSince,
			inputStartTime: mockStartTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("only one of --since or --start-time may be used"),
		},
		"returns error if follow and endTime flags are set together": {
			inputFollow:  true,
			inputEndTime: mockEndTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("only one of --follow or --end-time may be used"),
		},
		"returns error if invalid start time flag value": {
			inputStartTime: mockBadStartTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("invalid argument badStartTime for \"--start-time\" flag: reading time value badStartTime: parsing time \"badStartTime\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"badStartTime\" as \"2006\""),
		},
		"returns error if invalid end time flag value": {
			inputEndTime: mockBadEndTime,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("invalid argument badEndTime for \"--end-time\" flag: reading time value badEndTime: parsing time \"badEndTime\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"badEndTime\" as \"2006\""),
		},
		"returns error if invalid since flag value": {
			inputSince: -mockSince,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("--since must be greater than 0"),
		},
		"returns error if limit value is below limit": {
			inputLimit: -1,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("--limit -1 is out-of-bounds, value must be between 1 and 10000"),
		},
		"returns error if limit value is above limit": {
			inputLimit: 10001,

			mockstore: func(m *mocks.Mockstore) {},

			wantedError: fmt.Errorf("--limit 10001 is out-of-bounds, value must be between 1 and 10000"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			tc.mockstore(mockstore)

			jobLogs := &jobLogsOpts{
				jobLogsVars: jobLogsVars{
					wkldLogsVars: wkldLogsVars{
						follow:         tc.inputFollow,
						limit:          tc.inputLimit,
						envName:        tc.inputEnvName,
						humanStartTime: tc.inputStartTime,
						humanEndTime:   tc.inputEndTime,
						since:          tc.inputSince,
						name:           tc.inputSvc,
						appName:        tc.inputApp,
					},
					includeStateMachineLogs: tc.inputStateMachine,
					last:                    tc.inputLast,
				},
				wkldLogOpts: wkldLogOpts{
					configStore: mockstore,
				},
			}

			// WHEN
			err := jobLogs.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestJobLogs_Ask(t *testing.T) {
	const (
		inputApp = "my-app"
		inputEnv = "my-env"
		inputJob = "my-job"
	)
	testCases := map[string]struct {
		inputApp     string
		inputJob     string
		inputEnvName string

		setupMocks func(mocks wkldLogsMock)

		wantedApp   string
		wantedEnv   string
		wantedJob   string
		wantedError error
	}{
		"validate app env and job with all flags passed in": {
			inputApp:     inputApp,
			inputJob:     inputJob,
			inputEnvName: inputEnv,
			setupMocks: func(m wkldLogsMock) {
				gomock.InOrder(
					m.configStore.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil),
					m.configStore.EXPECT().GetEnvironment("my-app", "my-env").Return(&config.Environment{Name: "my-env"}, nil),
					m.configStore.EXPECT().GetJob("my-app", "my-job").Return(&config.Workload{}, nil),
					m.sel.EXPECT().DeployedJob(jobLogNamePrompt, jobLogNameHelpPrompt, "my-app", gomock.Any(), gomock.Any()).
						Return(&selector.DeployedJob{
							Env:  "my-env",
							Name: "my-job",
						}, nil), // Let prompter handles the case when svc(env) is definite.
				)
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedJob: inputJob,
		},
		"prompt for app name": {
			inputJob:     inputJob,
			inputEnvName: inputEnv,
			setupMocks: func(m wkldLogsMock) {
				m.sel.EXPECT().Application(jobAppNamePrompt, wkldAppNameHelpPrompt).Return("my-app", nil)
				m.configStore.EXPECT().GetApplication(gomock.Any()).Times(0)
				m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.configStore.EXPECT().GetJob(gomock.Any(), gomock.Any()).AnyTimes()
				m.sel.EXPECT().DeployedJob(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&selector.DeployedJob{
					Env:  "my-env",
					Name: "my-job",
				}, nil).AnyTimes()
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedJob: inputJob,
		},
		"returns error if fail to select app": {
			setupMocks: func(m wkldLogsMock) {
				gomock.InOrder(
					m.sel.EXPECT().Application(jobAppNamePrompt, wkldAppNameHelpPrompt).Return("", errors.New("some error")),
				)
			},
			wantedError: fmt.Errorf("select application: some error"),
		},
		"prompt for job and env": {
			inputApp: "my-app",
			setupMocks: func(m wkldLogsMock) {
				m.configStore.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.configStore.EXPECT().GetJob(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().DeployedJob(jobLogNamePrompt, jobLogNameHelpPrompt, "my-app", gomock.Any(), gomock.Any()).
					Return(&selector.DeployedJob{
						Env:  "my-env",
						Name: "my-job",
					}, nil)
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedJob: inputJob,
		},
		"return error if fail to select deployed job": {
			inputApp: inputApp,
			setupMocks: func(m wkldLogsMock) {
				m.configStore.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.configStore.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.configStore.EXPECT().GetJob(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().DeployedJob(jobLogNamePrompt, jobLogNameHelpPrompt, inputApp, gomock.Any(), gomock.Any()).
					Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("select deployed jobs for application my-app: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			mockSel := mocks.NewMockdeploySelector(ctrl)

			mocks := wkldLogsMock{
				configStore: mockstore,
				sel:         mockSel,
			}

			tc.setupMocks(mocks)

			jobLogs := &jobLogsOpts{
				jobLogsVars: jobLogsVars{
					wkldLogsVars: wkldLogsVars{
						envName: tc.inputEnvName,
						name:    tc.inputJob,
						appName: tc.inputApp,
					},
				},
				wkldLogOpts: wkldLogOpts{
					configStore: mockstore,
					sel:         mockSel,
				},
			}

			// WHEN
			err := jobLogs.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedApp, jobLogs.appName, "expected app name to match")
				require.Equal(t, tc.wantedJob, jobLogs.name, "expected job name to match")
				require.Equal(t, tc.wantedEnv, jobLogs.envName, "expected service name to match")
			}
		})
	}
}

func TestJobLogs_Execute(t *testing.T) {
	mockStartTime := int64(123456789)
	mockEndTime := int64(987654321)
	mockLimit := int64(10)
	var mockNilLimit *int64
	testCases := map[string]struct {
		inputJob  string
		follow    bool
		limit     int
		endTime   int64
		startTime int64
		taskIDs   []string

		mocklogsSvc func(ctrl *gomock.Controller) logEventsWriter

		last                int
		includeStateMachine bool

		wantedError error
	}{
		"success": {
			inputJob:            "mockJob",
			endTime:             mockEndTime,
			startTime:           mockStartTime,
			limit:               10,
			last:                4,
			includeStateMachine: true,

			mocklogsSvc: func(ctrl *gomock.Controller) logEventsWriter {
				m := mocks.NewMocklogEventsWriter(ctrl)
				m.EXPECT().WriteLogEvents(gomock.Any()).Do(func(param logging.WriteLogEventsOpts) {
					require.Equal(t, param.LogStreamLimit, 4)
					require.Equal(t, param.EndTime, &mockEndTime)
					require.Equal(t, param.StartTime, &mockStartTime)
					require.Equal(t, param.Limit, &mockLimit)
					require.Equal(t, param.IncludeStateMachineLogs, true)
				}).Return(nil)

				return m
			},

			wantedError: nil,
		},
		"success with no execution limit set": {
			inputJob:            "mockJob",
			includeStateMachine: true,
			mocklogsSvc: func(ctrl *gomock.Controller) logEventsWriter {
				m := mocks.NewMocklogEventsWriter(ctrl)
				m.EXPECT().WriteLogEvents(gomock.Any()).Do(func(param logging.WriteLogEventsOpts) {
					require.Equal(t, param.LogStreamLimit, 1)
					require.Equal(t, param.Limit, mockNilLimit)
					require.Equal(t, param.IncludeStateMachineLogs, true)
				}).Return(nil)

				return m
			},

			wantedError: nil,
		},
		"success with no limit set": {
			inputJob:  "mockJob",
			endTime:   mockEndTime,
			startTime: mockStartTime,
			follow:    true,
			taskIDs:   []string{"mockTaskID"},

			mocklogsSvc: func(ctrl *gomock.Controller) logEventsWriter {
				m := mocks.NewMocklogEventsWriter(ctrl)
				m.EXPECT().WriteLogEvents(gomock.Any()).Do(func(param logging.WriteLogEventsOpts) {
					require.Equal(t, param.TaskIDs, []string{"mockTaskID"})
					require.Equal(t, param.EndTime, &mockEndTime)
					require.Equal(t, param.StartTime, &mockStartTime)
					require.Equal(t, param.Follow, true)
					require.Equal(t, param.Limit, mockNilLimit)
				}).Return(nil)

				return m
			},

			wantedError: nil,
		},
		"returns error if fail to get event logs": {
			inputJob: "mockJob",

			mocklogsSvc: func(ctrl *gomock.Controller) logEventsWriter {
				m := mocks.NewMocklogEventsWriter(ctrl)
				m.EXPECT().WriteLogEvents(gomock.Any()).
					Return(errors.New("some error"))

				return m
			},

			wantedError: fmt.Errorf("write log events for job mockJob: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svcLogs := &jobLogsOpts{
				jobLogsVars: jobLogsVars{
					wkldLogsVars: wkldLogsVars{
						name:    tc.inputJob,
						follow:  tc.follow,
						limit:   tc.limit,
						taskIDs: tc.taskIDs,
					},
					includeStateMachineLogs: tc.includeStateMachine,
					last:                    tc.last,
				},
				wkldLogOpts: wkldLogOpts{
					startTime:          &tc.startTime,
					endTime:            &tc.endTime,
					initRuntimeClients: func() error { return nil },
					logsSvc:            tc.mocklogsSvc(ctrl),
				},
			}

			// WHEN
			err := svcLogs.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

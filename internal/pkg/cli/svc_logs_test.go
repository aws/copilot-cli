// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/logging"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type svcLogsMock struct {
	configStore *mocks.Mockstore
	sel         *mocks.MockdeploySelector
}

func TestSvcLogs_Validate(t *testing.T) {
	const (
		mockSince        = 1 * time.Minute
		mockStartTime    = "1970-01-01T01:01:01+00:00"
		mockBadStartTime = "badStartTime"
		mockEndTime      = "1971-01-01T01:01:01+00:00"
		mockBadEndTime   = "badEndTime"
	)
	testCases := map[string]struct {
		inputApp       string
		inputSvc       string
		inputLimit     int
		inputFollow    bool
		inputEnvName   string
		inputStartTime string
		inputEndTime   string
		inputSince     time.Duration

		mockstore func(m *mocks.Mockstore)

		wantedError error
	}{
		"with no flag set": {
			mockstore: func(m *mocks.Mockstore) {},

			wantedError: nil,
		},
		"invalid project name": {
			inputApp: "my-app",

			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(nil, errors.New("some error"))
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

			svcLogs := &svcLogsOpts{
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
				configStore: mockstore,
			}

			// WHEN
			err := svcLogs.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSvcLogs_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputApp     string
		inputSvc     string
		inputEnvName string

		setupMocks func(mocks svcLogsMock)

		wantedError error
	}{
		"with all flag set": {
			inputApp:     "mockApp",
			inputSvc:     "mockSvc",
			inputEnvName: "mockEnv",

			setupMocks: func(m svcLogsMock) {
				gomock.InOrder(
					m.sel.EXPECT().DeployedService(svcLogNamePrompt, svcLogNameHelpPrompt, "mockApp",
						gomock.Any(), gomock.Any()).Return(&selector.DeployedService{
						Env: "mockEnv",
						Svc: "mockSvc",
					}, nil),
				)
			},

			wantedError: nil,
		},
		"return error if fail to select deployed services": {
			inputApp:     "mockApp",
			inputSvc:     "mockSvc",
			inputEnvName: "mockEnv",

			setupMocks: func(m svcLogsMock) {
				gomock.InOrder(
					m.sel.EXPECT().DeployedService(svcLogNamePrompt, svcLogNameHelpPrompt, "mockApp",
						gomock.Any(), gomock.Any()).Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("select deployed services for application mockApp: some error"),
		},
		"with no flag set": {
			setupMocks: func(m svcLogsMock) {
				gomock.InOrder(
					m.sel.EXPECT().Application(svcAppNamePrompt, svcAppNameHelpPrompt).Return("mockApp", nil),
					m.sel.EXPECT().DeployedService(svcLogNamePrompt, svcLogNameHelpPrompt, "mockApp",
						gomock.Any(), gomock.Any()).Return(&selector.DeployedService{
						Env: "mockEnv",
						Svc: "mockSvc",
					}, nil),
				)
			},

			wantedError: nil,
		},
		"returns error if fail to select app": {
			setupMocks: func(m svcLogsMock) {
				gomock.InOrder(
					m.sel.EXPECT().Application(svcAppNamePrompt, svcAppNameHelpPrompt).Return("", errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("select application: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			mockSel := mocks.NewMockdeploySelector(ctrl)

			mocks := svcLogsMock{
				configStore: mockstore,
				sel:         mockSel,
			}

			tc.setupMocks(mocks)

			svcLogs := &svcLogsOpts{
				wkldLogsVars: wkldLogsVars{
					envName: tc.inputEnvName,
					name:    tc.inputSvc,
					appName: tc.inputApp,
				},
				configStore: mockstore,
				sel:         mockSel,
			}

			// WHEN
			err := svcLogs.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSvcLogs_Execute(t *testing.T) {
	mockStartTime := int64(123456789)
	mockEndTime := int64(987654321)
	mockLimit := int64(10)
	var mockNilLimit *int64
	testCases := map[string]struct {
		inputSvc  string
		follow    bool
		limit     int
		endTime   int64
		startTime int64
		taskIDs   []string

		mocklogsSvc func(ctrl *gomock.Controller) logEventsWriter

		wantedError error
	}{
		"success": {
			inputSvc:  "mockSvc",
			endTime:   mockEndTime,
			startTime: mockStartTime,
			follow:    true,
			limit:     10,
			taskIDs:   []string{"mockTaskID"},

			mocklogsSvc: func(ctrl *gomock.Controller) logEventsWriter {
				m := mocks.NewMocklogEventsWriter(ctrl)
				m.EXPECT().WriteLogEvents(gomock.Any()).Do(func(param logging.WriteLogEventsOpts) {
					require.Equal(t, param.TaskIDs, []string{"mockTaskID"})
					require.Equal(t, param.EndTime, &mockEndTime)
					require.Equal(t, param.StartTime, &mockStartTime)
					require.Equal(t, param.Follow, true)
					require.Equal(t, param.Limit, &mockLimit)
				}).Return(nil)

				return m
			},

			wantedError: nil,
		},
		"success with no limit set": {
			inputSvc:  "mockSvc",
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
			inputSvc: "mockSvc",

			mocklogsSvc: func(ctrl *gomock.Controller) logEventsWriter {
				m := mocks.NewMocklogEventsWriter(ctrl)
				m.EXPECT().WriteLogEvents(gomock.Any()).
					Return(errors.New("some error"))

				return m
			},

			wantedError: fmt.Errorf("write log events for service mockSvc: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			svcLogs := &svcLogsOpts{
				wkldLogsVars: wkldLogsVars{
					name:    tc.inputSvc,
					follow:  tc.follow,
					limit:   tc.limit,
					taskIDs: tc.taskIDs,
				},
				logTimeKeeping: logTimeKeeping{
					startTime: &tc.startTime,
					endTime:   &tc.endTime,
				},
				initLogsSvc: func() error { return nil },
				logsSvc:     tc.mocklogsSvc(ctrl),
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

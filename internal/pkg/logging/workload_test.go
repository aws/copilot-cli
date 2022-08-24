// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"bytes"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/logging/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type workloadLogsMocks struct {
	logGetter        *mocks.MocklogGetter
	serviceARNGetter *mocks.MockserviceARNGetter
}

func TestECSServiceLogger_WriteLogEvents(t *testing.T) {
	const (
		mockLogGroupName     = "mockLogGroup"
		logEventsHumanString = `firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
`
		logEventsJSONString = "{\"logStreamName\":\"firelens_log_router/fcfe4ab8043841c08162318e5ad805f1\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"GET / HTTP/1.1\\\" 200 -\",\"timestamp\":0}\n{\"logStreamName\":\"firelens_log_router/fcfe4ab8043841c08162318e5ad805f1\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"FATA some error\\\" - -\",\"timestamp\":0}\n{\"logStreamName\":\"firelens_log_router/fcfe4ab8043841c08162318e5ad805f1\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"WARN some warning\\\" - -\",\"timestamp\":0}\n"
	)
	mockLogEvents := []*cloudwatchlogs.Event{
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -`,
		},
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -`,
		},
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -`,
		},
	}
	mockMoreLogEvents := []*cloudwatchlogs.Event{
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 404 -`,
		},
	}
	mockCurrentTimestamp := time.Date(2020, 11, 23, 0, 0, 0, 0, time.UTC) // Copilot GA date :).
	testCases := map[string]struct {
		follow        bool
		limit         *int64
		startTime     *int64
		jsonOutput    bool
		taskIDs       []string
		containerName string
		setupMocks    func(mocks workloadLogsMocks)

		wantedError   error
		wantedContent string
	}{
		"failed to get task log events": {
			setupMocks: func(m workloadLogsMocks) {
				m.logGetter.EXPECT().LogEvents(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("get log events for log group mockLogGroup: some error"),
		},
		"success with human output": {
			limit: aws.Int64(100),
			setupMocks: func(m workloadLogsMocks) {
				m.logGetter.EXPECT().LogEvents(gomock.Any()).
					Do(func(param cloudwatchlogs.LogEventsOpts) {
						require.Equal(t, param.Limit, aws.Int64(100))
					}).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: mockLogEvents,
					}, nil)
			},
			wantedContent: logEventsHumanString,
		},
		"success with json output": {
			jsonOutput: true,
			startTime:  aws.Int64(123456789),
			setupMocks: func(m workloadLogsMocks) {
				m.logGetter.EXPECT().LogEvents(gomock.Any()).
					Do(func(param cloudwatchlogs.LogEventsOpts) {
						require.Equal(t, param.Limit, (*int64)(nil))
					}).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: mockLogEvents,
					}, nil)
			},
			wantedContent: logEventsJSONString,
		},
		"success with follow flag": {
			follow: true,
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.Limit, (*int64)(nil))
							require.Equal(t, param.StartTime, aws.Int64(mockCurrentTimestamp.UnixMilli()))
						}).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: mockLogEvents,
							StreamLastEventTime: map[string]int64{
								"mockLogStreamName": 123456,
							},
						}, nil),
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events:              mockMoreLogEvents,
							StreamLastEventTime: nil,
						}, nil),
				)
			},
			wantedContent: `firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 404 -
`,
		},
		"success with log limit set": {
			limit: aws.Int64(50),
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.LogStreamPrefixFilters, []string{"copilot/"})
							require.Equal(t, param.Limit, aws.Int64(50))
						}).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: mockLogEvents,
						}, nil),
				)
			},
			wantedContent: logEventsHumanString,
		},
		"success when filtered by task IDs": {
			taskIDs: []string{"mockTaskID1"},
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.LogStreamPrefixFilters, []string{"copilot/mockSvc/mockTaskID1"})
							require.Equal(t, param.Limit, aws.Int64(10))
						}).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: mockLogEvents,
						}, nil),
				)
			},
			wantedContent: logEventsHumanString,
		},
		"success when filtered by certain container and certain tasks": {
			containerName: "datadog",
			taskIDs:       []string{"mockTaskID"},
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.LogStreamPrefixFilters, []string{"copilot/datadog/mockTaskID"})
							require.Equal(t, param.Limit, aws.Int64(10))
						}).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: mockLogEvents,
						}, nil),
				)
			},
			wantedContent: logEventsHumanString,
		},
		"success when filtered by certain container": {
			containerName: "datadog",
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.LogStreamPrefixFilters, []string{"copilot/datadog"})
							require.Equal(t, param.Limit, aws.Int64(10))
						}).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: mockLogEvents,
						}, nil),
				)
			},
			wantedContent: logEventsHumanString,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocklogGetter := mocks.NewMocklogGetter(ctrl)

			mocks := workloadLogsMocks{
				logGetter: mocklogGetter,
			}

			tc.setupMocks(mocks)

			b := &bytes.Buffer{}
			svcLogs := &ECSServiceLogger{
				workloadLogger: &workloadLogger{
					app:          "mockApp",
					env:          "mockEnv",
					name:         "mockSvc",
					eventsGetter: mocklogGetter,
					w:            b,
					now: func() time.Time {
						return mockCurrentTimestamp
					},
				},
			}

			// WHEN
			logWriter := WriteHumanLogs
			if tc.jsonOutput {
				logWriter = WriteJSONLogs
			}
			err := svcLogs.WriteLogEvents(WriteLogEventsOpts{
				Follow:        tc.follow,
				TaskIDs:       tc.taskIDs,
				Limit:         tc.limit,
				StartTime:     tc.startTime,
				OnEvents:      logWriter,
				ContainerName: tc.containerName,
				LogGroup:      mockLogGroupName,
			})

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedContent, b.String(), "expected output content match")
			}
		})
	}
}

func TestAppRunnerServiceLogger_WriteLogEvents(t *testing.T) {
	const (
		logEventsHumanString = `instance/85372273718e4806 web-server@1.0.0 start /app
instance/4e66ee07f2034a7c Server is running on port 4055
`
		logEventsJSONString = "{\"logStreamName\":\"instance/85372273718e4806b5cd805044755bc8\",\"ingestionTime\":0,\"message\":\"web-server@1.0.0 start /app\",\"timestamp\":0}\n{\"logStreamName\":\"instance/4e66ee07f2034a7cb287fdb5f2fd04f9\",\"ingestionTime\":0,\"message\":\"Server is running on port 4055\",\"timestamp\":0}\n"
	)
	logEvents := []*cloudwatchlogs.Event{
		{
			LogStreamName: "instance/85372273718e4806b5cd805044755bc8",
			Message:       `web-server@1.0.0 start /app`,
		},
		{
			LogStreamName: "instance/4e66ee07f2034a7cb287fdb5f2fd04f9",
			Message:       `Server is running on port 4055`,
		},
	}
	mockLimit := aws.Int64(100)
	var mockNilLimit *int64
	mockStartTime := aws.Int64(123456789)
	testCases := map[string]struct {
		follow       bool
		limit        *int64
		startTime    *int64
		jsonOutput   bool
		logGroupName string
		setupMocks   func(mocks workloadLogsMocks)

		wantedError   error
		wantedContent string
	}{
		"failed to get service ARN": {
			setupMocks: func(m workloadLogsMocks) {
				m.serviceARNGetter.EXPECT().ServiceARN("mockEnv").Return("", errors.New("some error"))
			},
			wantedError: fmt.Errorf("get service ARN for mockSvc: some error"),
		},
		"failed to get log events": {
			logGroupName: "mockLogGroup",
			setupMocks: func(m workloadLogsMocks) {
				m.logGetter.EXPECT().LogEvents(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("get log events for log group mockLogGroup: some error"),
		},
		"success with human output": {
			limit:        mockLimit,
			logGroupName: "mockLogGroup",
			setupMocks: func(m workloadLogsMocks) {
				m.logGetter.EXPECT().LogEvents(gomock.Any()).
					Do(func(param cloudwatchlogs.LogEventsOpts) {
						require.Equal(t, param.Limit, mockLimit)
					}).Return(&cloudwatchlogs.LogEventsOutput{
					Events: logEvents,
				}, nil)
			},
			wantedContent: logEventsHumanString,
		},
		"success with json output": {
			jsonOutput:   true,
			startTime:    mockStartTime,
			logGroupName: "mockLogGroup",
			setupMocks: func(m workloadLogsMocks) {
				m.logGetter.EXPECT().LogEvents(gomock.Any()).
					Do(func(param cloudwatchlogs.LogEventsOpts) {
						require.Equal(t, param.Limit, mockNilLimit)
					}).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: logEvents,
					}, nil)
			},
			wantedContent: logEventsJSONString,
		},
		"success with application log": {
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.serviceARNGetter.EXPECT().ServiceARN("mockEnv").Return("arn:aws:apprunner:us-east-1:11111111111:service/mockSvc/mockSvcID", nil),
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.LogGroup, "/aws/apprunner/mockSvc/mockSvcID/application")
						}).Return(&cloudwatchlogs.LogEventsOutput{
						Events: logEvents,
					}, nil),
				)
			},
			wantedContent: logEventsHumanString,
		},
		"success with system log": {
			logGroupName: "system",
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.serviceARNGetter.EXPECT().ServiceARN("mockEnv").Return("arn:aws:apprunner:us-east-1:11111111111:service/mockSvc/mockSvcID", nil),
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.LogGroup, "/aws/apprunner/mockSvc/mockSvcID/service")
						}).Return(&cloudwatchlogs.LogEventsOutput{
						Events: logEvents,
					}, nil),
				)
			},
			wantedContent: logEventsHumanString,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := workloadLogsMocks{
				logGetter:        mocks.NewMocklogGetter(ctrl),
				serviceARNGetter: mocks.NewMockserviceARNGetter(ctrl),
			}

			tc.setupMocks(m)

			b := &bytes.Buffer{}
			svcLogs := &AppRunnerServiceLogger{
				workloadLogger: &workloadLogger{
					app:          "mockApp",
					env:          "mockEnv",
					name:         "mockSvc",
					eventsGetter: m.logGetter,
					w:            b,
				},
				serviceARNGetter: m.serviceARNGetter,
			}

			// WHEN
			logWriter := WriteHumanLogs
			if tc.jsonOutput {
				logWriter = WriteJSONLogs
			}
			err := svcLogs.WriteLogEvents(WriteLogEventsOpts{
				Follow:    tc.follow,
				Limit:     tc.limit,
				StartTime: tc.startTime,
				LogGroup:  tc.logGroupName,
				OnEvents:  logWriter,
			})

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedContent, b.String(), "expected output content match")
			}
		})
	}
}

func TestJobLogger_WriteLogEvents(t *testing.T) {
	const (
		mockLogGroupName     = "mockLogGroup"
		logEventsHumanString = `firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
`
		logEventsJSONString = "{\"logStreamName\":\"firelens_log_router/fcfe4ab8043841c08162318e5ad805f1\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"GET / HTTP/1.1\\\" 200 -\",\"timestamp\":0}\n{\"logStreamName\":\"firelens_log_router/fcfe4ab8043841c08162318e5ad805f1\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"FATA some error\\\" - -\",\"timestamp\":0}\n{\"logStreamName\":\"firelens_log_router/fcfe4ab8043841c08162318e5ad805f1\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"WARN some warning\\\" - -\",\"timestamp\":0}\n"
	)
	mockLogEvents := []*cloudwatchlogs.Event{
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -`,
		},
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -`,
		},
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -`,
		},
	}
	mockMoreLogEvents := []*cloudwatchlogs.Event{
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 404 -`,
		},
	}
	mockCurrentTimestamp := time.Date(2020, 11, 23, 0, 0, 0, 0, time.UTC) // Copilot GA date :).
	testCases := map[string]struct {
		follow              bool
		logStreamLimit      int
		limit               *int64
		startTime           *int64
		jsonOutput          bool
		taskIDs             []string
		includeStateMachine bool
		setupMocks          func(mocks workloadLogsMocks)

		wantedError   error
		wantedContent string
	}{
		"failed to get task log events": {
			setupMocks: func(m workloadLogsMocks) {
				m.logGetter.EXPECT().LogEvents(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedError: fmt.Errorf("get log events for log group mockLogGroup: some error"),
		},
		"success with human output": {
			limit: aws.Int64(100),
			setupMocks: func(m workloadLogsMocks) {
				m.logGetter.EXPECT().LogEvents(gomock.Any()).
					Do(func(param cloudwatchlogs.LogEventsOpts) {
						require.Equal(t, param.Limit, aws.Int64(100))
					}).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: mockLogEvents,
					}, nil)
			},
			wantedContent: logEventsHumanString,
		},
		"success with json output": {
			jsonOutput: true,
			startTime:  aws.Int64(123456789),
			setupMocks: func(m workloadLogsMocks) {
				m.logGetter.EXPECT().LogEvents(gomock.Any()).
					Do(func(param cloudwatchlogs.LogEventsOpts) {
						require.Equal(t, param.Limit, (*int64)(nil))
					}).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: mockLogEvents,
					}, nil)
			},
			wantedContent: logEventsJSONString,
		},
		"success with follow flag": {
			follow: true,
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.Limit, (*int64)(nil))
							require.Equal(t, param.StartTime, aws.Int64(mockCurrentTimestamp.UnixMilli()))
						}).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: mockLogEvents,
							StreamLastEventTime: map[string]int64{
								"mockLogStreamName": 123456,
							},
						}, nil),
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events:              mockMoreLogEvents,
							StreamLastEventTime: nil,
						}, nil),
				)
			},
			wantedContent: `firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 404 -
`,
		},
		"success with log limit set": {
			limit: aws.Int64(50),
			setupMocks: func(m workloadLogsMocks) {
				m.logGetter.EXPECT().LogEvents(gomock.Any()).
					Do(func(param cloudwatchlogs.LogEventsOpts) {
						require.Equal(t, param.LogStreamPrefixFilters, []string{"copilot/"})
						require.Equal(t, param.Limit, aws.Int64(50))
					}).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: mockLogEvents,
					}, nil)
			},
			wantedContent: logEventsHumanString,
		},
		"success with state machine included": {
			includeStateMachine: true,
			logStreamLimit:      1,
			setupMocks: func(m workloadLogsMocks) {
				m.logGetter.EXPECT().LogEvents(gomock.Any()).
					Do(func(param cloudwatchlogs.LogEventsOpts) {
						require.Equal(t, param.LogStreamPrefixFilters, []string{"copilot/", "states"})
						require.Equal(t, param.Limit, (*int64)(nil))
						require.Equal(t, param.LogStreamLimit, 2)
					}).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: mockLogEvents,
					}, nil)
			},
			wantedContent: logEventsHumanString,
		},
		"success with log stream limit set": {
			logStreamLimit: 1,
			setupMocks: func(m workloadLogsMocks) {
				m.logGetter.EXPECT().LogEvents(gomock.Any()).
					Do(func(param cloudwatchlogs.LogEventsOpts) {
						require.Equal(t, param.LogStreamPrefixFilters, []string{"copilot/"})
						require.Equal(t, param.LogStreamLimit, 1)
						require.Equal(t, param.Limit, (*int64)(nil))
					}).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: mockLogEvents,
					}, nil)
			},
			wantedContent: logEventsHumanString,
		},
		"success with log stream limit and log limit": {
			logStreamLimit: 1,
			limit:          aws.Int64(50),
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.LogStreamPrefixFilters, []string{"copilot/"})
							require.Equal(t, param.LogStreamLimit, 1)
							require.Equal(t, param.Limit, aws.Int64(50))
						}).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: mockLogEvents,
						}, nil),
				)
			},
			wantedContent: logEventsHumanString,
		},
		"success when filtered by task IDs": {
			taskIDs: []string{"mockTaskID1"},
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.LogStreamPrefixFilters, []string{"copilot/mockSvc/mockTaskID1"})
							require.Equal(t, param.Limit, aws.Int64(10))
						}).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: mockLogEvents,
						}, nil),
				)
			},
			wantedContent: logEventsHumanString,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocklogGetter := mocks.NewMocklogGetter(ctrl)

			mocks := workloadLogsMocks{
				logGetter: mocklogGetter,
			}

			tc.setupMocks(mocks)

			b := &bytes.Buffer{}
			svcLogs := &JobLogger{
				workloadLogger: &workloadLogger{
					app:          "mockApp",
					env:          "mockEnv",
					name:         "mockSvc",
					eventsGetter: mocklogGetter,
					w:            b,
					now: func() time.Time {
						return mockCurrentTimestamp
					},
				},
			}

			// WHEN
			logWriter := WriteHumanLogs
			if tc.jsonOutput {
				logWriter = WriteJSONLogs
			}
			err := svcLogs.WriteLogEvents(WriteLogEventsOpts{
				Follow:                  tc.follow,
				TaskIDs:                 tc.taskIDs,
				Limit:                   tc.limit,
				StartTime:               tc.startTime,
				OnEvents:                logWriter,
				LogStreamLimit:          tc.logStreamLimit,
				LogGroup:                mockLogGroupName,
				IncludeStateMachineLogs: tc.includeStateMachine,
			})

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedContent, b.String(), "expected output content match")
			}
		})
	}
}

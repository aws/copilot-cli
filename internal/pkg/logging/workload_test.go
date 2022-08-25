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
	logGetter *mocks.MocklogGetter
}

func TestWorkloadClient_WriteLogEvents(t *testing.T) {
	const (
		mockLogGroupName     = "mockLogGroup"
		logEventsHumanString = `firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
`
		logEventsJSONString = "{\"logStreamName\":\"firelens_log_router/fcfe4ab8043841c08162318e5ad805f1\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"GET / HTTP/1.1\\\" 200 -\",\"timestamp\":0}\n{\"logStreamName\":\"firelens_log_router/fcfe4ab8043841c08162318e5ad805f1\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"FATA some error\\\" - -\",\"timestamp\":0}\n{\"logStreamName\":\"firelens_log_router/fcfe4ab8043841c08162318e5ad805f1\",\"ingestionTime\":0,\"message\":\"10.0.0.00 - - [01/Jan/1970 01:01:01] \\\"WARN some warning\\\" - -\",\"timestamp\":0}\n"
	)
	mockLastEventTime := map[string]int64{
		"mockLogStreamName": 123456,
	}
	logEvents := []*cloudwatchlogs.Event{
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
	moreLogEvents := []*cloudwatchlogs.Event{
		{
			LogStreamName: "firelens_log_router/fcfe4ab8043841c08162318e5ad805f1",
			Message:       `10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 404 -`,
		},
	}
	mockLimit := aws.Int64(100)
	var mockNilLimit *int64
	mockStartTime := aws.Int64(123456789)
	mockCurrentTimestamp := time.Date(2020, 11, 23, 0, 0, 0, 0, time.UTC) // Copilot GA date :).
	testCases := map[string]struct {
		follow              bool
		last                int
		limit               *int64
		startTime           *int64
		jsonOutput          bool
		taskIDs             []string
		includeStateMachine bool
		containerName       string
		setupMocks          func(mocks workloadLogsMocks)

		wantedError   error
		wantedContent string
	}{
		"failed to get task log events": {
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("get log events for log group mockLogGroup: some error"),
		},
		"success with human output": {
			limit: mockLimit,
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.Limit, mockLimit)
						}).Return(&cloudwatchlogs.LogEventsOutput{
						Events: logEvents,
					}, nil),
				)
			},

			wantedContent: logEventsHumanString,
		},
		"success with json output": {
			jsonOutput: true,
			startTime:  mockStartTime,
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.Limit, mockNilLimit)
						}).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: logEvents,
						}, nil),
				)
			},

			wantedContent: logEventsJSONString,
		},
		"success with follow flag": {
			follow:  true,
			taskIDs: []string{"mockTaskID1", "mockTaskID2"},
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.LogStreamPrefixFilters, []string{"copilot/mockSvc/mockTaskID1", "copilot/mockSvc/mockTaskID2"})
							var val *int64 = nil // Explicitly mark that nil is of type *int64 otherwise require.Equal returns an error.
							require.Equal(t, param.Limit, val)
							require.Equal(t, param.StartTime, aws.Int64(mockCurrentTimestamp.UnixMilli()))
						}).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events:              logEvents,
							StreamLastEventTime: mockLastEventTime,
						}, nil),
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events:              moreLogEvents,
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
		"success with no filtering": {
			taskIDs: []string{"mockTaskID1"},
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.LogStreamPrefixFilters, []string{"copilot/mockSvc/mockTaskID1"})
							require.Equal(t, param.Limit, aws.Int64(10))
						}).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: logEvents,
						}, nil),
				)
			},

			wantedContent: `firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
`,
		},
		"success with state machine included": {
			includeStateMachine: true,
			last:                1,
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.LogStreamPrefixFilters, []string{"copilot/", "states"})
							require.Equal(t, param.Limit, mockNilLimit)
							require.Equal(t, param.LogStreamLimit, 2)
						}).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: logEvents,
						}, nil),
				)
			},
			wantedContent: `firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
`,
		},
		"success with log stream limit set": {
			last: 1,
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.LogStreamPrefixFilters, []string{"copilot/"})
							require.Equal(t, param.LogStreamLimit, 1)
							require.Equal(t, param.Limit, mockNilLimit)
						}).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: logEvents,
						}, nil),
				)
			},
			wantedContent: `firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
`,
		},
		"success with log stream limit and log limit": {
			last:  1,
			limit: aws.Int64(50),
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.LogStreamPrefixFilters, []string{"copilot/"})
							require.Equal(t, param.LogStreamLimit, 1)
							require.Equal(t, param.Limit, aws.Int64(50))
						}).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: logEvents,
						}, nil),
				)
			},
			wantedContent: `firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
`,
		},
		"success with only log stream from certain container": {
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
							Events: logEvents,
						}, nil),
				)
			},
			wantedContent: `firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
`,
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
			svcLogs := &WorkloadClient{
				name:         "mockSvc",
				logGroupName: mockLogGroupName,
				isECS:        true,
				eventsGetter: mocklogGetter,
				w:            b,
				now: func() time.Time {
					return mockCurrentTimestamp
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
				LogStreamLimit:          tc.last,
				IncludeStateMachineLogs: tc.includeStateMachine,
				ContainerName:           tc.containerName,
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

func TestServiceClient_WriteAppRunnerSvcLogEvents(t *testing.T) {
	const (
		mockLogGroupName     = "mockLogGroup"
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
		follow     bool
		limit      *int64
		startTime  *int64
		jsonOutput bool
		setupMocks func(mocks workloadLogsMocks)

		wantedError   error
		wantedContent string
	}{
		"failed to get log events": {
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("get log events for log group mockLogGroup: some error"),
		},
		"success with human output": {
			limit: mockLimit,
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.Limit, mockLimit)
						}).Return(&cloudwatchlogs.LogEventsOutput{
						Events: logEvents,
					}, nil),
				)
			},
			wantedContent: logEventsHumanString,
		},
		"success with json output": {
			jsonOutput: true,
			startTime:  mockStartTime,
			setupMocks: func(m workloadLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Do(func(param cloudwatchlogs.LogEventsOpts) {
							require.Equal(t, param.Limit, mockNilLimit)
						}).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: logEvents,
						}, nil),
				)
			},
			wantedContent: logEventsJSONString,
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
			svcLogs := &WorkloadClient{
				logGroupName: mockLogGroupName,
				eventsGetter: mocklogGetter,
				w:            b,
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

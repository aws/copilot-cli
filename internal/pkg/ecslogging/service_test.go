// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ecslogging

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/ecslogging/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type serviceLogsMocks struct {
	logGetter *mocks.MocklogGetter
}

func TestServiceLogs_WriteServiceLogEvents(t *testing.T) {
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
	testCases := map[string]struct {
		follow     bool
		jsonOutput bool
		setupMocks func(mocks serviceLogsMocks)

		wantedError   error
		wantedContent string
	}{
		"failed to get task log events": {
			setupMocks: func(m serviceLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Return(nil, errors.New("some error")),
				)
			},

			wantedError: fmt.Errorf("get task log events for log group mockLogGroup: some error"),
		},
		"success with human output": {
			setupMocks: func(m serviceLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: logEvents,
						}, nil),
				)
			},

			wantedContent: logEventsHumanString,
		},
		"success with json output": {
			jsonOutput: true,
			setupMocks: func(m serviceLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events: logEvents,
						}, nil),
				)
			},

			wantedContent: logEventsJSONString,
		},
		"success with follow flag": {
			follow: true,
			setupMocks: func(m serviceLogsMocks) {
				gomock.InOrder(
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events:        logEvents,
							LastEventTime: mockLastEventTime,
						}, nil),
					m.logGetter.EXPECT().LogEvents(gomock.Any()).
						Return(&cloudwatchlogs.LogEventsOutput{
							Events:        moreLogEvents,
							LastEventTime: nil,
						}, nil),
				)
			},

			wantedContent: `firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 200 -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "FATA some error" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "WARN some warning" - -
firelens_log_router/fcfe4 10.0.0.00 - - [01/Jan/1970 01:01:01] "GET / HTTP/1.1" 404 -
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocklogGetter := mocks.NewMocklogGetter(ctrl)

			mocks := serviceLogsMocks{
				logGetter: mocklogGetter,
			}

			tc.setupMocks(mocks)

			b := &bytes.Buffer{}
			svcLogs := &ServiceClient{
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
				Follow:   tc.follow,
				OnEvents: logWriter,
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

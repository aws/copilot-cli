// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudwatchlogs

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestLogEvents(t *testing.T) {
	mockError := errors.New("some error")
	testCases := map[string]struct {
		logGroupName             string
		logStream                []string
		startTime                *int64
		endTime                  *int64
		limit                    *int64
		lastEventTime            map[string]int64
		mockcloudwatchlogsClient func(m *mocks.Mockapi)

		wantLogEvents     []*Event
		wantLastEventTime map[string]int64
		wantErr           error
	}{
		"should get log stream name and return log events": {
			logGroupName: "mockLogGroup",
			logStream:    []string{"copilot/mockLogGroup/mockLogStream1", "copilot/mockLogGroup/mockLogStream2"},
			mockcloudwatchlogsClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
					LogGroupName: aws.String("mockLogGroup"),
					Descending:   aws.Bool(true),
					OrderBy:      aws.String("LastEventTime"),
				}).Return(&cloudwatchlogs.DescribeLogStreamsOutput{
					LogStreams: []*cloudwatchlogs.LogStream{
						{
							LogStreamName: aws.String("copilot/mockLogGroup/mockLogStream1"),
						},
						{
							LogStreamName: aws.String("copilot/mockLogGroup/mockLogStream2"),
						},
						{
							LogStreamName: aws.String("copilot/mockLogGroup/mockLogStream3"),
						},
					},
				}, nil)

				m.EXPECT().GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
					LogGroupName:  aws.String("mockLogGroup"),
					LogStreamName: aws.String("copilot/mockLogGroup/mockLogStream1"),
				}).Return(&cloudwatchlogs.GetLogEventsOutput{
					Events: []*cloudwatchlogs.OutputLogEvent{
						{
							Message:   aws.String("some log"),
							Timestamp: aws.Int64(1),
						},
					},
				}, nil)

				m.EXPECT().GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
					LogGroupName:  aws.String("mockLogGroup"),
					LogStreamName: aws.String("copilot/mockLogGroup/mockLogStream2"),
				}).Return(&cloudwatchlogs.GetLogEventsOutput{
					Events: []*cloudwatchlogs.OutputLogEvent{
						{
							Message:   aws.String("other log"),
							Timestamp: aws.Int64(0),
						},
					},
				}, nil)
			},

			wantLogEvents: []*Event{
				{
					LogStreamName: "copilot/mockLogGroup/mockLogStream2",
					Message:       "other log",
					Timestamp:     0,
				},
				{
					LogStreamName: "copilot/mockLogGroup/mockLogStream1",
					Message:       "some log",
					Timestamp:     1,
				},
			},
			wantLastEventTime: map[string]int64{
				"copilot/mockLogGroup/mockLogStream1": 1,
				"copilot/mockLogGroup/mockLogStream2": 0,
			},
			wantErr: nil,
		},
		"should override startTime to be last event time when follow mode": {
			logGroupName: "mockLogGroup",
			startTime:    aws.Int64(1234567),
			lastEventTime: map[string]int64{
				"copilot/mockLogGroup/mockLogStream": 1234890,
			},
			mockcloudwatchlogsClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
					LogGroupName: aws.String("mockLogGroup"),
					Descending:   aws.Bool(true),
					OrderBy:      aws.String("LastEventTime"),
				}).Return(&cloudwatchlogs.DescribeLogStreamsOutput{
					LogStreams: []*cloudwatchlogs.LogStream{
						{
							LogStreamName: aws.String("copilot/mockLogGroup/mockLogStream"),
						},
					},
				}, nil)

				m.EXPECT().GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
					StartTime:     aws.Int64(1234891),
					LogGroupName:  aws.String("mockLogGroup"),
					LogStreamName: aws.String("copilot/mockLogGroup/mockLogStream"),
				}).Return(&cloudwatchlogs.GetLogEventsOutput{
					Events: []*cloudwatchlogs.OutputLogEvent{
						{
							Message:   aws.String("some log"),
							Timestamp: aws.Int64(1234892),
						},
					},
				}, nil)
			},

			wantLogEvents: []*Event{
				{
					LogStreamName: "copilot/mockLogGroup/mockLogStream",
					Message:       "some log",
					Timestamp:     1234892,
				},
			},
			wantLastEventTime: map[string]int64{
				"copilot/mockLogGroup/mockLogStream": 1234892,
			},
			wantErr: nil,
		},
		"should return limited number of log events": {
			logGroupName: "mockLogGroup",
			limit:        aws.Int64(1),
			mockcloudwatchlogsClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
					LogGroupName: aws.String("mockLogGroup"),
					Descending:   aws.Bool(true),
					OrderBy:      aws.String("LastEventTime"),
				}).Return(&cloudwatchlogs.DescribeLogStreamsOutput{
					LogStreams: []*cloudwatchlogs.LogStream{
						{
							LogStreamName: aws.String("copilot/mockLogGroup/mockLogStream"),
						},
					},
				}, nil)

				m.EXPECT().GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
					Limit:         aws.Int64(1),
					LogGroupName:  aws.String("mockLogGroup"),
					LogStreamName: aws.String("copilot/mockLogGroup/mockLogStream"),
				}).Return(&cloudwatchlogs.GetLogEventsOutput{
					Events: []*cloudwatchlogs.OutputLogEvent{
						{
							Message:   aws.String("some log"),
							Timestamp: aws.Int64(0),
						},
						{
							Message:   aws.String("other log"),
							Timestamp: aws.Int64(1),
						},
					},
				}, nil)
			},

			wantLogEvents: []*Event{
				{
					LogStreamName: "copilot/mockLogGroup/mockLogStream",
					Message:       "other log",
					Timestamp:     1,
				},
			},
			wantLastEventTime: map[string]int64{
				"copilot/mockLogGroup/mockLogStream": 1,
			},
			wantErr: nil,
		},
		"returns error if fail to describe log streams": {
			logGroupName: "mockLogGroup",
			mockcloudwatchlogsClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
					LogGroupName: aws.String("mockLogGroup"),
					Descending:   aws.Bool(true),
					OrderBy:      aws.String("LastEventTime"),
				}).Return(nil, mockError)
			},

			wantLogEvents: nil,
			wantErr:       fmt.Errorf("describe log streams of log group %s: %w", "mockLogGroup", mockError),
		},
		"returns error if no log stream found": {
			logGroupName: "mockLogGroup",
			mockcloudwatchlogsClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
					LogGroupName: aws.String("mockLogGroup"),
					Descending:   aws.Bool(true),
					OrderBy:      aws.String("LastEventTime"),
				}).Return(&cloudwatchlogs.DescribeLogStreamsOutput{
					LogStreams: []*cloudwatchlogs.LogStream{},
				}, nil)
			},

			wantLogEvents: nil,
			wantErr:       fmt.Errorf("no log stream found in log group %s", "mockLogGroup"),
		},
		"returns error if fail to get log events": {
			logGroupName: "mockLogGroup",
			mockcloudwatchlogsClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
					LogGroupName: aws.String("mockLogGroup"),
					Descending:   aws.Bool(true),
					OrderBy:      aws.String("LastEventTime"),
				}).Return(&cloudwatchlogs.DescribeLogStreamsOutput{
					LogStreams: []*cloudwatchlogs.LogStream{
						{
							LogStreamName: aws.String("mockLogStream"),
						},
					},
				}, nil)
				m.EXPECT().GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
					LogGroupName:  aws.String("mockLogGroup"),
					LogStreamName: aws.String("mockLogStream"),
				}).Return(nil, mockError)
			},

			wantLogEvents: nil,
			wantErr:       fmt.Errorf("get log events of %s/%s: %w", "mockLogGroup", "mockLogStream", mockError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockcloudwatchlogsClient := mocks.NewMockapi(ctrl)
			tc.mockcloudwatchlogsClient(mockcloudwatchlogsClient)

			service := CloudWatchLogs{
				client: mockcloudwatchlogsClient,
			}
			gotLogEventsOutput, gotErr := service.LogEvents(LogEventsOpts{
				LogGroup:            tc.logGroupName,
				EndTime:             tc.endTime,
				Limit:               tc.limit,
				LogStreams:          tc.logStream,
				StartTime:           tc.startTime,
				StreamLastEventTime: tc.lastEventTime,
			})

			if gotErr != nil {
				require.Equal(t, tc.wantErr, gotErr)
			} else {
				require.ElementsMatch(t, tc.wantLogEvents, gotLogEventsOutput.Events)
				require.Equal(t, tc.wantLastEventTime, gotLogEventsOutput.StreamLastEventTime)
			}
		})
	}
}

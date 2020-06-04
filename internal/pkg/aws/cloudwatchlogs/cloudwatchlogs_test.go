// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudwatchlogs

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/cloudwatchlogs/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestLogEvents(t *testing.T) {
	mockError := errors.New("some error")
	testCases := map[string]struct {
		logGroupName             string
		startTime                int64
		endTime                  int64
		limit                    int
		lastEventTime            map[string]int64
		mockcloudwatchlogsClient func(m *mocks.Mockapi)

		wantLogEvents     []*Event
		wantLastEventTime map[string]int64
		wantErr           error
	}{
		"should get log stream name and return log events": {
			logGroupName:  "mockLogGroup",
			startTime:     1234567,
			endTime:       1234568,
			limit:         10,
			lastEventTime: make(map[string]int64),
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
					},
				}, nil)

				m.EXPECT().GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
					StartTime:     aws.Int64(1234567),
					EndTime:       aws.Int64(1234568),
					Limit:         aws.Int64(10),
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
					StartTime:     aws.Int64(1234567),
					EndTime:       aws.Int64(1234568),
					Limit:         aws.Int64(10),
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
					LogStreamName: "mockLogGroup/mockLogStream2",
					Message:       "other log",
					Timestamp:     0,
				},
				{
					LogStreamName: "mockLogGroup/mockLogStream1",
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
			startTime:    1234567,
			endTime:      1234999,
			limit:        10,
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
					Limit:         aws.Int64(10),
					EndTime:       aws.Int64(1234999),
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
					LogStreamName: "mockLogGroup/mockLogStream",
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
			logGroupName:  "mockLogGroup",
			startTime:     1234567,
			endTime:       1234568,
			limit:         1,
			lastEventTime: make(map[string]int64),
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
					StartTime:     aws.Int64(1234567),
					EndTime:       aws.Int64(1234568),
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
					LogStreamName: "mockLogGroup/mockLogStream",
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
			logGroupName:  "mockLogGroup",
			startTime:     1234567,
			endTime:       1234568,
			limit:         10,
			lastEventTime: make(map[string]int64),
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
			logGroupName:  "mockLogGroup",
			startTime:     1234567,
			endTime:       1234568,
			limit:         10,
			lastEventTime: make(map[string]int64),
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
			logGroupName:  "mockLogGroup",
			startTime:     1234567,
			endTime:       1234568,
			limit:         10,
			lastEventTime: make(map[string]int64),
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
					StartTime:     aws.Int64(1234567),
					EndTime:       aws.Int64(1234568),
					Limit:         aws.Int64(10),
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

			gotLogEventsOutput, gotErr := service.TaskLogEvents(tc.logGroupName, tc.lastEventTime, WithLimit(tc.limit), WithStartTime(tc.startTime), WithEndTime(tc.endTime))

			if gotErr != nil {
				require.Equal(t, tc.wantErr, gotErr)
			} else {
				require.ElementsMatch(t, tc.wantLogEvents, gotLogEventsOutput.Events)
				require.Equal(t, tc.wantLastEventTime, gotLogEventsOutput.LastEventTime)
			}
		})
	}
}

func TestLogGroupExists(t *testing.T) {
	mockError := errors.New("some error")
	testCases := map[string]struct {
		logGroupName string

		mockcloudwatchlogsClient func(m *mocks.Mockapi)

		exist   bool
		wantErr error
	}{
		"should return true if a log group exists": {
			logGroupName: "mockLogGroup",
			mockcloudwatchlogsClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
					LogGroupName: aws.String("mockLogGroup"),
				}).Return(&cloudwatchlogs.DescribeLogStreamsOutput{
					LogStreams: []*cloudwatchlogs.LogStream{
						{
							LogStreamName: aws.String("mockLogStream"),
						},
					},
				}, nil)
			},

			exist:   true,
			wantErr: nil,
		},
		"should return false if a log group does not exist": {
			logGroupName: "mockLogGroup",
			mockcloudwatchlogsClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
					LogGroupName: aws.String("mockLogGroup"),
				}).Return(nil, awserr.New("ResourceNotFoundException", "some error", nil))
			},

			exist:   false,
			wantErr: nil,
		},
		"should return error if fail to describe log stream": {
			logGroupName: "mockLogGroup",
			mockcloudwatchlogsClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
					LogGroupName: aws.String("mockLogGroup"),
				}).Return(nil, mockError)
			},

			exist:   false,
			wantErr: mockError,
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

			exist, gotErr := service.LogGroupExists(tc.logGroupName)

			require.Equal(t, tc.exist, exist)
			require.Equal(t, tc.wantErr, gotErr)
		})
	}
}

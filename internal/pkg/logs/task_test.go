// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/task"
	"github.com/aws/copilot-cli/internal/pkg/task/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type writeEventMocks struct {
	eventsLogger *mocks.MockEventsLogger
	describer    *mocks.MockTasksDescriber
}

type mockWriter struct{}

func (mockWriter) Write(p []byte) (int, error) { return 0, nil }

func TestEventsWriter_WriteEventsUntilStopped(t *testing.T) {
	groupName := "my-log-group"
	testCases := map[string]struct {
		setUpMocks func(m writeEventMocks)

		wantedError error
	}{
		"error getting log events": {
			setUpMocks: func(m writeEventMocks) {
				m.eventsLogger.EXPECT().TaskLogEvents(groupName, gomock.Any(), gomock.Any()).
					Return(&cloudwatchlogs.LogEventsOutput{}, errors.New("error getting log events"))
			},
			wantedError: errors.New("get task log events: error getting log events"),
		},
		"error describing tasks": {
			setUpMocks: func(m writeEventMocks) {
				m.eventsLogger.EXPECT().TaskLogEvents(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: []*cloudwatchlogs.Event{},
					}, nil).AnyTimes()
				m.describer.EXPECT().DescribeTasks("cluster", []string{"task-1", "task-2", "task-3"}).
					Return(nil, errors.New("error describing tasks"))
			},
			wantedError: errors.New("describe tasks: error describing tasks"),
		},
		"success": {
			setUpMocks: func(m writeEventMocks) {
				m.eventsLogger.EXPECT().TaskLogEvents(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: []*cloudwatchlogs.Event{},
					}, nil).AnyTimes()
				m.describer.EXPECT().DescribeTasks("cluster", []string{"task-1", "task-2", "task-3"}).
					Return([]*ecs.Task{
						{
							TaskArn:    aws.String("task-1"),
							LastStatus: aws.String(ecs.DesiredStatusStopped),
						},
						{
							TaskArn:    aws.String("task-2"),
							LastStatus: aws.String(ecs.DesiredStatusStopped),
						},
						{
							TaskArn:    aws.String("task-3"),
							LastStatus: aws.String(ecs.DesiredStatusStopped),
						},
					}, nil)
			},
		},
	}

	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)
	theDayAfter := now.AddDate(0, 0, 2)

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tasks := []*task.Task{
				{
					TaskARN:    "task-1",
					ClusterARN: "cluster",
					StartedAt:  &now,
				},
				{
					TaskARN:    "task-2",
					ClusterARN: "cluster",
					StartedAt:  &tomorrow,
				},
				{
					TaskARN:    "task-3",
					ClusterARN: "cluster",
					StartedAt:  &theDayAfter,
				},
			}

			mocks := writeEventMocks{
				eventsLogger: mocks.NewMockEventsLogger(ctrl),
				describer:    mocks.NewMockTasksDescriber(ctrl),
			}
			tc.setUpMocks(mocks)

			ew := &TaskLogs{
				GroupName: groupName,
				Tasks:     tasks,

				Writer:       mockWriter{},
				EventsLogger: mocks.eventsLogger,
				Describer:    mocks.describer,
			}

			err := ew.WriteEventsUntilStopped()
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

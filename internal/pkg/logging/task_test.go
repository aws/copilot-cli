// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/logging/mocks"
	"github.com/aws/copilot-cli/internal/pkg/task"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type writeEventMocks struct {
	logGetter *mocks.MocklogGetter
	describer *mocks.MockTasksDescriber
}

type mockWriter struct{}

func (mockWriter) Write(p []byte) (int, error) { return 0, nil }

func TestEventsWriter_WriteEventsUntilStopped(t *testing.T) {
	const (
		groupName = "my-log-group"
		taskARN1  = "arn:aws:ecs:us-west-2:123456789:task/cluster/task1"
		taskARN2  = "arn:aws:ecs:us-west-2:123456789:task/cluster/task2"
		taskARN3  = "arn:aws:ecs:us-west-2:123456789:task/cluster/task3"
		taskARN4  = "task4"
	)
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)
	theDayAfter := now.AddDate(0, 0, 2)
	goodTasks := []*task.Task{
		{
			TaskARN:    taskARN1,
			ClusterARN: "cluster",
			StartedAt:  &now,
		},
		{
			TaskARN:    taskARN2,
			ClusterARN: "cluster",
			StartedAt:  &tomorrow,
		},
		{
			TaskARN:    taskARN3,
			ClusterARN: "cluster",
			StartedAt:  &theDayAfter,
		},
	}
	badTasks := []*task.Task{
		{
			TaskARN:    taskARN4,
			ClusterARN: "cluster",
			StartedAt:  &now,
		},
	}
	testCases := map[string]struct {
		tasks      []*task.Task
		setUpMocks func(m writeEventMocks)

		wantedError error
	}{
		"error parsing task ID": {
			tasks:       badTasks,
			setUpMocks:  func(m writeEventMocks) {},
			wantedError: errors.New("parse task ID from ARN task4"),
		},
		"error getting log events": {
			tasks: goodTasks,
			setUpMocks: func(m writeEventMocks) {
				m.logGetter.EXPECT().LogEvents(gomock.Any()).
					Return(&cloudwatchlogs.LogEventsOutput{}, errors.New("error getting log events"))
			},
			wantedError: errors.New("get task log events: error getting log events"),
		},
		"error describing tasks": {
			tasks: goodTasks,
			setUpMocks: func(m writeEventMocks) {
				m.logGetter.EXPECT().LogEvents(gomock.Any()).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: []*cloudwatchlogs.Event{},
					}, nil).AnyTimes()
				m.describer.EXPECT().DescribeTasks("cluster", []string{taskARN1, taskARN2, taskARN3}).
					Return(nil, errors.New("error describing tasks"))
			},
			wantedError: errors.New("describe tasks: error describing tasks"),
		},
		"success": {
			tasks: goodTasks,
			setUpMocks: func(m writeEventMocks) {
				m.logGetter.EXPECT().LogEvents(gomock.Any()).Do(func(param cloudwatchlogs.LogEventsOpts) {
					require.Equal(t, param.LogGroup, "/copilot/my-log-group")
					require.Equal(t, param.LogStreamPrefixFilters, []string{"copilot-task/my-log-group/task1", "copilot-task/my-log-group/task2", "copilot-task/my-log-group/task3"})
				}).
					Return(&cloudwatchlogs.LogEventsOutput{
						Events: []*cloudwatchlogs.Event{},
					}, nil).Times(numCWLogsCallsPerRound)
				m.describer.EXPECT().DescribeTasks("cluster", []string{taskARN1, taskARN2, taskARN3}).
					Return([]*ecs.Task{
						{
							TaskArn:    aws.String(taskARN1),
							LastStatus: aws.String(ecs.DesiredStatusStopped),
						},
						{
							TaskArn:    aws.String(taskARN2),
							LastStatus: aws.String(ecs.DesiredStatusStopped),
						},
						{
							TaskArn:    aws.String(taskARN3),
							LastStatus: aws.String(ecs.DesiredStatusStopped),
						},
					}, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := writeEventMocks{
				logGetter: mocks.NewMocklogGetter(ctrl),
				describer: mocks.NewMockTasksDescriber(ctrl),
			}
			tc.setUpMocks(mocks)

			ew := &TaskClient{
				groupName: groupName,
				tasks:     tc.tasks,

				eventsWriter:  mockWriter{},
				eventsLogger:  mocks.logGetter,
				taskDescriber: mocks.describer,

				sleep: func() {}, // no-op.
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

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecslogging contains utility functions for ECS logging.
package ecslogging

import (
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/task"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
)

const (
	numCWLogsCallsPerRound = 10
	fmtTaskLogGroupName    = "/copilot/%s"
)

// TasksDescriber describes ECS tasks.
type TasksDescriber interface {
	DescribeTasks(cluster string, taskARNs []string) ([]*ecs.Task, error)
}

// TaskClient retrieves the logs of Amazon ECS tasks.
type TaskClient struct {
	GroupName string
	Tasks     []*task.Task

	Writer       io.Writer
	EventsLogger logGetter
	Describer    TasksDescriber

	// Fields that are private that get modified each time
	runningTasks []*task.Task
}

// NewTaskClient returns a TaskClient that can retrieve logs from the given tasks under the groupName.
func NewTaskClient(sess *session.Session, groupName string, tasks []*task.Task) *TaskClient {
	logGroupName := fmt.Sprintf(fmtTaskLogGroupName, groupName)
	return &TaskClient{
		GroupName: logGroupName,
		Tasks:     tasks,

		Describer:    ecs.New(sess),
		EventsLogger: cloudwatchlogs.New(sess),
		Writer:       log.OutputWriter,
	}
}

// WriteEventsUntilStopped writes tasks' events to a writer until all tasks have stopped.
func (t *TaskClient) WriteEventsUntilStopped() error {
	startTime := earliestStartTime(t.Tasks)
	t.runningTasks = t.Tasks
	lastEventTimestampByLogGroup := make(map[string]int64)
	for {
		for i := 0; i < numCWLogsCallsPerRound; i++ {
			logEventsOutput, err := t.EventsLogger.LogEvents(t.GroupName, lastEventTimestampByLogGroup, cloudwatchlogs.WithStartTime(aws.TimeUnixMilli(startTime)))
			if err != nil {
				return fmt.Errorf("get task log events: %w", err)
			}
			if err := WriteHumanLogs(t.Writer, cwEventsToHumanJSONStringers(logEventsOutput.Events)); err != nil {
				return fmt.Errorf("write log event: %w", err)
			}
			lastEventTimestampByLogGroup = logEventsOutput.LastEventTime
			time.Sleep(cloudwatchlogs.SleepDuration)
		}
		stopped, err := t.allTasksStopped()
		if err != nil {
			return err
		}
		if stopped {
			return nil
		}
	}
}

func (t *TaskClient) allTasksStopped() (bool, error) {
	taskARNs := make([]string, len(t.runningTasks))
	for idx, task := range t.runningTasks {
		taskARNs[idx] = task.TaskARN
	}

	// NOTE: all tasks are deployed to the same cluster and there are at least one tasks being deployed
	cluster := t.runningTasks[0].ClusterARN

	tasksResp, err := t.Describer.DescribeTasks(cluster, taskARNs)
	if err != nil {
		return false, fmt.Errorf("describe tasks: %w", err)
	}

	stopped := true
	var runningTasks []*task.Task
	for _, t := range tasksResp {
		if *t.LastStatus != ecs.DesiredStatusStopped {
			stopped = false
			runningTasks = append(runningTasks, &task.Task{
				ClusterARN: *t.ClusterArn,
				TaskARN:    *t.TaskArn,
			})
		}
	}
	t.runningTasks = runningTasks
	return stopped, nil
}

// The `StartedAt` field for the tasks shouldn't be nil.
func earliestStartTime(tasks []*task.Task) time.Time {
	earliest := *tasks[0].StartedAt
	for _, task := range tasks {
		if task.StartedAt.Before(earliest) {
			earliest = *task.StartedAt
		}
	}
	return earliest
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logs

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

// EventsLogger gets a log group's log events.
type EventsLogger interface {
	TaskLogEvents(logGroupName string,
		streamLastEventTime map[string]int64,
		opts ...cloudwatchlogs.GetLogEventsOpts) (*cloudwatchlogs.LogEventsOutput, error)
}

// TaskLogs represents a service that writes task log events.
type TaskLogs struct {
	GroupName string
	Tasks     []*task.Task

	Writer       io.Writer
	EventsLogger EventsLogger
	Describer    TasksDescriber

	// Fields that are private that get modified each time
	runningTasks []*task.Task
}

// NewTaskLogs returns a TaskLogs configured against the input.
func NewTaskLogs(groupName string, tasks []*task.Task, sess *session.Session) *TaskLogs {
	logGroupName := fmt.Sprintf(fmtTaskLogGroupName, groupName)
	return &TaskLogs{
		GroupName: logGroupName,
		Tasks:     tasks,

		Describer:    ecs.New(sess),
		EventsLogger: cloudwatchlogs.New(sess),
		Writer:       log.OutputWriter,
	}
}

// WriteEventsUntilStopped writes tasks' events to a writer until all tasks have stopped.
func (t *TaskLogs) WriteEventsUntilStopped() error {
	startTime := earliestStartTime(t.Tasks)
	t.runningTasks = t.Tasks
	lastEventTimestampByLogGroup := make(map[string]int64)
	for {
		for i := 0; i < numCWLogsCallsPerRound; i++ {
			logEventsOutput, err := t.EventsLogger.TaskLogEvents(t.GroupName, lastEventTimestampByLogGroup, cloudwatchlogs.WithStartTime(aws.TimeUnixMilli(startTime)))
			if err != nil {
				return fmt.Errorf("get task log events: %w", err)
			}
			if err := OutputCwLogsHuman(t.Writer, logEventsOutput.Events); err != nil {
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

func (t *TaskLogs) allTasksStopped() (bool, error) {
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

// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"io"
	"time"
)

const (
	numCWLogsCallsPerRound = 10
)

// TasksDescriber describes ECS tasks.
type TasksDescriber interface {
	DescribeTasks(cluster string, taskARNs []string) ([]*ecs.Task, error)
}

// CWLogService gets a log group's log events.
type CWLogService interface {
	TaskLogEvents(logGroupName string,
			streamLastEventTime map[string]int64,
			opts ...cloudwatchlogs.GetLogEventsOpts) (*cloudwatchlogs.LogEventsOutput, error)
}

// EventsWriter represents a writer that writes tasks' log events to a writer.
type EventsWriter struct {
	GroupName string
	Tasks     []*Task

	// Fields that are private that get modified each time
	lastEventTimestampByLogGroup map[string]int64
	runningTasks []*Task
}


// WriteEventsUntilStopped writes tasks' events to a writer until all tasks have stopped.
func (ew *EventsWriter) WriteEventsUntilStopped(w io.Writer, cwLogsGetter CWLogService, describer TaskDescriber) error {
	startTime := EarliestStartTime(ew.Tasks)
	ew.runningTasks = ew.Tasks
	ew.lastEventTimestampByLogGroup = make(map[string]int64)

	for {
		for i := 0; i < numCWLogsCallsPerRound; i++ {
			if err := ew.writeEvents(w, cwLogsGetter, cloudwatchlogs.WithStartTime(aws.TimeUnixMilli(startTime))); err != nil {
				return err
			}
			time.Sleep(cloudwatchlogs.SleepDuration)
		}
		stopped, err := ew.stopped(describer)
		if err != nil {
			return err
		}
		if stopped {
			return nil
		}
	}
}

// TODO: move this to a different package because this is not a task-specific method
func (ew *EventsWriter) writeEvents(w io.Writer, cwLogsGetter CWLogService, opts ...cloudwatchlogs.GetLogEventsOpts) error {
	logEventsOutput, err := cwLogsGetter.TaskLogEvents(ew.GroupName, ew.lastEventTimestampByLogGroup, opts...)
	if err != nil {
		return fmt.Errorf("get task log events: %w", err)
	}
	for _, event := range logEventsOutput.Events {
		if _, err := fmt.Fprintf(w, event.HumanString()); err != nil {
			return fmt.Errorf("write log event: %w", err)
		}
	}
	ew.lastEventTimestampByLogGroup = logEventsOutput.LastEventTime
	return nil
}
func (ew *EventsWriter) stopped(describer TaskDescriber) (bool, error){
	taskARNs := make([]string, len(ew.runningTasks))
	for idx, task := range ew.runningTasks {
		taskARNs[idx] = task.TaskARN
	}

	// NOTE: all tasks are deployed to the same cluster and there are at least one tasks being deployed
	cluster := ew.runningTasks[0].ClusterARN

	tasksResp, err := describer.DescribeTasks(cluster, taskARNs)
	if err != nil {
		return false, fmt.Errorf("describe tasks: %w", err)
	}

	ew.runningTasks = nil
	stopped := true
	for _, t := range tasksResp {
		if *t.LastStatus != ecs.DesiredStatusStopped {
			stopped = false
			ew.runningTasks = append(ew.runningTasks, &Task{
				TaskARN: *t.TaskArn,
			})
		}
	}
	return stopped, nil
}

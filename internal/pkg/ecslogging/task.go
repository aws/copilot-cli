// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package ecslogging contains utility functions for ECS logging.
package ecslogging

import (
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/task"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
)

const (
	numCWLogsCallsPerRound = 10
	fmtTaskLogGroupName    = "/copilot/%s"
	// e.g., copilot-task/python/4f8243e83f8a4bdaa7587fa1eaff2ea3
	fmtTaskLogStreamName = "copilot-task/%s/%s"
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
}

// NewTaskClient returns a TaskClient that can retrieve logs from the given tasks under the groupName.
func NewTaskClient(sess *session.Session, groupName string, tasks []*task.Task) *TaskClient {
	return &TaskClient{
		GroupName: groupName,
		Tasks:     tasks,

		Describer:    ecs.New(sess),
		EventsLogger: cloudwatchlogs.New(sess),
		Writer:       log.OutputWriter,
	}
}

// WriteEventsUntilStopped writes tasks' events to a writer until all tasks have stopped.
func (t *TaskClient) WriteEventsUntilStopped() error {
	in := cloudwatchlogs.LogEventsOpts{
		LogGroup: fmt.Sprintf(fmtTaskLogGroupName, t.GroupName),
	}
	for {
		logStreams, err := t.logStreamNamesFromTasks(t.Tasks)
		if err != nil {
			return err
		}
		in.LogStreams = logStreams
		for i := 0; i < numCWLogsCallsPerRound; i++ {
			logEventsOutput, err := t.EventsLogger.LogEvents(in)
			if err != nil {
				return fmt.Errorf("get task log events: %w", err)
			}
			if err := WriteHumanLogs(t.Writer, cwEventsToHumanJSONStringers(logEventsOutput.Events)); err != nil {
				return fmt.Errorf("write log event: %w", err)
			}
			in.StreamLastEventTime = logEventsOutput.StreamLastEventTime

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
	taskARNs := make([]string, len(t.Tasks))
	for idx, task := range t.Tasks {
		taskARNs[idx] = task.TaskARN
	}

	// NOTE: all tasks are deployed to the same cluster and there are at least one tasks being deployed
	cluster := t.Tasks[0].ClusterARN

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
	t.Tasks = runningTasks
	return stopped, nil
}

func (t *TaskClient) logStreamNamesFromTasks(tasks []*task.Task) ([]string, error) {
	var logStreamNames []string
	for _, task := range tasks {
		id, err := ecs.TaskID(task.TaskARN)
		if err != nil {
			return nil, fmt.Errorf("parse task ID from ARN %s", task.TaskARN)
		}
		logStreamNames = append(logStreamNames, fmt.Sprintf(fmtTaskLogStreamName, t.GroupName, id))
	}
	return logStreamNames, nil
}

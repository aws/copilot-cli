// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
)

const (
	numCWLogsCallsPerRound = 10
	sleepDuration          = 1 * time.Second
)

// TasksDescriber describes ECS tasks.
type TasksDescriber interface {
	DescribeTasks(cluster string, taskARNs []string) ([]*ecs.Task, error)
}

// EventsLogger gets a log group's log events.
type EventsLogger interface {
	TaskLogEvents(follow bool, opts ...cloudwatchlogs.GetLogEventsOpts) error
}

type stopResult struct {
	stop bool
	err  error
}

// EventsWriter represents a writer that writes tasks' log events to a writer.
type EventsWriter struct {
	GroupName string
	Tasks     []*Task

	Writer       io.Writer
	EventsLogger EventsLogger
	Describer    TasksDescriber

	// Fields that are private that get modified each time
	lastEventTimestampByLogGroup map[string]int64
	runningTasks                 []*Task
}

// WriteEventsUntilStopped writes tasks' events to a writer until all tasks have stopped.
func (ew *EventsWriter) WriteEventsUntilStopped() error {
	startTime := earliestStartTime(ew.Tasks)
	ew.runningTasks = ew.Tasks
	errCh := make(chan error)
	stopCh := make(chan stopResult)
	waitGroup := &sync.WaitGroup{}
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		errCh <- ew.EventsLogger.TaskLogEvents(true, cloudwatchlogs.WithStartTime(aws.TimeUnixMilli(startTime)))
	}(waitGroup)
	waitGroup.Add(1)
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		for {
			stopResult := ew.allTasksStopped()
			if stopResult.err != nil || stopResult.stop {
				stopCh <- stopResult
				errCh <- nil
				break
			}
			time.Sleep(sleepDuration)
		}
		close(stopCh)
	}(waitGroup)
	go func() {
		waitGroup.Wait()
		close(errCh)
	}()
	for {
		select {
		case err := <-errCh:
			if err != nil {
				return fmt.Errorf("write task log events: %w", err)
			}
		case stopResult := <-stopCh:
			if stopResult.err != nil {
				return stopResult.err
			}
			if stopResult.stop {
				return nil
			}
		}
	}
}

func (ew *EventsWriter) allTasksStopped() stopResult {
	taskARNs := make([]string, len(ew.runningTasks))
	for idx, task := range ew.runningTasks {
		taskARNs[idx] = task.TaskARN
	}

	// NOTE: all tasks are deployed to the same cluster and there are at least one tasks being deployed
	cluster := ew.runningTasks[0].ClusterARN

	tasksResp, err := ew.Describer.DescribeTasks(cluster, taskARNs)
	if err != nil {
		return stopResult{
			stop: false,
			err:  fmt.Errorf("describe tasks: %w", err),
		}
	}

	stopped := true
	var runningTasks []*Task
	for _, t := range tasksResp {
		if *t.LastStatus != ecs.DesiredStatusStopped {
			stopped = false
			runningTasks = append(runningTasks, &Task{
				ClusterARN: *t.ClusterArn,
				TaskARN:    *t.TaskArn,
			})
		}
	}
	ew.runningTasks = runningTasks
	return stopResult{
		stop: stopped,
	}
}

// The `StartedAt` field for the tasks shouldn't be nil.
func earliestStartTime(tasks []*Task) time.Time {
	earliest := *tasks[0].StartedAt
	for _, task := range tasks {
		if task.StartedAt.Before(earliest) {
			earliest = *task.StartedAt
		}
	}
	return earliest
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package logging contains utility functions for ECS logging.
package logging

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/apprunner"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
)

const (
	defaultServiceLogsLimit = 10

	fmtWkldLogGroupName         = "/copilot/%s-%s-%s"
	wkldLogStreamPrefix         = "copilot"
	stateMachineLogStreamPrefix = "states"
)

type logGetter interface {
	LogEvents(opts cloudwatchlogs.LogEventsOpts) (*cloudwatchlogs.LogEventsOutput, error)
}

// NewWorkloadLoggerOpts contains fields that initiate workloadLogger struct.
type NewWorkloadLoggerOpts struct {
	App         string
	Env         string
	Name        string
	Sess        *session.Session
	LogGroup    string
	ConfigStore describe.ConfigStoreSvc
}

// newWorkloadLogger returns a workloadLogger for the svc service under env and app.
// The logging client is initialized from the given sess session.
func newWorkloadLogger(opts *NewWorkloadLoggerOpts) (*workloadLogger, error) {
	logGroup := fmt.Sprintf(fmtWkldLogGroupName, opts.App, opts.Env, opts.Name)
	if opts.LogGroup != "" {
		logGroup = opts.LogGroup
	}
	return &workloadLogger{
		name:         opts.Name,
		logGroupName: logGroup,
		eventsGetter: cloudwatchlogs.New(opts.Sess),
		w:            log.OutputWriter,
		now:          time.Now,
	}, nil
}

type workloadLogger struct {
	name         string
	logGroupName string
	eventsGetter logGetter
	w            io.Writer

	now func() time.Time
}

// WriteLogEvents writes service logs.
func (s *workloadLogger) writeEventLogs(logEventsOpts cloudwatchlogs.LogEventsOpts, onEvent func(io.Writer, []HumanJSONStringer) error, follow bool) error {
	for {
		logEventsOutput, err := s.eventsGetter.LogEvents(logEventsOpts)
		if err != nil {
			return fmt.Errorf("get log events for log group %s: %w", logEventsOpts.LogGroup, err)
		}
		if err := onEvent(s.w, cwEventsToHumanJSONStringers(logEventsOutput.Events)); err != nil {
			return err
		}
		if !follow {
			return nil
		}
		// for unit test.
		if logEventsOutput.StreamLastEventTime == nil {
			return nil
		}
		logEventsOpts.StreamLastEventTime = logEventsOutput.StreamLastEventTime
		time.Sleep(cloudwatchlogs.SleepDuration)
	}
}

func (s *workloadLogger) ecsLogStreamPrefixes(taskIDs []string, container string) []string {
	// By default, we only want logs from copilot task log streams.
	// This filters out log stream not starting with `copilot/`, or `copilot/datadog` if container is set.
	if len(taskIDs) == 0 {
		return []string{fmt.Sprintf("%s/%s", wkldLogStreamPrefix, container)}
	}
	var logStreamPrefixes []string
	if container == "" {
		container = s.name
	}
	for _, taskID := range taskIDs {
		prefix := fmt.Sprintf("%s/%s/%s", wkldLogStreamPrefix, container, taskID) // Example: copilot/sidecar/1111 or copilot/web/1111
		logStreamPrefixes = append(logStreamPrefixes, prefix)
	}
	return logStreamPrefixes
}

// NewECSServiceClient returns an ECSServiceClient for the svc service under env and app.
func NewECSServiceClient(opts *NewWorkloadLoggerOpts) (*ECSServiceLogger, error) {
	logger, err := newWorkloadLogger(opts)
	if err != nil {
		return nil, err
	}
	return &ECSServiceLogger{
		workloadLogger: logger,
	}, nil
}

// ECSServiceLogger retrieves the logs of an Amazon ECS service.
type ECSServiceLogger struct {
	*workloadLogger
}

// WriteLogEvents writes service logs.
func (s *ECSServiceLogger) WriteLogEvents(opts WriteLogEventsOpts) error {
	logEventsOpts := cloudwatchlogs.LogEventsOpts{
		LogGroup:               s.logGroupName,
		Limit:                  opts.limit(),
		StartTime:              opts.startTime(s.now),
		EndTime:                opts.EndTime,
		StreamLastEventTime:    nil,
		LogStreamLimit:         opts.LogStreamLimit,
		LogStreamPrefixFilters: s.logStreamPrefixes(opts.TaskIDs, opts.ContainerName),
	}
	return s.workloadLogger.writeEventLogs(logEventsOpts, opts.OnEvents, opts.Follow)
}

func (s *ECSServiceLogger) logStreamPrefixes(taskIDs []string, container string) []string {
	return s.ecsLogStreamPrefixes(taskIDs, container)
}

// NewAppRunnerServiceLogger returns an AppRunnerServiceLogger for the svc service under env and app.
func NewAppRunnerServiceLogger(opts *NewWorkloadLoggerOpts) (*AppRunnerServiceLogger, error) {
	logger, err := newWorkloadLogger(opts)
	if err != nil {
		return nil, err
	}
	serviceDescriber, err := describe.NewRDWebServiceDescriber(describe.NewServiceConfig{
		App: opts.App,
		Svc: opts.Name,

		ConfigStore: opts.ConfigStore,
	})
	if err != nil {
		return nil, err
	}
	serviceArn, err := serviceDescriber.ServiceARN(opts.Env)
	if err != nil {
		return nil, err
	}
	logGroup := opts.LogGroup
	switch strings.ToLower(logGroup) {
	case "system":
		logGroup, err = apprunner.SystemLogGroupName(serviceArn)
		if err != nil {
			return nil, fmt.Errorf("get system log group name: %w", err)
		}
	case "":
		logGroup, err = apprunner.LogGroupName(serviceArn)
		if err != nil {
			return nil, fmt.Errorf("get log group name: %w", err)
		}
	}

	logger.logGroupName = logGroup
	return &AppRunnerServiceLogger{
		workloadLogger: logger,
	}, nil
}

// AppRunnerServiceLogger retrieves the logs of an AppRunner service.
type AppRunnerServiceLogger struct {
	*workloadLogger
}

// WriteLogEvents writes service logs.
func (s *AppRunnerServiceLogger) WriteLogEvents(opts WriteLogEventsOpts) error {
	logEventsOpts := cloudwatchlogs.LogEventsOpts{
		LogGroup:            s.logGroupName,
		Limit:               opts.limit(),
		StartTime:           opts.startTime(s.now),
		EndTime:             opts.EndTime,
		StreamLastEventTime: nil,
		LogStreamLimit:      opts.LogStreamLimit,
	}
	return s.workloadLogger.writeEventLogs(logEventsOpts, opts.OnEvents, opts.Follow)
}

// NewJobLogger returns an JobLogger for the job under env and app.
func NewJobLogger(opts *NewWorkloadLoggerOpts) (*JobLogger, error) {
	logger, err := newWorkloadLogger(opts)
	if err != nil {
		return nil, err
	}
	return &JobLogger{
		workloadLogger: logger,
	}, nil
}

// JobLogger retrieves the logs of a job.
type JobLogger struct {
	*workloadLogger
}

// WriteLogEvents writes job logs.
func (s *JobLogger) WriteLogEvents(opts WriteLogEventsOpts) error {
	logStreamLimit := opts.LogStreamLimit
	if opts.IncludeStateMachineLogs {
		logStreamLimit *= 2
	}
	logEventsOpts := cloudwatchlogs.LogEventsOpts{
		LogGroup:               s.logGroupName,
		Limit:                  opts.limit(),
		StartTime:              opts.startTime(s.now),
		EndTime:                opts.EndTime,
		StreamLastEventTime:    nil,
		LogStreamLimit:         logStreamLimit,
		LogStreamPrefixFilters: s.logStreamPrefixes(opts.TaskIDs, opts.IncludeStateMachineLogs),
	}
	return s.workloadLogger.writeEventLogs(logEventsOpts, opts.OnEvents, opts.Follow)
}

//  The log stream prefixes for a job should be:
// 1. copilot/;
// 2. copilot/, states;
// 3. copilot/query/taskID where query is the job's name, thus the main container's name.
func (s *JobLogger) logStreamPrefixes(taskIDs []string, includeStateMachineLogs bool) []string {
	// `includeStateMachineLogs` is mutually exclusive with specific task IDs and only used for jobs. Therefore, we
	// need to grab all recent log streams with no prefix filtering.
	if includeStateMachineLogs {
		return []string{fmt.Sprintf("%s/", wkldLogStreamPrefix), stateMachineLogStreamPrefix}
	}
	return s.ecsLogStreamPrefixes(taskIDs, "")
}

// WriteLogEventsOpts wraps the parameters to call WriteLogEvents.
type WriteLogEventsOpts struct {
	Follow    bool
	Limit     *int64
	StartTime *int64
	EndTime   *int64
	// OnEvents is a handler that's invoked when logs are retrieved from the service.
	OnEvents func(w io.Writer, logs []HumanJSONStringer) error

	// Job specific options.
	IncludeStateMachineLogs bool
	// LogStreamLimit is an optional parameter for jobs and tasks to speed up CW queries
	// involving multiple log streams.
	LogStreamLimit int

	// ECS specific options.
	ContainerName string
	TaskIDs       []string
}

func (o WriteLogEventsOpts) limit() *int64 {
	if o.Limit != nil {
		return o.Limit
	}
	if o.hasTimeFilters() {
		// If time filtering is set, then set limit to be maximum number.
		// https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_GetLogEvents.html#CWL-GetLogEvents-request-limit
		return nil
	}
	if o.hasLogStreamLimit() {
		// If log stream limit is set and no log event limit is set, then set limit to maximum.
		return nil
	}
	return aws.Int64(defaultServiceLogsLimit)
}

func (o WriteLogEventsOpts) hasLogStreamLimit() bool {
	return o.LogStreamLimit != 0
}

func (o WriteLogEventsOpts) startTime(now func() time.Time) *int64 {
	if o.StartTime != nil {
		return o.StartTime
	}
	if o.Follow {
		// Start following log events from current timestamp.
		return aws.Int64(now().UnixMilli())
	}
	return nil
}

func (o WriteLogEventsOpts) hasTimeFilters() bool {
	return o.Follow || o.StartTime != nil || o.EndTime != nil
}

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

type serviceARNGetter interface {
	ServiceARN(env string) (string, error)
}

// NewWorkloadLoggerOpts contains fields that initiate workloadLogger struct.
type NewWorkloadLoggerOpts struct {
	App  string
	Env  string
	Name string
	Sess *session.Session
}

// newWorkloadLogger returns a workloadLogger for the service under env and app.
// The logging client is initialized from the given sess session.
func newWorkloadLogger(opts *NewWorkloadLoggerOpts) *workloadLogger {
	return &workloadLogger{
		app:          opts.App,
		env:          opts.Env,
		name:         opts.Name,
		eventsGetter: cloudwatchlogs.New(opts.Sess),
		w:            log.OutputWriter,
		now:          time.Now,
	}
}

type workloadLogger struct {
	app  string
	env  string
	name string

	eventsGetter logGetter
	w            io.Writer
	now          func() time.Time
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
		// For unit test.
		if logEventsOutput.StreamLastEventTime == nil {
			return nil
		}
		logEventsOpts.StreamLastEventTime = logEventsOutput.StreamLastEventTime
		time.Sleep(cloudwatchlogs.SleepDuration)
	}
}

func ecsLogStreamPrefixes(taskIDs []string, service, container string) []string {
	// By default, we only want logs from copilot task log streams.
	// This filters out log stream not starting with `copilot/`, or `copilot/datadog` if container is set.
	if len(taskIDs) == 0 {
		return []string{fmt.Sprintf("%s/%s", wkldLogStreamPrefix, container)}
	}
	var logStreamPrefixes []string
	if container == "" {
		container = service
	}
	for _, taskID := range taskIDs {
		prefix := fmt.Sprintf("%s/%s/%s", wkldLogStreamPrefix, container, taskID) // Example: copilot/sidecar/1111 or copilot/web/1111
		logStreamPrefixes = append(logStreamPrefixes, prefix)
	}
	return logStreamPrefixes
}

// NewECSServiceClient returns an ECSServiceClient for the service under env and app.
func NewECSServiceClient(opts *NewWorkloadLoggerOpts) *ECSServiceLogger {
	return &ECSServiceLogger{
		workloadLogger: newWorkloadLogger(opts),
	}
}

// ECSServiceLogger retrieves the logs of an Amazon ECS service.
type ECSServiceLogger struct {
	*workloadLogger
}

// WriteLogEvents writes service logs.
func (s *ECSServiceLogger) WriteLogEvents(opts WriteLogEventsOpts) error {
	logGroup := fmt.Sprintf(fmtWkldLogGroupName, s.app, s.env, s.name)
	if opts.LogGroup != "" {
		logGroup = opts.LogGroup
	}
	logEventsOpts := cloudwatchlogs.LogEventsOpts{
		LogGroup:               logGroup,
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
	return ecsLogStreamPrefixes(taskIDs, s.name, container)
}

// NewAppRunnerServiceLoggerOpts contains fields that initiate AppRunnerServiceLoggerOpts struct.
type NewAppRunnerServiceLoggerOpts struct {
	*NewWorkloadLoggerOpts
	ConfigStore describe.ConfigStoreSvc
}

// NewAppRunnerServiceLogger returns an AppRunnerServiceLogger for the service under env and app.
func NewAppRunnerServiceLogger(opts *NewAppRunnerServiceLoggerOpts) (*AppRunnerServiceLogger, error) {
	serviceDescriber, err := describe.NewRDWebServiceDescriber(describe.NewServiceConfig{
		App:         opts.App,
		Svc:         opts.Name,
		ConfigStore: opts.ConfigStore,
	})
	if err != nil {
		return nil, err
	}
	return &AppRunnerServiceLogger{
		workloadLogger:   newWorkloadLogger(opts.NewWorkloadLoggerOpts),
		serviceARNGetter: serviceDescriber,
	}, nil
}

// AppRunnerServiceLogger retrieves the logs of an AppRunner service.
type AppRunnerServiceLogger struct {
	*workloadLogger
	serviceARNGetter serviceARNGetter
}

// WriteLogEvents writes service logs.
func (s *AppRunnerServiceLogger) WriteLogEvents(opts WriteLogEventsOpts) error {
	var logGroup string
	switch strings.ToLower(opts.LogGroup) {
	case "system":
		serviceArn, err := s.serviceARNGetter.ServiceARN(s.env)
		if err != nil {
			return fmt.Errorf("get service ARN for %s: %w", s.name, err)
		}
		logGroup, err = apprunner.SystemLogGroupName(serviceArn)
		if err != nil {
			return fmt.Errorf("get system log group name: %w", err)
		}
	case "":
		serviceArn, err := s.serviceARNGetter.ServiceARN(s.env)
		if err != nil {
			return fmt.Errorf("get service ARN for %s: %w", s.name, err)
		}
		logGroup, err = apprunner.LogGroupName(serviceArn)
		if err != nil {
			return fmt.Errorf("get log group name: %w", err)
		}
	default:
		logGroup = opts.LogGroup
	}
	logEventsOpts := cloudwatchlogs.LogEventsOpts{
		LogGroup:            logGroup,
		Limit:               opts.limit(),
		StartTime:           opts.startTime(s.now),
		EndTime:             opts.EndTime,
		StreamLastEventTime: nil,
		LogStreamLimit:      opts.LogStreamLimit,
	}
	return s.workloadLogger.writeEventLogs(logEventsOpts, opts.OnEvents, opts.Follow)
}

// NewJobLogger returns an JobLogger for the job under env and app.
func NewJobLogger(opts *NewWorkloadLoggerOpts) *JobLogger {
	return &JobLogger{
		workloadLogger: newWorkloadLogger(opts),
	}
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
	logGroup := fmt.Sprintf(fmtWkldLogGroupName, s.app, s.env, s.name)
	if opts.LogGroup != "" {
		logGroup = opts.LogGroup
	}
	logEventsOpts := cloudwatchlogs.LogEventsOpts{
		LogGroup:               logGroup,
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
	return ecsLogStreamPrefixes(taskIDs, s.name, "")
}

// WriteLogEventsOpts wraps the parameters to call WriteLogEvents.
type WriteLogEventsOpts struct {
	Follow    bool
	Limit     *int64
	StartTime *int64
	EndTime   *int64
	// OnEvents is a handler that's invoked when logs are retrieved from the service.
	OnEvents func(w io.Writer, logs []HumanJSONStringer) error
	LogGroup string

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

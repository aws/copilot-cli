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
	"github.com/aws/copilot-cli/internal/pkg/manifest"
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

// WorkloadClient retrieves the logs of an Amazon ECS or AppRunner service.
type WorkloadClient struct {
	name         string
	logGroupName string
	isECS        bool
	eventsGetter logGetter
	w            io.Writer

	now func() time.Time
}

// WriteLogEventsOpts wraps the parameters to call WriteLogEvents.
type WriteLogEventsOpts struct {
	Follow    bool
	Limit     *int64
	StartTime *int64
	EndTime   *int64
	TaskIDs   []string
	// OnEvents is a handler that's invoked when logs are retrieved from the service.
	OnEvents func(w io.Writer, logs []HumanJSONStringer) error
	// LogStreamLimit is an optional parameter for jobs and tasks to speed up CW queries
	// involving multiple log streams.
	LogStreamLimit          int
	IncludeStateMachineLogs bool
	ContainerName           string
}

// NewWorkloadLogsConfig contains fields that initiates WorkloadClient struct.
type NewWorkloadLogsConfig struct {
	App         string
	Env         string
	Name        string
	Sess        *session.Session
	LogGroup    string
	WkldType    string
	TaskIDs     []string
	ConfigStore describe.ConfigStoreSvc
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

// NewWorkloadClient returns a WorkloadClient for the svc service under env and app.
// The logging client is initialized from the given sess session.
func NewWorkloadClient(opts *NewWorkloadLogsConfig) (*WorkloadClient, error) {
	if opts.WkldType == manifest.RequestDrivenWebServiceType {
		return newAppRunnerServiceClient(opts)
	}
	logGroup := fmt.Sprintf(fmtWkldLogGroupName, opts.App, opts.Env, opts.Name)
	if opts.LogGroup != "" {
		logGroup = opts.LogGroup
	}
	return &WorkloadClient{
		name:         opts.Name,
		logGroupName: logGroup,
		isECS:        true,
		eventsGetter: cloudwatchlogs.New(opts.Sess),
		w:            log.OutputWriter,
		now:          time.Now,
	}, nil
}

func newAppRunnerServiceClient(opts *NewWorkloadLogsConfig) (*WorkloadClient, error) {
	if opts.TaskIDs != nil {
		return nil, fmt.Errorf("cannot use --tasks for App Runner service logs")
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
	return &WorkloadClient{
		name:         opts.Name,
		logGroupName: logGroup,
		eventsGetter: cloudwatchlogs.New(opts.Sess),
		w:            log.OutputWriter,
		now:          time.Now,
	}, nil
}

// WriteLogEvents writes service logs.
func (s *WorkloadClient) WriteLogEvents(opts WriteLogEventsOpts) error {
	logEventsOpts := cloudwatchlogs.LogEventsOpts{
		LogGroup:            s.logGroupName,
		Limit:               opts.limit(),
		StartTime:           opts.startTime(s.now),
		EndTime:             opts.EndTime,
		StreamLastEventTime: nil,
	}
	logStreamLimit := opts.LogStreamLimit
	if opts.IncludeStateMachineLogs {
		logStreamLimit *= 2
	}
	// TODO(lou1415926): there should be a separate logging client for ECS services
	// and App Runner services. This `if` check is only ever true for ECS services.
	// Refactor so that there are separate client for rdws, job and other services.
	// As well as the `isECS` variable in `WorkloadClient`.
	if s.isECS {
		logEventsOpts.LogStreamPrefixFilters = s.logStreams(opts.TaskIDs, opts.IncludeStateMachineLogs, opts.ContainerName)
	}
	logEventsOpts.LogStreamLimit = logStreamLimit

	for {
		logEventsOutput, err := s.eventsGetter.LogEvents(logEventsOpts)
		if err != nil {
			return fmt.Errorf("get log events for log group %s: %w", s.logGroupName, err)
		}
		if err := opts.OnEvents(s.w, cwEventsToHumanJSONStringers(logEventsOutput.Events)); err != nil {
			return err
		}
		if !opts.Follow {
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

func (s *WorkloadClient) logStreams(taskIDs []string, includeStateMachineLogs bool, container string) []string {
	// By default, we only want logs from copilot task log streams.
	// This filters out log streams not starting with `copilot/`, or `copilot/mysidecar` if container is set.
	switch {
	case includeStateMachineLogs:
		// includeStateMachineLogs is mutually exclusive with specific task IDs and only used for jobs. Therefore, we
		// need to grab all recent log streams with no prefix filtering.
		return []string{fmt.Sprintf("%s/%s", wkldLogStreamPrefix, container), stateMachineLogStreamPrefix}
	case len(taskIDs) == 0:
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

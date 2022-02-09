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

	fmtSvclogGroupName    = "/copilot/%s-%s-%s"
	fmtSvcLogStreamPrefix = "copilot/%s"
)

type logGetter interface {
	LogEvents(opts cloudwatchlogs.LogEventsOpts) (*cloudwatchlogs.LogEventsOutput, error)
}

// ServiceClient retrieves the logs of an Amazon ECS or AppRunner service.
type ServiceClient struct {
	logGroupName        string
	logStreamNamePrefix string
	eventsGetter        logGetter
	w                   io.Writer

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
}

// NewServiceLogsConfig contains fields that initiates ServiceClient struct.
type NewServiceLogsConfig struct {
	App         string
	Env         string
	Svc         string
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
	return aws.Int64(defaultServiceLogsLimit)
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

// NewServiceClient returns a ServiceClient for the svc service under env and app.
// The logging client is initialized from the given sess session.
func NewServiceClient(opts *NewServiceLogsConfig) (*ServiceClient, error) {
	if opts.WkldType == manifest.RequestDrivenWebServiceType {
		return newAppRunnerServiceClient(opts)
	}
	logGroup := fmt.Sprintf(fmtSvclogGroupName, opts.App, opts.Env, opts.Svc)
	if opts.LogGroup != "" {
		logGroup = opts.LogGroup
	}
	return &ServiceClient{
		logGroupName:        logGroup,
		logStreamNamePrefix: fmt.Sprintf(fmtSvcLogStreamPrefix, opts.Svc),
		eventsGetter:        cloudwatchlogs.New(opts.Sess),
		w:                   log.OutputWriter,
		now:                 time.Now,
	}, nil
}

func newAppRunnerServiceClient(opts *NewServiceLogsConfig) (*ServiceClient, error) {
	if opts.TaskIDs != nil {
		return nil, fmt.Errorf("cannot use --tasks for App Runner service logs")
	}
	serviceDescriber, err := describe.NewAppRunnerServiceDescriber(describe.NewServiceConfig{
		App: opts.App,
		Env: opts.Env,
		Svc: opts.Svc,

		ConfigStore: opts.ConfigStore,
	})
	if err != nil {
		return nil, err
	}
	serviceArn, err := serviceDescriber.ServiceARN()
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
	return &ServiceClient{
		logGroupName: logGroup,
		eventsGetter: cloudwatchlogs.New(opts.Sess),
		w:            log.OutputWriter,
		now:          time.Now,
	}, nil
}

// WriteLogEvents writes service logs.
func (s *ServiceClient) WriteLogEvents(opts WriteLogEventsOpts) error {
	logEventsOpts := cloudwatchlogs.LogEventsOpts{
		LogGroup:  s.logGroupName,
		Limit:     opts.limit(),
		EndTime:   opts.EndTime,
		StartTime: opts.startTime(s.now),
	}
	if opts.TaskIDs != nil {
		logEventsOpts.LogStreams = s.logStreams(opts.TaskIDs)
	}
	for {
		logEventsOutput, err := s.eventsGetter.LogEvents(logEventsOpts)
		if err != nil {
			return fmt.Errorf("get task log events for log group %s: %w", s.logGroupName, err)
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

func (s *ServiceClient) logStreams(taskIDs []string) (logStreamName []string) {
	for _, taskID := range taskIDs {
		logStreamName = append(logStreamName, fmt.Sprintf("%s/%s", s.logStreamNamePrefix, taskID))
	}
	return
}

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
	"github.com/aws/copilot-cli/internal/pkg/term/log"
)

const (
	logGroupNamePattern = "/copilot/%s-%s-%s"
)

type logGetter interface {
	LogEvents(opts cloudwatchlogs.LogEventsOpts) (*cloudwatchlogs.LogEventsOutput, error)
}

// ServiceClient retrieves the logs of an Amazon ECS service.
type ServiceClient struct {
	logGroupName string
	eventsGetter logGetter
	w            io.Writer
}

// WriteLogEventsOpts wraps the parameters to call WriteLogEvents.
type WriteLogEventsOpts struct {
	Follow    bool
	Limit     int
	StartTime *int64
	EndTime   *int64
	// OnEvents is a handler that's invoked when logs are retrieved from the service.
	OnEvents func(w io.Writer, logs []HumanJSONStringer) error
}

// NewServiceClient returns a ServiceClient for the svc service under env and app.
// The logging client is initialized from the given sess session.
func NewServiceClient(sess *session.Session, app, env, svc string) *ServiceClient {
	return &ServiceClient{
		logGroupName: fmt.Sprintf(logGroupNamePattern, app, env, svc),
		eventsGetter: cloudwatchlogs.New(sess),
		w:            log.OutputWriter,
	}
}

// WriteLogEvents writes service logs.
func (s *ServiceClient) WriteLogEvents(opts WriteLogEventsOpts) error {
	logEventsOpts := cloudwatchlogs.LogEventsOpts{
		LogGroup:  s.logGroupName,
		Limit:     aws.Int64(int64(opts.Limit)),
		EndTime:   opts.EndTime,
		StartTime: opts.StartTime,
		// TODO: Add log filtering by task ID.
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

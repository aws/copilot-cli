// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ecslogging

import (
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
)

const (
	logGroupNamePattern = "/copilot/%s-%s-%s"
)

type logGetter interface {
	LogEvents(logGroupName string,
		streamLastEventTime map[string]int64,
		opts ...cloudwatchlogs.GetLogEventsOpts) (*cloudwatchlogs.LogEventsOutput, error)
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
	StartTime int64
	EndTime   int64
	// OnEvents is a handler that's invoked when logs are retrieved from the service.
	OnEvents func(w io.Writer, logs []HumanJSONStringer) error
}

// NewServiceClient returns a ServiceClient configured against the input.
func NewServiceClient(sess *session.Session, appName, envName, svcName string) *ServiceClient {
	return &ServiceClient{
		logGroupName: fmt.Sprintf(logGroupNamePattern, appName, envName, svcName),
		eventsGetter: cloudwatchlogs.New(sess),
		w:            log.OutputWriter,
	}
}

// WriteLogEvents writes service logs.
func (s *ServiceClient) WriteLogEvents(opts WriteLogEventsOpts) error {
	logEventsOutput := &cloudwatchlogs.LogEventsOutput{
		LastEventTime: make(map[string]int64),
	}
	var err error
	for {
		logEventsOutput, err = s.eventsGetter.LogEvents(s.logGroupName, logEventsOutput.LastEventTime, opts.generateGetLogEventOpts()...)
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
		if logEventsOutput.LastEventTime == nil {
			return nil
		}
		time.Sleep(cloudwatchlogs.SleepDuration)
	}
}

func (w *WriteLogEventsOpts) generateGetLogEventOpts() []cloudwatchlogs.GetLogEventsOpts {
	opts := []cloudwatchlogs.GetLogEventsOpts{
		cloudwatchlogs.WithLimit(w.Limit),
	}
	if w.StartTime != 0 {
		opts = append(opts, cloudwatchlogs.WithStartTime(w.StartTime))
	}
	if w.EndTime != 0 {
		opts = append(opts, cloudwatchlogs.WithEndTime(w.EndTime))
	}
	return opts
}

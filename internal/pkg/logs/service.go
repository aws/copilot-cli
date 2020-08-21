// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"fmt"
	"io"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
)

const (
	logGroupNamePattern = "/copilot/%s-%s-%s"
)

type logGetter interface {
	TaskLogEvents(logGroupName string,
		streamLastEventTime map[string]int64,
		opts ...cloudwatchlogs.GetLogEventsOpts) (*cloudwatchlogs.LogEventsOutput, error)
}

// ServiceLogs wraps service that writes service log events to a writer.
type ServiceLogs struct {
	logGroupName string
	eventsGetter logGetter
	w            io.Writer
}

// WriteLogEventsOpts wraps the parameters to call WriteLogEvents.
type WriteLogEventsOpts struct {
	Follow             bool
	OutputLogs         func(w io.Writer, logs []*cloudwatchlogs.Event) error
	GetLogEventConfigs []cloudwatchlogs.GetLogEventsOpts
}

// NewServiceLogs returns a ServiceLogs configured against the input.
func NewServiceLogs(appName, envName, svcName string) (*ServiceLogs, error) {
	configStore, err := config.NewStore()
	if err != nil {
		return nil, fmt.Errorf("connect to environment config store: %w", err)
	}
	env, err := configStore.GetEnvironment(appName, envName)
	if err != nil {
		return nil, fmt.Errorf("get environment: %w", err)
	}
	sess, err := sessions.NewProvider().FromRole(env.ManagerRoleARN, env.Region)
	if err != nil {
		return nil, err
	}
	return &ServiceLogs{
		logGroupName: fmt.Sprintf(logGroupNamePattern, appName, envName, svcName),
		eventsGetter: cloudwatchlogs.New(sess),
		w:            log.OutputWriter,
	}, nil
}

// WriteLogEvents writes service logs.
func (s *ServiceLogs) WriteLogEvents(opts WriteLogEventsOpts) error {
	logEventsOutput := &cloudwatchlogs.LogEventsOutput{
		LastEventTime: make(map[string]int64),
	}
	var err error
	for {
		logEventsOutput, err = s.eventsGetter.TaskLogEvents(s.logGroupName, logEventsOutput.LastEventTime, opts.GetLogEventConfigs...)
		if err != nil {
			return fmt.Errorf("get task log events for log group %s: %w", s.logGroupName, err)
		}
		if err := opts.OutputLogs(s.w, logEventsOutput.Events); err != nil {
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

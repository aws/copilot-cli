// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudwatchlogs provides a client to make API requests to Amazon CloudWatch Logs.
package cloudwatchlogs

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

const (
	// SleepDuration is the sleep time for making the next request for log events.
	SleepDuration = 1 * time.Second
)

var (
	fatalCodes   = []string{"FATA", "FATAL", "fatal", "ERR", "ERROR", "error"}
	warningCodes = []string{"WARN", "warn", "WARNING", "warning"}
)

type api interface {
	DescribeLogStreams(input *cloudwatchlogs.DescribeLogStreamsInput) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
	GetLogEvents(input *cloudwatchlogs.GetLogEventsInput) (*cloudwatchlogs.GetLogEventsOutput, error)
}

// CloudWatchLogs wraps an AWS Cloudwatch Logs client.
type CloudWatchLogs struct {
	client api
}

// LogEventsOutput contains the output for LogEvents
type LogEventsOutput struct {
	// Retrieved log events.
	Events []*Event
	// Timestamp for the last event
	StreamLastEventTime map[string]int64
}

// LogEventsOpts wraps the parameters to call LogEvents.
type LogEventsOpts struct {
	LogGroup            string
	LogStreams          []string // If nil, retrieve logs from all log streams.
	Limit               *int64
	StartTime           *int64
	EndTime             *int64
	StreamLastEventTime map[string]int64
}

// New returns a CloudWatchLogs configured against the input session.
func New(s *session.Session) *CloudWatchLogs {
	return &CloudWatchLogs{
		client: cloudwatchlogs.New(s),
	}
}

// logStreams returns all name of the log streams in a log group.
func (c *CloudWatchLogs) logStreams(logGroup string, logStreams ...string) ([]string, error) {
	resp, err := c.client.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String(logGroup),
		Descending:   aws.Bool(true),
		OrderBy:      aws.String(cloudwatchlogs.OrderByLastEventTime),
	})
	if err != nil {
		return nil, fmt.Errorf("describe log streams of log group %s: %w", logGroup, err)
	}
	if len(resp.LogStreams) == 0 {
		return nil, fmt.Errorf("no log stream found in log group %s", logGroup)
	}
	var logStreamNames []string
	for _, logStream := range resp.LogStreams {
		name := aws.StringValue(logStream.LogStreamName)
		if name == "" {
			continue
		}
		logStreamNames = append(logStreamNames, name)
	}
	if len(logStreams) != 0 {
		logStreamNames = filterStringSliceByPrefix(logStreamNames, logStreams)
	}
	return logStreamNames, nil
}

// LogEvents returns an array of Cloudwatch Logs events.
func (c *CloudWatchLogs) LogEvents(opts LogEventsOpts) (*LogEventsOutput, error) {
	var events []*Event
	// Set default value
	in := defaultGetLogEventsInput(opts)
	logStreams, err := c.logStreams(opts.LogGroup, opts.LogStreams...)
	if err != nil {
		return nil, err
	}
	streamLastEventTime := make(map[string]int64)
	for k, v := range opts.StreamLastEventTime {
		streamLastEventTime[k] = v
	}
	for _, logStream := range logStreams {
		// Set override value
		in.SetLogStreamName(logStream)
		if streamLastEventTime[logStream] != 0 {
			// If last event for this log stream exists, increment last log event timestamp
			// by one to get logs after the last event.
			in.SetStartTime(streamLastEventTime[logStream] + 1)
		}
		// TODO: https://github.com/aws/copilot-cli/pull/628#discussion_r374291068 and https://github.com/aws/copilot-cli/pull/628#discussion_r374294362
		resp, err := c.client.GetLogEvents(in)
		if err != nil {
			return nil, fmt.Errorf("get log events of %s/%s: %w", opts.LogGroup, logStream, err)
		}

		for _, event := range resp.Events {
			log := &Event{
				LogStreamName: logStream,
				IngestionTime: aws.Int64Value(event.IngestionTime),
				Message:       aws.StringValue(event.Message),
				Timestamp:     aws.Int64Value(event.Timestamp),
			}
			events = append(events, log)
		}
		if len(resp.Events) != 0 {
			streamLastEventTime[logStream] = *resp.Events[len(resp.Events)-1].Timestamp
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Timestamp < events[j].Timestamp })
	limit := int(aws.Int64Value(in.Limit))
	if limit != 0 {
		return &LogEventsOutput{
			Events:              truncateEvents(limit, events),
			StreamLastEventTime: streamLastEventTime,
		}, nil
	}
	return &LogEventsOutput{
		Events:              events,
		StreamLastEventTime: streamLastEventTime,
	}, nil
}

func truncateEvents(limit int, events []*Event) []*Event {
	if len(events) <= limit {
		return events
	}
	return events[len(events)-limit:] // Only grab the last N elements where N = limit
}

func defaultGetLogEventsInput(opts LogEventsOpts) *cloudwatchlogs.GetLogEventsInput {
	return &cloudwatchlogs.GetLogEventsInput{
		LogGroupName: aws.String(opts.LogGroup),
		StartTime:    opts.StartTime,
		EndTime:      opts.EndTime,
		Limit:        opts.Limit,
	}
}

// Example: if the prefixes is []string{"a"} and all is []string{"a", "b", "ab"}
// then it returns []string{"a", "ab"}.
func filterStringSliceByPrefix(all, prefixes []string) (res []string) {
	m := make(map[string]bool)
	for _, candidate := range all {
		for _, prefix := range prefixes {
			if strings.HasPrefix(candidate, prefix) {
				m[candidate] = true
			}
		}
	}
	for k := range m {
		res = append(res, k)
	}
	return
}

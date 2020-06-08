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
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

const (
	// SleepDuration is the sleep time for making the next request for log events.
	SleepDuration = 1 * time.Second

	logStreamNamePrefix = "copilot/"
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

// GetLogEventsOpts sets up optional parameters for LogEvents function.
type GetLogEventsOpts func(*cloudwatchlogs.GetLogEventsInput)

// WithLimit sets up limit for GetLogEventsInput
func WithLimit(limit int) GetLogEventsOpts {
	return func(in *cloudwatchlogs.GetLogEventsInput) {
		in.Limit = aws.Int64(int64(limit))
	}
}

// WithStartTime sets up startTime for GetLogEventsInput
func WithStartTime(startTime int64) GetLogEventsOpts {
	return func(in *cloudwatchlogs.GetLogEventsInput) {
		in.StartTime = aws.Int64(startTime)
	}
}

// WithEndTime sets up endTime for GetLogEventsInput
func WithEndTime(endTime int64) GetLogEventsOpts {
	return func(in *cloudwatchlogs.GetLogEventsInput) {
		in.EndTime = aws.Int64(endTime)
	}
}

// LogEventsOutput contains the output for LogEvents
type LogEventsOutput struct {
	// Retrieved log events.
	Events []*Event
	// Timestamp for the last event
	LastEventTime map[string]int64
}

// New returns a CloudWatchLogs configured against the input session.
func New(s *session.Session) *CloudWatchLogs {
	return &CloudWatchLogs{
		client: cloudwatchlogs.New(s),
	}
}

// logStreams returns all name of the log streams in a log group.
func (c *CloudWatchLogs) logStreams(logGroupName string) ([]*string, error) {
	resp, err := c.client.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String(logGroupName),
		Descending:   aws.Bool(true),
		OrderBy:      aws.String(cloudwatchlogs.OrderByLastEventTime),
	})
	if err != nil {
		return nil, fmt.Errorf("describe log streams of log group %s: %w", logGroupName, err)
	}
	if len(resp.LogStreams) == 0 {
		return nil, fmt.Errorf("no log stream found in log group %s", logGroupName)
	}
	logStreamNames := make([]*string, len(resp.LogStreams))
	for ind, logStream := range resp.LogStreams {
		logStreamNames[ind] = logStream.LogStreamName
	}
	return logStreamNames, nil
}

// TaskLogEvents returns an array of Cloudwatch Logs events.
func (c *CloudWatchLogs) TaskLogEvents(logGroupName string, streamLastEventTime map[string]int64, opts ...GetLogEventsOpts) (*LogEventsOutput, error) {
	var events []*Event
	var in *cloudwatchlogs.GetLogEventsInput
	logStreamNames, err := c.logStreams(logGroupName)
	if err != nil {
		return nil, err
	}
	for _, logStreamName := range logStreamNames {
		in = &cloudwatchlogs.GetLogEventsInput{
			LogGroupName:  aws.String(logGroupName),
			LogStreamName: logStreamName,
			Limit:         aws.Int64(10), // default to be 10
		}
		for _, opt := range opts {
			opt(in)
		}
		if streamLastEventTime[*logStreamName] != 0 {
			// If last event for this log stream exists, increment last log event timestamp
			// by one to get logs after the last event.
			in.SetStartTime(streamLastEventTime[*logStreamName] + 1)
		}
		// TODO: https://github.com/aws/amazon-ecs-cli-v2/pull/628#discussion_r374291068 and https://github.com/aws/amazon-ecs-cli-v2/pull/628#discussion_r374294362
		resp, err := c.client.GetLogEvents(in)
		if err != nil {
			return nil, fmt.Errorf("get log events of %s/%s: %w", logGroupName, *logStreamName, err)
		}

		for _, event := range resp.Events {
			log := &Event{
				LogStreamName: trimLogStreamName(*logStreamName),
				IngestionTime: aws.Int64Value(event.IngestionTime),
				Message:       aws.StringValue(event.Message),
				Timestamp:     aws.Int64Value(event.Timestamp),
			}
			events = append(events, log)
		}
		if len(resp.Events) != 0 {
			streamLastEventTime[*logStreamName] = *resp.Events[len(resp.Events)-1].Timestamp
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].Timestamp < events[j].Timestamp })
	var truncatedEvents []*Event
	if len(events) >= int(*in.Limit) {
		truncatedEvents = events[len(events)-int(*in.Limit):]
	} else {
		truncatedEvents = events
	}
	return &LogEventsOutput{
		Events:        truncatedEvents,
		LastEventTime: streamLastEventTime,
	}, nil
}

// LogGroupExists returns if a log group exists.
func (c *CloudWatchLogs) LogGroupExists(logGroupName string) (bool, error) {
	_, err := c.client.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
		LogGroupName: aws.String(logGroupName),
	})
	if err == nil {
		return true, nil
	}
	aerr, ok := err.(awserr.Error)
	if !ok {
		return false, err
	}
	if aerr.Code() != cloudwatchlogs.ErrCodeResourceNotFoundException {
		return false, err
	}
	return false, nil
}

func trimLogStreamName(logStreamName string) string {
	// logStreamName example: copilot/{name}/1cc0685ad01d4d0f8e4e2c00d1775c56
	return strings.TrimPrefix(logStreamName, logStreamNamePrefix)
}

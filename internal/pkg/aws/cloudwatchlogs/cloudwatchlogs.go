// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudwatchlogs provides a client to make API requests to Amazon CloudWatch Logs.
package cloudwatchlogs

import (
	"fmt"
	"sort"
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
	LogGroup               string
	LogStreamPrefixFilters []string // If nil, retrieve logs from all log streams.
	Limit                  *int64
	StartTime              *int64
	EndTime                *int64
	StreamLastEventTime    map[string]int64

	LogStreamLimit int
}

// New returns a CloudWatchLogs configured against the input session.
func New(s *session.Session) *CloudWatchLogs {
	return &CloudWatchLogs{
		client: cloudwatchlogs.New(s),
	}
}

// logStreams returns all name of the log streams in a log group with optional limit and prefix filters.
func (c *CloudWatchLogs) logStreams(logGroup string, logStreamLimit int, logStreamPrefixes ...string) ([]string, error) {
	var logStreamNames []string
	logStreamsResp := &cloudwatchlogs.DescribeLogStreamsOutput{}
	for {
		var err error
		logStreamsResp, err = c.client.DescribeLogStreams(&cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String(logGroup),
			Descending:   aws.Bool(true),
			OrderBy:      aws.String(cloudwatchlogs.OrderByLastEventTime),
			NextToken:    logStreamsResp.NextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("describe log streams of log group %s: %w", logGroup, err)
		}
		if len(logStreamsResp.LogStreams) == 0 {
			return nil, fmt.Errorf("no log stream found in log group %s", logGroup)
		}

		var streams []string
		for _, logStream := range logStreamsResp.LogStreams {
			name := aws.StringValue(logStream.LogStreamName)
			if name == "" {
				continue
			}
			streams = append(streams, name)
		}
		if len(logStreamPrefixes) != 0 {
			logStreamNames = append(logStreamNames, filterStringSliceByPrefix(streams, logStreamPrefixes)...)
		} else {
			logStreamNames = append(logStreamNames, streams...)
		}
		if logStreamLimit != 0 && len(logStreamNames) >= logStreamLimit {
			break
		}
		if token := logStreamsResp.NextToken; aws.StringValue(token) == "" {
			break
		}
	}

	return truncateStreams(logStreamLimit, logStreamNames), nil
}

// LogEvents returns an array of Cloudwatch Logs events.
func (c *CloudWatchLogs) LogEvents(opts LogEventsOpts) (*LogEventsOutput, error) {
	var events []*Event
	in := initGetLogEventsInput(opts)

	logStreams, err := c.logStreams(opts.LogGroup, opts.LogStreamLimit, opts.LogStreamPrefixFilters...)
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

func truncateStreams(limit int, streams []string) []string {
	if limit == 0 || len(streams) <= limit {
		return streams
	}
	return streams[:limit]
}

func initGetLogEventsInput(opts LogEventsOpts) *cloudwatchlogs.GetLogEventsInput {
	return &cloudwatchlogs.GetLogEventsInput{
		LogGroupName: aws.String(opts.LogGroup),
		StartTime:    opts.StartTime,
		EndTime:      opts.EndTime,
		Limit:        opts.Limit,
	}
}

// Example: if the prefixes is []string{"a"} and all is []string{"a", "b", "ab"}
// then it returns []string{"a", "ab"}. Empty string prefixes are not supported.
func filterStringSliceByPrefix(all, prefixes []string) []string {
	trie := buildTrie(prefixes)
	var matches []string
	for _, str := range all {
		if trie.isPrefixOf(str) {
			matches = append(matches, str)
		}
	}
	return matches
}

type trieNode struct {
	children map[rune]*trieNode
	hasWord  bool
}

func newTrieNode() *trieNode {
	return &trieNode{
		children: make(map[rune]*trieNode),
	}
}

type trie struct {
	root *trieNode
}

func buildTrie(strs []string) trie {
	root := newTrieNode()
	for _, str := range strs {
		node := root
		for _, char := range str {
			if _, ok := node.children[char]; !ok {
				node.children[char] = newTrieNode()
			}
			node = node.children[char]
		}
		node.hasWord = true
	}
	return trie{root: root}
}

func (t *trie) isPrefixOf(str string) bool {
	node := t.root
	for _, char := range str {
		child, ok := node.children[char]
		if !ok {
			return false
		}
		if child.hasWord {
			return true
		}
		node = child
	}
	return false
}

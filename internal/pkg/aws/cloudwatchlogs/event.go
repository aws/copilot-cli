// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cloudwatchlogs contains utility functions for Cloudwatch Logs client.
package cloudwatchlogs

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
)

const (
	shortLogStreamNameLength = 25
)

// Event represents a log event.
type Event struct {
	LogStreamName string `json:"logStreamName"`
	IngestionTime int64  `json:"ingestionTime"`
	Message       string `json:"message"`
	Timestamp     int64  `json:"timestamp"`
}

// JSONString returns the stringified LogEvent struct with json format.
func (l *Event) JSONString() (string, error) {
	b, err := json.Marshal(l)
	if err != nil {
		return "", fmt.Errorf("marshal a log event: %w", err)
	}
	return fmt.Sprintf("%s\n", b), nil
}

// HumanString returns the stringified LogEvent struct with human readable format.
func (l *Event) HumanString() string {
	for _, code := range fatalCodes {
		l.Message = strings.ReplaceAll(l.Message, code, color.Red.Sprint(code))
	}
	for _, code := range warningCodes {
		l.Message = strings.ReplaceAll(l.Message, code, color.Yellow.Sprint(code))
	}
	return fmt.Sprintf("%s %s\n", color.Grey.Sprint(l.shortLogStreamName()), l.Message)
}

func (l *Event) shortLogStreamName() string {
	if len(l.LogStreamName) < shortLogStreamNameLength {
		return l.LogStreamName
	}
	return l.LogStreamName[0:shortLogStreamNameLength]
}

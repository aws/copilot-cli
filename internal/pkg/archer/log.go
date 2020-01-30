// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package archer contains the structs that represent archer concepts, and the associated interfaces to manipulate them.
package archer

// LogEntry represents a single CloudWatch log entry.
type LogEntry struct {
	Timestamp  string `json:"timestamp"`
	StreamName string `json:"streamName"`
	Message    string `json:"message"`
}

type LogManager interface {
	LogGetter
}

// LogGetter fetches and returns log events from CloudWatch.
type LogGetter interface {
	GetLog(appName, startTime string) (*[]LogEntry, error)
}

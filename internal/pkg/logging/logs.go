// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package logging contains utility functions for ECS logging.
package logging

import (
	"fmt"
	"io"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
)

// HumanJSONStringer can output in both human-readable and JSON format.
type HumanJSONStringer interface {
	HumanString() string
	JSONString() (string, error)
}

// WriteJSONLogs outputs CloudWatch logs in JSON format.
func WriteJSONLogs(w io.Writer, logStringers []HumanJSONStringer) error {
	for _, logStringer := range logStringers {
		data, err := logStringer.JSONString()
		if err != nil {
			return fmt.Errorf("get log string in JSON: %w", err)
		}
		fmt.Fprint(w, data)
	}
	return nil
}

// WriteHumanLogs outputs CloudWatch logs in human-readable format.
func WriteHumanLogs(w io.Writer, logStringers []HumanJSONStringer) error {
	for _, logStringer := range logStringers {
		fmt.Fprint(w, logStringer.HumanString())
	}
	return nil
}

func cwEventsToHumanJSONStringers(events []*cloudwatchlogs.Event) []HumanJSONStringer {
	// golang limitation: https://golang.org/doc/faq#convert_slice_of_interface
	logStringers := make([]HumanJSONStringer, len(events))
	for ind, event := range events {
		logStringers[ind] = event
	}
	return logStringers
}

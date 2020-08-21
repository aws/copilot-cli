// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package logs

import (
	"fmt"
	"io"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudwatchlogs"
)

// OutputCwLogsJSON outputs CloudWatch logs in JSON format.
func OutputCwLogsJSON(w io.Writer, logs []*cloudwatchlogs.Event) error {
	for _, log := range logs {
		data, err := log.JSONString()
		if err != nil {
			return err
		}
		fmt.Fprint(w, data)
	}
	return nil
}

// OutputCwLogsHuman outputs CloudWatch logs in human-readable format.
func OutputCwLogsHuman(w io.Writer, logs []*cloudwatchlogs.Event) error {
	for _, log := range logs {
		fmt.Fprint(w, log.HumanString())
	}
	return nil
}

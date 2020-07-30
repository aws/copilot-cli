// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cloudwatchlogs

import "fmt"

// NoMoreLogEvents indicates no more log events.
type NoMoreLogEvents struct {
	LogGroupName string
}

func (e *NoMoreLogEvents) Error() string {
	return fmt.Sprintf("no more log events for log group %s", e.LogGroupName)
}

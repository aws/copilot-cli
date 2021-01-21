// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

var (
	notStartedStackStatus = stackStatus{
		value: "not started",
	}
)

type stackStatus struct {
	value  cloudformation.StackStatus
	reason string
}

func prettifyLatestStackStatus(statuses []stackStatus) string {
	color := colorStackStatus(statuses)
	latest := string(statuses[len(statuses)-1].value)
	pretty := strings.ToLower(strings.ReplaceAll(latest, "_", " "))
	return color("[%s]", pretty)
}

func prettifyRolloutStatus(rollout string) string {
	pretty := strings.ToLower(strings.ReplaceAll(rollout, "_", " "))
	return fmt.Sprintf("[%s]", pretty)
}

func prettifyElapsedTime(sw *stopWatch) string {
	elapsed, hasStarted := sw.elapsed()
	if !hasStarted {
		return ""
	}
	return color.Faint.Sprintf("[%.1fs]", elapsed.Seconds())
}

func failureReasons(statuses []stackStatus) []string {
	var reasons []string
	for _, status := range statuses {
		if !status.value.Failure() {
			continue
		}
		if status.reason == "" {
			continue
		}
		reasons = append(reasons, status.reason)

	}
	return reasons
}

func splitByLength(s string, maxLength int) []string {
	numItems := len(s)/maxLength + 1
	var ss []string
	for i := 0; i < numItems; i += 1 {
		if i == numItems-1 {
			ss = append(ss, s[i*maxLength:])
			continue
		}
		ss = append(ss, s[i*maxLength:(i+1)*maxLength])
	}
	return ss
}

// colorStackStatus returns a function to colorize a stack status based on past events.
// If there was any failure in the history of the stack, then color the status as red.
// If the latest event is a success, then it's green.
// Otherwise, it's fainted.
func colorStackStatus(statuses []stackStatus) func(format string, a ...interface{}) string {
	hasPastFailure := false
	for _, status := range statuses {
		if status.value.Failure() {
			hasPastFailure = true
			break
		}
	}

	latestStatus := statuses[len(statuses)-1]
	if latestStatus.value.Success() && !hasPastFailure {
		return color.Green.Sprintf
	}
	if hasPastFailure {
		return color.Red.Sprintf
	}
	return color.Faint.Sprintf
}

func colorFailureReason(text string) string {
	return color.DullRed.Sprint(text)
}

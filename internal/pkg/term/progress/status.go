// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

var (
	notStartedStackStatus = cfnStatus{
		value: notStartedResult{},
	}
	alarmOKState = "OK"
	inAlarmState = "ALARM"
)

type result interface {
	IsSuccess() bool
	IsFailure() bool
	InProgress() bool
	fmt.Stringer
}

// notStartedResult represents an unbegun status that implements the result interface.
type notStartedResult struct{}

// IsSuccess is false for a non-started state.
func (r notStartedResult) IsSuccess() bool {
	return false
}

// IsFailure is false for a non-started state.
func (r notStartedResult) IsFailure() bool {
	return false
}

// InProgress is false for a non-started state.
func (r notStartedResult) InProgress() bool {
	return false
}

// String implements the fmt.Stringer interface.
func (r notStartedResult) String() string {
	return "not started"
}

type cfnStatus struct {
	value  result
	reason string
}

func prettifyLatestStackStatus(statuses []cfnStatus) string {
	color := colorStackStatus(statuses)
	latest := statuses[len(statuses)-1].value.String()
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

func prettifyAlarmState(state string) string {
	var pretty string
	switch state {
	case alarmOKState:
		pretty = color.Green.Sprintf("[%s]", state)
	case inAlarmState:
		pretty = color.Red.Sprintf("[%s]", state)
	default:
		pretty = fmt.Sprintf("[%s]", state)
	}
	return pretty
}

func failureReasons(statuses []cfnStatus) []string {
	var reasons []string
	for _, status := range statuses {
		if !status.value.IsFailure() {
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
func colorStackStatus(statuses []cfnStatus) func(format string, a ...interface{}) string {
	hasPastFailure := false
	for _, status := range statuses {
		if status.value.IsFailure() {
			hasPastFailure = true
			break
		}
	}

	latestStatus := statuses[len(statuses)-1]
	if latestStatus.value.IsSuccess() && !hasPastFailure {
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

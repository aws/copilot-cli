// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

// ResourceMatcher is a function that returns true if the resource event matches a criteria.
type ResourceMatcher func(deploy.Resource) bool

// HumanizeResourceEvents groups raw deploy events under human-friendly tab-separated texts
// that can be passed into the Events() method. Every text to display starts with status in progress.
// For every resource event that belongs to a text, we  preserve failure events if there was one.
// Otherwise, the text remains in progress until the expected number of resources reach the complete status.
func HumanizeResourceEvents(orderedTexts []Text, resourceEvents []deploy.ResourceEvent, matcher map[Text]ResourceMatcher, wantedCount map[Text]int) []TabRow {
	// Assign a status to text from all matched events.
	statuses := make(map[Text]Status)
	reasons := make(map[Text]string)
	for text, matches := range matcher {
		statuses[text] = StatusInProgress
		for _, resourceEvent := range resourceEvents {
			if !matches(resourceEvent.Resource) {
				continue
			}
			if oldStatus, ok := statuses[text]; ok && oldStatus == StatusFailed {
				// There was a failure event, keep its status.
				continue
			}
			status := toStatus(resourceEvent.Status)
			if status == StatusComplete || status == StatusSkipped {
				// If there are more resources that needs to have StatusComplete then the text should remain in StatusInProgress.
				wantedCount[text] = wantedCount[text] - 1
				if wantedCount[text] > 0 {
					status = StatusInProgress
				}
			}
			statuses[text] = status
			reasons[text] = resourceEvent.StatusReason
		}
	}

	// Serialize the text and status to a format digestible by Events().
	var rows []TabRow
	for _, text := range orderedTexts {
		status, ok := statuses[text]
		if !ok {
			continue
		}
		coloredStatus := fmt.Sprintf("[%s]", status)
		if status == StatusInProgress {
			coloredStatus = color.Grey.Sprint(coloredStatus)
		}
		if status == StatusFailed {
			coloredStatus = color.Red.Sprint(coloredStatus)
		}

		rows = append(rows, TabRow(fmt.Sprintf("%s\t%s", color.Grey.Sprint(text), coloredStatus)))
		if status == StatusFailed {
			rows = append(rows, TabRow(fmt.Sprintf("  %s\t", reasons[text])))
		}
	}
	return rows
}

func toStatus(s string) Status {
	if strings.HasSuffix(s, "FAILED") {
		return StatusFailed
	}
	if strings.HasSuffix(s, "COMPLETE") {
		return StatusComplete
	}
	if strings.HasSuffix(s, "SKIPPED") {
		return StatusSkipped
	}
	return StatusInProgress
}

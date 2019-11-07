// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
)

// ResourceMatcher is a function that returns true if the resource event matches a criteria.
type ResourceMatcher func(deploy.Resource) bool

// HumanizeResourceEvents groups raw deploy events under human-friendly progress texts that can be passed into the Events() method.
// It iterates through the list of resources, if the resource matches a progress text then the progress text is displayed.
// For every progress text that's matched, we prioritize failure events first, then in progress, and finally complete or skipped events.
func HumanizeResourceEvents(resourceEvents []deploy.ResourceEvent, displayOrder []Text, matcher map[Text]ResourceMatcher) []TabRow {
	// Assign a status to text from all matched events.
	// If a failure event occurred we keep that status otherwise we use the latest matched resource's status.
	textStatus := make(map[Text]Status)
	textReason := make(map[Text]string)
	for text, matches := range matcher {
		for _, resourceEvent := range resourceEvents {
			if !matches(resourceEvent.Resource) {
				continue
			}
			if curStatus, ok := textStatus[text]; ok && curStatus == StatusFailed {
				continue
			}
			textStatus[text] = toStatus(resourceEvent.Status)
			textReason[text] = resourceEvent.StatusReason
		}
	}

	// Serialize the text and status to a format digestible by Events().
	var updates []TabRow
	for _, text := range displayOrder {
		status, ok := textStatus[text]
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

		updates = append(updates, TabRow(fmt.Sprintf("%s\t%s", color.Grey.Sprint(text), coloredStatus)))
		if status == StatusFailed {
			updates = append(updates, TabRow(fmt.Sprintf("  %s\t", textReason[text])))
		}
	}
	return updates
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

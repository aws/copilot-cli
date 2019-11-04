// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
)

// progress is the interface to inform the user that a long operation is taking place.
type progress interface {
	// Start starts displaying progress with a label.
	Start(label string)
	// Stop ends displaying progress with a label.
	Stop(label string)
	// Events writes additional information in between the start and stop stages.
	Events([]string)
}

type progressText string
type progressStatus string
type progressMatcher func(deploy.ResourceEvent) bool

// envProgressOrder is the order in which we want to progress text to appear on the terminal.
var envProgressOrder = []progressText{vpc, internetGateway, publicSubnets, privateSubnets, natGateway, routeTables, ecsCluster, alb}

// Sub-task headers displayed while deploying an environment.
const (
	vpc             progressText = "- Virtual private cloud on 2 availability zones to hold your services"
	internetGateway progressText = "  - Internet gateway to connect the network to the internet"
	publicSubnets   progressText = "  - Public subnets for internet facing services "
	privateSubnets  progressText = "  - Private subnets for services that can't be reached from the internet"
	natGateway      progressText = "  - NAT gateway for private services to send requests to the internet"
	routeTables     progressText = "  - Routing tables for services to talk with each other"
	ecsCluster      progressText = "- ECS Cluster to hold your services "
	alb             progressText = "- Application load balancer to distribute traffic "
)

// Sub-task progression status displayed while deploying an environment.
const (
	inProgress progressStatus = "IN_PROGRESS"
	failed     progressStatus = "FAILED"
	complete   progressStatus = "COMPLETE"
	skipped    progressStatus = "SKIPPED"
)

// humanizeResourceEvents groups raw deploy events under human-friendly progress texts.
// It iterates through the list of resources, if the resource matches a progress text then the progress text is displayed.
// For every progress text that's matched, we prioritize failure events first, then in progress, and finally complete or skipped events.
func humanizeResourceEvents(resources []deploy.ResourceEvent, displayOrder []progressText, matcher map[progressText]progressMatcher) []string {
	// Gather statuses and reasons for each progress text.
	statuses := make(map[progressText]progressStatus)
	reasons := make(map[progressText]string)
	for _, resource := range resources {
		for text, match := range matcher {
			if !match(resource) {
				continue
			}
			newStatus := resourceStatus(resource.Status).toProgressStatus()
			newReason := resource.StatusReason
			if _, ok := statuses[text]; !ok {
				statuses[text] = newStatus
				reasons[text] = newReason
				continue
			}
			switch currentStatus := statuses[text]; currentStatus {
			case failed:
				// Do nothing. If the text had a resource with a failure we don't want to overwrite its status and reasons.
			case inProgress:
				if newStatus == failed {
					// Only overwrite the value if the new resource had a failure.
					statuses[text] = newStatus
					reasons[text] = newReason
				}
			case complete, skipped:
				if newStatus == inProgress {
					// Only overwrite the value if a new resource comes in under the progress text that's in progress.
					statuses[text] = newStatus
					reasons[text] = newReason
				}
			}
		}
	}

	// Transform events into strings "{progressText}\t{progressStatus}"
	var updates []string
	for _, text := range displayOrder {
		status, ok := statuses[text]
		if !ok {
			continue
		}
		coloredStatus := fmt.Sprintf("[%s]", status)
		if status == inProgress {
			coloredStatus = color.Grey.Sprint(coloredStatus)
		}
		if status == failed {
			coloredStatus = color.Red.Sprint(coloredStatus)
		}

		// The "\t" character is used to denote columns.
		updates = append(updates, fmt.Sprintf("%s\t%s", color.Grey.Sprint(text), coloredStatus))
		if status == failed {
			updates = append(updates, fmt.Sprintf("  %s\t", reasons[text]))
		}
	}
	return updates
}

type resourceStatus string

func (s resourceStatus) toProgressStatus() progressStatus {
	if strings.HasSuffix(string(s), string(failed)) {
		return failed
	}
	if strings.HasSuffix(string(s), string(complete)) {
		return complete
	}
	if strings.HasSuffix(string(s), string(skipped)) {
		return skipped
	}
	return inProgress
}

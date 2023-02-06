// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package selector

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

var (
	// ErrLocalEnvsNotFound is returned when there are no environment manifests in the workspace.
	ErrLocalEnvsNotFound = errors.New("no environments found")
	// ErrVPCNotFound is returned when no existing VPCs are found.
	ErrVPCNotFound = errors.New("no existing VPCs found")
	// ErrSubnetsNotFound is returned when no existing subnets are found.
	ErrSubnetsNotFound = errors.New("no existing subnets found")
)

// ErrNoWorkloadInApp occurs when there is no workload in an application.
type ErrNoWorkloadInApp struct {
	appName string
}

func (e *ErrNoWorkloadInApp) Error() string {
	return fmt.Sprintf("no workloads found in app %s", e.appName)
}

// RecommendActions gives suggestions to fix the error.
func (e *ErrNoWorkloadInApp) RecommendActions() string {
	return fmt.Sprintf("Couldn't find any workloads associated with app %s, try initializing one: %s.",
		color.HighlightUserInput(e.appName),
		color.HighlightCode("copilot [svc/job] init"))
}

// ErrNoJobInApp occurs when there is no job in an application.
type ErrNoJobInApp struct {
	appName string
}

func (e *ErrNoJobInApp) Error() string {
	return fmt.Sprintf("no jobs found in app %s", e.appName)
}

// RecommendActions gives suggestions to fix the error.
func (e *ErrNoJobInApp) RecommendActions() string {
	return fmt.Sprintf("Couldn't find any jobs associated with app %s, try initializing one: %s.",
		color.HighlightUserInput(e.appName),
		color.HighlightCode("copilot job init"))
}

// ErrNoServiceInApp occurs when there is no service in an application.
type ErrNoServiceInApp struct {
	appName string
}

func (e *ErrNoServiceInApp) Error() string {
	return fmt.Sprintf("no services found in app %s", e.appName)
}

// RecommendActions gives suggestions to fix the error.
func (e *ErrNoServiceInApp) RecommendActions() string {
	return fmt.Sprintf("Couldn't find any services associated with app %s, try initializing one: %s.",
		color.HighlightUserInput(e.appName),
		color.HighlightCode("copilot svc init"))
}

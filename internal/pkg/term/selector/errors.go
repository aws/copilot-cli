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

// errNoWorkloadInApp occurs when there is no workload in an application.
type errNoWorkloadInApp struct {
	appName string
}

func (e *errNoWorkloadInApp) Error() string {
	return fmt.Sprintf("no workloads found in app %s", e.appName)
}

// RecommendActions gives suggestions to fix the error.
func (e *errNoWorkloadInApp) RecommendActions() string {
	return fmt.Sprintf("Couldn't find any workloads associated with app %s, try initializing one: %s.",
		color.HighlightUserInput(e.appName),
		color.HighlightCode("copilot [svc/job] init"))
}

// errNoJobInApp occurs when there is no job in an application.
type errNoJobInApp struct {
	appName string
}

func (e *errNoJobInApp) Error() string {
	return fmt.Sprintf("no jobs found in app %s", e.appName)
}

// RecommendActions gives suggestions to fix the error.
func (e *errNoJobInApp) RecommendActions() string {
	return fmt.Sprintf("Couldn't find any jobs associated with app %s, try initializing one: %s.",
		color.HighlightUserInput(e.appName),
		color.HighlightCode("copilot job init"))
}

// errNoServiceInApp occurs when there is no service in an application.
type errNoServiceInApp struct {
	appName string
}

func (e *errNoServiceInApp) Error() string {
	return fmt.Sprintf("no services found in app %s", e.appName)
}

// RecommendActions gives suggestions to fix the error.
func (e *errNoServiceInApp) RecommendActions() string {
	return fmt.Sprintf("Couldn't find any services associated with app %s, try initializing one: %s.",
		color.HighlightUserInput(e.appName),
		color.HighlightCode("copilot svc init"))
}

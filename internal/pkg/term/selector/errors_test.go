// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package selector

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrNoWorkloadInApp_RecommendActions(t *testing.T) {
	err := &ErrNoWorkloadInApp{
		appName: "mockApp",
	}
	require.Equal(t, "Couldn't find any workloads associated with app mockApp, try initializing one: `copilot [svc/job] init`.", err.RecommendActions())
}

func TestErrNoJobInApp_RecommendActions(t *testing.T) {
	err := &ErrNoJobInApp{
		appName: "mockApp",
	}
	require.Equal(t, "Couldn't find any jobs associated with app mockApp, try initializing one: `copilot job init`.", err.RecommendActions())
}

func TestErrNoServiceInApp_RecommendActions(t *testing.T) {
	err := &ErrNoServiceInApp{
		appName: "mockApp",
	}
	require.Equal(t, "Couldn't find any services associated with app mockApp, try initializing one: `copilot svc init`.", err.RecommendActions())
}

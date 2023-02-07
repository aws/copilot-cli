// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package selector

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrNoWorkloadInApp_Error(t *testing.T) {
	err := &errNoWorkloadInApp{
		appName: "mockApp",
	}
	require.Equal(t, "no workloads found in app mockApp", err.Error())
}

func TestErrNoWorkloadInApp_RecommendActions(t *testing.T) {
	err := &errNoWorkloadInApp{
		appName: "mockApp",
	}
	require.Equal(t, "Couldn't find any workloads associated with app mockApp, try initializing one: `copilot [svc/job] init`.", err.RecommendActions())
}

func TestErrNoJobInApp_Error(t *testing.T) {
	err := &errNoJobInApp{
		appName: "mockApp",
	}
	require.Equal(t, "no jobs found in app mockApp", err.Error())
}

func TestErrNoJobInApp_RecommendActions(t *testing.T) {
	err := &errNoJobInApp{
		appName: "mockApp",
	}
	require.Equal(t, "Couldn't find any jobs associated with app mockApp, try initializing one: `copilot job init`.", err.RecommendActions())
}

func TestErrNoServiceInApp_Error(t *testing.T) {
	err := &errNoServiceInApp{
		appName: "mockApp",
	}
	require.Equal(t, "no services found in app mockApp", err.Error())
}

func TestErrNoServiceInApp_RecommendActions(t *testing.T) {
	err := &errNoServiceInApp{
		appName: "mockApp",
	}
	require.Equal(t, "Couldn't find any services associated with app mockApp, try initializing one: `copilot svc init`.", err.RecommendActions())
}

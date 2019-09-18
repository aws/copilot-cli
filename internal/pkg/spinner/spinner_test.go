// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package spinner

import (
	"testing"

	"github.com/stretchr/testify/mock"
)

type mockSpinner struct {
	mock.Mock
}

func (m mockSpinner) Start() {
	m.Called()
}

func (m mockSpinner) Stop() {
	m.Called()
}

func TestStart(t *testing.T) {
	mockSpinner := new(mockSpinner)
	mockSpinner.On("Start")
	s := &Spinner{
		internal: mockSpinner,
	}

	s.Start("doing stuff")

	mockSpinner.AssertExpectations(t)
}

func TestStop(t *testing.T) {
	mockSpinner := new(mockSpinner)
	mockSpinner.On("Stop")
	s := &Spinner{
		internal: mockSpinner,
	}

	s.Stop("done")

	mockSpinner.AssertExpectations(t)
}

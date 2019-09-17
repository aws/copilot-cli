// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package spinner

import (
	"testing"

	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/spinner/mocks"
	"github.com/golang/mock/gomock"
)

func TestStart(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSpinner := mocks.NewMockspinner(ctrl)

	s := &Spinner{
		internal: mockSpinner,
	}

	mockSpinner.EXPECT().Start()

	s.Start("start")
}

func TestStop(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockSpinner := mocks.NewMockspinner(ctrl)

	s := &Spinner{
		internal: mockSpinner,
	}

	mockSpinner.EXPECT().Stop()

	s.Stop("stop")
}

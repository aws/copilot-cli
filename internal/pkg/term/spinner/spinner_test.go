// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package spinner

import (
	"os"
	"testing"
	"time"

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/term/spinner/mocks"
	spin "github.com/briandowns/spinner"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("it should initialize the internal spinner", func(t *testing.T) {
		got := New()

		v, ok := got.internal.(*spin.Spinner)
		require.True(t, ok)

		require.Equal(t, os.Stderr, v.Writer)
		require.Equal(t, 125*time.Millisecond, v.Delay)
	})
}

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

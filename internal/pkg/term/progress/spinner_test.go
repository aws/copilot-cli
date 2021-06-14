// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/term/progress/mocks"
	spin "github.com/briandowns/spinner"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("it should initialize the spin spinner", func(t *testing.T) {
		buf := new(strings.Builder)
		got := NewSpinner(buf)
		wantedInterval := 125 * time.Millisecond
		if os.Getenv("CI") == "true" {
			wantedInterval = 30 * time.Second
		}

		v, ok := got.spin.(*spin.Spinner)
		require.True(t, ok)

		require.Equal(t, buf, v.Writer)
		require.Equal(t, wantedInterval, v.Delay)
	})
}

func TestSpinner_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSpinner := mocks.NewMockstartStopper(ctrl)

	s := &Spinner{
		spin: mockSpinner,
	}

	mockSpinner.EXPECT().Start()

	s.Start("start")
}

func TestSpinner_Stop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSpinner := mocks.NewMockstartStopper(ctrl)
	s := &Spinner{
		spin: mockSpinner,
	}
	mockSpinner.EXPECT().Stop()

	// WHEN
	s.Stop("stop")
}

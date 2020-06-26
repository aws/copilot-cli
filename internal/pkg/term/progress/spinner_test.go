// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/term/progress/mocks"
	spin "github.com/briandowns/spinner"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type mockWriteFlusher struct {
	buf *bytes.Buffer
}

func (m *mockWriteFlusher) Write(p []byte) (n int, err error) {
	return m.buf.Write(p)
}

func (m *mockWriteFlusher) Flush() error {
	return nil
}

type mockCursor struct {
	buf *bytes.Buffer
}

func (m *mockCursor) Up(n int) {
	s := fmt.Sprintf("[up%d]", n)
	m.buf.Write([]byte(s))
}

func (m *mockCursor) Down(n int) {
	s := fmt.Sprintf("[down%d]", n)
	m.buf.Write([]byte(s))
}

func (m *mockCursor) EraseLine() {
	m.buf.Write([]byte("erase"))
}

func TestNew(t *testing.T) {
	t.Run("it should initialize the spin spinner", func(t *testing.T) {
		got := NewSpinner()

		v, ok := got.spin.(*spin.Spinner)
		require.True(t, ok)

		require.Equal(t, os.Stderr, v.Writer)
		require.Equal(t, 125*time.Millisecond, v.Delay)
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
	testCases := map[string]struct {
		eventsBuf     *bytes.Buffer
		pastEvents    []TabRow
		wantedSEvents string
	}{
		"without existing events": {
			eventsBuf:     &bytes.Buffer{},
			wantedSEvents: "",
		},
		"with exiting events": {
			eventsBuf:     &bytes.Buffer{},
			pastEvents:    []TabRow{"hello", "world"},
			wantedSEvents: "hello\nworld\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			mockSpinner := mocks.NewMockstartStopper(ctrl)
			mockWriter := &mockWriteFlusher{buf: tc.eventsBuf}
			s := &Spinner{
				spin:         mockSpinner,
				pastEvents:   tc.pastEvents,
				eventsWriter: mockWriter,
			}
			mockSpinner.EXPECT().Stop()

			// WHEN
			s.Stop("stop")

			// THEN
			require.Equal(t, tc.wantedSEvents, tc.eventsBuf.String())
			require.Nil(t, s.pastEvents)
		})
	}
}

func TestSpinner_Events(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testCases := map[string]struct {
		eventsBuf *bytes.Buffer

		pastEvents []TabRow
		newEvents  []TabRow

		wantedSEvents string
	}{
		"without existing events": {
			eventsBuf: &bytes.Buffer{},
			newEvents: []TabRow{"hello", "world"},

			wantedSEvents: "\nhello\nworld[up2]\r",
		},
		"with existing events": {
			eventsBuf:  &bytes.Buffer{},
			pastEvents: []TabRow{"hello", "world"},
			newEvents:  []TabRow{"this", "is", "fine"},

			wantedSEvents: "[down1]erase[down1]erase[up2]\nthis\nis\nfine[up3]\r", // write new events
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			mockSpinner := mocks.NewMockstartStopper(ctrl)
			s := &Spinner{
				spin:         mockSpinner,
				cur:          &mockCursor{buf: tc.eventsBuf},
				pastEvents:   tc.pastEvents,
				eventsWriter: &mockWriteFlusher{buf: tc.eventsBuf},
			}

			// WHEN
			s.Events(tc.newEvents)

			// THEN
			require.Equal(t, tc.wantedSEvents, tc.eventsBuf.String())
		})
	}
}

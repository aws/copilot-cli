// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package spinner

import (
	"bytes"
	"os"
	"testing"
	"time"

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/term/spinner/mocks"
	spin "github.com/briandowns/spinner"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type mockWriteFluster struct {
	buf *bytes.Buffer
}

func (m *mockWriteFluster) Write(p []byte) (n int, err error) {
	return m.buf.Write(p)
}

func (m *mockWriteFluster) Flush() error {
	return nil
}

func TestNew(t *testing.T) {
	t.Run("it should initialize the internal spinner", func(t *testing.T) {
		got := New()

		v, ok := got.internal.(*spin.Spinner)
		require.True(t, ok)

		require.Equal(t, os.Stderr, v.Writer)
		require.Equal(t, 125*time.Millisecond, v.Delay)
	})
}

func TestSpinner_Start(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mockSpinner := mocks.NewMockspinner(ctrl)

	s := &Spinner{
		internal: mockSpinner,
	}

	mockSpinner.EXPECT().Start()

	s.Start("start")
}

func TestSpinner_Stop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	testCases := map[string]struct {
		tipsBuf      *bytes.Buffer
		existingTips []string
		wantedTips   string
	}{
		"without existing tips": {
			tipsBuf:    &bytes.Buffer{},
			wantedTips: "",
		},
		"with exiting tips": {
			tipsBuf:      &bytes.Buffer{},
			existingTips: []string{"hello", "world"},
			wantedTips:   "hello\nworld\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			mockSpinner := mocks.NewMockspinner(ctrl)
			mockWriter := &mockWriteFluster{buf: tc.tipsBuf}
			s := &Spinner{
				internal:   mockSpinner,
				tips:       tc.existingTips,
				tipsWriter: mockWriter,
			}
			mockSpinner.EXPECT().Stop()

			// WHEN
			s.Stop("stop")

			// THEN
			require.Equal(t, tc.wantedTips, tc.tipsBuf.String())
		})
	}
}

func TestSpinner_Tips(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	testCases := map[string]struct {
		tipsBuf      *bytes.Buffer
		existingTips []string
		newTips      []string

		wantedTips string
	}{
		"without existing tips": {
			tipsBuf: &bytes.Buffer{},
			newTips: []string{"hello", "world"},

			wantedTips: "\nhello\nworld\033[2A",
		},
		"with existing tips": {
			tipsBuf:      &bytes.Buffer{},
			existingTips: []string{"hello", "world"},
			newTips:      []string{"this", "is", "fine"},

			wantedTips: "\033[1B\r\033[K" + // delete the "hello" line
				"\033[1B\r\033[K" + // delete the "world" line
				"\033[2A" + // move back the cursor to the spinner
				"\nthis\nis\nfine" + // write new tips
				"\033[3A", // move back the cursor to the spinner
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			mockSpinner := mocks.NewMockspinner(ctrl)
			mockWriter := &mockWriteFluster{buf: tc.tipsBuf}
			s := &Spinner{
				internal:   mockSpinner,
				tips:       tc.existingTips,
				tipsWriter: mockWriter,
			}

			// WHEN
			s.Tips(tc.newTips)

			// THEN
			require.Equal(t, tc.wantedTips, tc.tipsBuf.String())
		})
	}
}

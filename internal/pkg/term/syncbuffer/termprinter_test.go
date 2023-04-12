// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package syncbuffer

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

type mockFileWriter struct {
	io.Writer
}

func (m mockFileWriter) Fd() uintptr {
	return 0
}

func TestLabeledTermPrinter_Print(t *testing.T) {
	testCases := map[string]struct {
		inNumLines int
		inPadding  int
		wanted     string
	}{
		"display label with given numLines": {
			inNumLines: 2,
			inPadding:  5,
			wanted: `Building your container image 1
     line3 from image1
     line4 from image1
Building your container image 2
     line3 from image2
     line4 from image2
`,
		},

		"display ": {
			inNumLines: -1,
			inPadding:  5,
			wanted: `Building your container image 1
     line1 from image1
     line2 from image1
     line3 from image1
     line4 from image1
Building your container image 2
     line1 from image2
     line2 from image2
     line3 from image2
     line4 from image2
`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			mockBuffer1 := []byte(`line1 from image1
line2 from image1
line3 from image1
line4 from image1`)
			mockSyncBuf1 := New()
			mockSyncBuf1.Write(mockBuffer1)
			buf2 := []byte(`line1 from image2
line2 from image2
line3 from image2
line4 from image2`)
			mockSyncBuf2 := New()
			mockSyncBuf2.Write(buf2)
			var mockLabeledSyncbufs []*LabeledSyncBuffer
			mockLabeledSyncbufs = append(mockLabeledSyncbufs, mockSyncBuf1.WithLabel("Building your container image 1"),
				mockSyncBuf2.WithLabel("Building your container image 2"))

			termOut := &bytes.Buffer{}

			ltp := LabeledTermPrinter{
				term:     mockFileWriter{termOut},
				buffers:  mockLabeledSyncbufs,
				numLines: tc.inNumLines,
				padding:  tc.inPadding,
			}

			// WHEN
			for _, buf := range ltp.buffers {
				buf.MarkDone()
			}
			ltp.Print()

			// THEN
			require.Equal(t, tc.wanted, termOut.String())
		})
	}
}

func TestLabeledTermPrinter_IsDone(t *testing.T) {
	testCases := map[string]struct {
		mockSyncBuf1 *syncBuffer
		mockSyncBuf2 *syncBuffer
		wanted       bool
	}{
		"return false if all buffers are not done": {
			mockSyncBuf1: New(),
			mockSyncBuf2: New(),
			wanted:       false,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			var mockLabeledSyncBufs []*LabeledSyncBuffer
			mockLabeledSyncBufs = append(mockLabeledSyncBufs, tc.mockSyncBuf1.WithLabel("title 1"),
				tc.mockSyncBuf2.WithLabel("title 2"))
			ltp := LabeledTermPrinter{
				term:    mockFileWriter{},
				buffers: mockLabeledSyncBufs,
			}
			mockLabeledSyncBufs[0].MarkDone()

			// WHEN
			got := ltp.IsDone()

			//THEN
			require.Equal(t, tc.wanted, got)
		})
	}
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package syncbuffer

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLabeledTermPrinter_lastNLines(t *testing.T) {
	testCases := map[string]struct {
		logs     []string
		wanted   []string
		numLines int
	}{
		"more than five lines": {
			logs:   []string{"line1", "line2", "line3", "line4", "line5", "line6", "line7"},
			wanted: []string{"line3", "line4", "line5", "line6", "line7"},
		},
		"less than five lines": {
			logs:   []string{"line1", "line2"},
			wanted: []string{"line1", "line2", "", "", ""},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tp := &LabeledTermPrinter{
				numLines: 5,
			}

			// WHEN
			actual := tp.lastNLines(tc.logs)

			// THEN
			require.Equal(t, tc.wanted, actual)
		})
	}
}

func TestLabeledTermPrinter_Print(t *testing.T) {
	testCases := map[string]struct {
		inLabeledTermPrinter LabeledTermPrinter
		wanted               string
	}{
		"display label with given numLines": {
			inLabeledTermPrinter: LabeledTermPrinter{
				buffers: []*LabeledSyncBuffer{
					{
						label: "Building your container image 1",
						syncBuf: &SyncBuffer{
							buf: *bytes.NewBufferString(`line1 from image1
line2 from image1
line3 from image1
line4 from image1
line5 from image1
line6 from image1
line7 from image1`),
							done: make(chan struct{}),
						},
					},
					{
						label: "Building your container image 2",
						syncBuf: &SyncBuffer{
							buf: *bytes.NewBufferString(`line1 from image2
line2 from image2
line3 from image2
line4 from image2
line5 from image2
line6 from image2
line7 from image2`),
							done: make(chan struct{}),
						},
					},
				},
				numLines: 2,
				padding:  5,
			},
			wanted: `Building your container image 1
      line6 from image1
      line7 from image1
Building your container image 2
      line6 from image2
      line7 from image2
`,
		},
		"display all logs if numLines is set to -1": {
			inLabeledTermPrinter: LabeledTermPrinter{
				buffers: []*LabeledSyncBuffer{
					{
						label: "Building your container image 1",
						syncBuf: &SyncBuffer{
							buf:  *bytes.NewBufferString(`line1 from image1`),
							done: make(chan struct{}),
						},
					},
					{
						label: "Building your container image 2",
						syncBuf: &SyncBuffer{
							buf:  *bytes.NewBufferString(`line1 from image2`),
							done: make(chan struct{}),
						},
					},
				},
				numLines: -1,
				padding:  5,
			},
			wanted: `Building your container image 1
      line1 from image1
Building your container image 2
      line1 from image2
`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			termOut := &bytes.Buffer{}
			ltp := tc.inLabeledTermPrinter
			ltp.term = termOut

			// WHEN
			for _, buf := range ltp.buffers {
				buf.syncBuf.MarkDone()
			}
			ltp.Print()

			// THEN
			require.Equal(t, tc.wanted, termOut.String())
		})
	}
}

func TestLabeledTermPrinter_PrintAll(t *testing.T) {
	testCases := map[string]struct {
		inLabeledTermPrinter LabeledTermPrinter
		wanted               string
	}{
		"display label with given numLines": {
			inLabeledTermPrinter: LabeledTermPrinter{
				buffers: []*LabeledSyncBuffer{
					{
						label: "Building your container image 1",
						syncBuf: &SyncBuffer{
							buf: *bytes.NewBufferString(`line1 from image1
line2 from image1
line3 from image1`),
							done: make(chan struct{}),
						},
					},
					{
						label: "Building your container image 2",
						syncBuf: &SyncBuffer{
							buf: *bytes.NewBufferString(`line1 from image2
line2 from image2
line3 from image2`),
							done: make(chan struct{}),
						},
					},
				},
				numLines: 1,
				padding:  5,
			},
			wanted: `Building your container image 1
      line1 from image1
      line2 from image1
      line3 from image1
Building your container image 2
      line1 from image2
      line2 from image2
      line3 from image2
`,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			termOut := &bytes.Buffer{}
			ltp := tc.inLabeledTermPrinter
			ltp.term = termOut

			// WHEN
			for _, buf := range ltp.buffers {
				buf.syncBuf.MarkDone()
			}
			ltp.PrintAll()

			// THEN
			require.Equal(t, tc.wanted, termOut.String())
		})
	}
}

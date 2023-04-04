package syncbuffer

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyncBuffer_Write(t *testing.T) {
	testCases := map[string]struct {
		input        []byte
		wantedOutput string
	}{
		"append to custom buffer with simple input": {
			input:        []byte("hello world"),
			wantedOutput: "hello world",
		},
		"append to custom buffer with empty input": {
			input:        []byte(""),
			wantedOutput: "",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			sb := &SyncBuffer{}

			// WHEN
			sb.Write(tc.input)

			// THEN
			require.Equal(t, tc.wantedOutput, sb.Buf.String())
		})
	}
}

func TestSyncBuffer_IsDone(t *testing.T) {
	testCases := map[string]struct {
		buffer     *SyncBuffer
		wantedDone bool
	}{
		"Buffer is done": {
			buffer:     &SyncBuffer{Done: make(chan struct{}), Buf: bytes.Buffer{}},
			wantedDone: true,
		},
		"Buffer is not done": {
			buffer: &SyncBuffer{Done: make(chan struct{}), Buf: bytes.Buffer{}},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			if tc.wantedDone {
				tc.buffer.MarkDone()
			}

			// WHEN
			actual := tc.buffer.IsDone()

			// THEN
			require.Equal(t, tc.wantedDone, actual)

		})
	}
}

func TestSyncBuffer_strings(t *testing.T) {
	testCases := map[string]struct {
		input  []byte
		wanted []string
	}{
		"single line": {
			input:  []byte("hello"),
			wanted: []string{"hello"},
		},
		"multiple lines": {
			input:  []byte("hello\nworld\n"),
			wanted: []string{"hello", "world"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {

			// GIVEN
			sb := &SyncBuffer{}
			sb.Write(tc.input)

			// WHEN
			actual := sb.strings()
			require.Equal(t, tc.wanted, actual)
		})
	}
}

func TestTermPrinter_lastFiveLogLines(t *testing.T) {
	testCases := map[string]struct {
		logs   []string
		wanted [5]string
	}{
		"more than five lines": {
			logs:   []string{"label", "line1", "line2", "line3", "line4", "line5", "line6", "line7"},
			wanted: [5]string{"line3", "line4", "line5", "line6", "line7"},
		},
		"less than five lines": {
			logs:   []string{"label", "line1", "line2"},
			wanted: [5]string{"line1", "line2"},
		},
		"empty log": {
			logs: []string{},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			tp := &TermPrinter{}

			// WHEN
			actual := tp.lastFiveLogLines(tc.logs)

			// THEN
			require.Equal(t, tc.wanted, actual)
		})
	}
}

func TestTermPrinter_Print(t *testing.T) {
	testCases := map[string]struct {
		logs   []string
		wanted string
	}{
		"display label and last five log lines": {
			logs: []string{
				"label",
				"line 2",
				"line 3",
				"line 4",
				"line 5",
				"line 6",
				"line 7",
				"line 8",
			},
			wanted: `label
line 4
line 5
line 6
line 7
line 8
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN

			buf := &SyncBuffer{
				Buf:  bytes.Buffer{},
				Done: make(chan struct{}),
			}
			buf.Buf.Write([]byte(strings.Join(tc.logs, "\n")))
			termOut := &bytes.Buffer{}
			printer := TermPrinter{
				Buf:  buf,
				Term: termOut,
			}
			// WHEN

			printer.Print()

			// THEN
			require.Equal(t, tc.wanted, termOut.String())
		})
	}
}

func TestTermPrinter_PrintAll(t *testing.T) {
	testCases := map[string]struct {
		logs   []string
		wanted string
	}{
		"display all the output at once": {
			logs: []string{
				"label",
				"line 2",
				"line 3",
				"line 4",
				"line 5",
				"line 6",
				"line 7",
				"line 8",
			},
			wanted: `label
line 2
line 3
line 4
line 5
line 6
line 7
line 8
`,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN

			buf := &SyncBuffer{
				Buf:  bytes.Buffer{},
				Done: make(chan struct{}),
			}
			buf.Buf.Write([]byte(strings.Join(tc.logs, "\n")))
			termOut := &bytes.Buffer{}
			printer := TermPrinter{
				Buf:  buf,
				Term: termOut,
			}
			// WHEN

			printer.PrintAll()

			// THEN
			require.Equal(t, tc.wanted, termOut.String())
		})
	}
}

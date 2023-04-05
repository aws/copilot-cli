// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package syncbuffer provides a synchronized buffer to store and access logs from multiple goroutines.
package syncbuffer

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// maxLogLines is the maximum number of lines to display in the terminal.
const maxLogLines = 5

// fileWriter is the interface to write to a file.
type fileWriter interface {
	io.Writer
}

// SyncBuffer is a synchronized buffer that can be used to store output data and coordinate between multiple goroutines.
type SyncBuffer struct {
	BufMu sync.Mutex   // BufMu is a mutex that can be used to protect access to the buffer.
	Buf   bytes.Buffer // Buf is the buffer that stores the data.

	Done chan struct{} // Done is a channel that can be used to signal when the operations are complete.
}

// Write appends the given bytes to the buffer.
func (b *SyncBuffer) Write(p []byte) (n int, err error) {
	b.BufMu.Lock()
	defer b.BufMu.Unlock()
	return b.Buf.Write(p)
}

// Strings returns an empty slice if the buffer is empty.
// Otherwise, it returns a slice of all the lines stored in the buffer.
func (b *SyncBuffer) strings() []string {
	b.BufMu.Lock()
	defer b.BufMu.Unlock()

	// Split the buffer bytes into lines.
	lines := strings.Split(strings.TrimSpace(b.Buf.String()), "\n")

	return lines
}

// IsDone returns true if the Done channel has been closed, otherwise return false.
func (b *SyncBuffer) IsDone() bool {
	select {
	case <-b.Done:
		return true
	default:
		return false
	}
}

// MarkDone closes the Done channel.
func (b *SyncBuffer) MarkDone() {
	close(b.Done)
}

// TermPrinter is a printer to display logs in the terminal.
type TermPrinter struct {
	Term             fileWriter
	Buf              *SyncBuffer
	PrevWrittenLines int
	TermWidth        int
}

// NewTermPrinter returns a new instance of TermPrinter that writes logs to the given file writer and reads logs from a new synchronized buffer.
func NewTermPrinter(fw fileWriter) *TermPrinter {
	return &TermPrinter{
		Term: fw,
		Buf: &SyncBuffer{
			Done: make(chan struct{}),
		},
	}
}

// PrintLastFiveLines prints the label and the last five lines of logs to the termPrinter fileWriter.
func (t *TermPrinter) PrintLastFiveLines() {
	logs := t.Buf.strings()
	outputLogs := t.lastFiveLogLines(logs)
	if len(outputLogs) > 0 {
		fmt.Fprintln(t.Term, logs[0])
		for _, logLine := range outputLogs {
			fmt.Fprintln(t.Term, logLine)
		}
		writtenLines := t.numLines(append(outputLogs[:], logs[0]))
		t.PrevWrittenLines = writtenLines
	}
}

// lastFiveLogLines returns the last five lines of the given logs, or all the logs if there are less than five lines.
func (t *TermPrinter) lastFiveLogLines(logs []string) [maxLogLines]string {
	// Determine the start and end index to extract last 5 lines
	start := 1
	if len(logs) > maxLogLines {
		start = len(logs) - maxLogLines
	}
	end := len(logs)

	// Extract the last 5 lines
	var logLines [maxLogLines]string
	idx := 0
	for start < end {
		logLines[idx] = strings.TrimSpace(logs[start])
		start++
		idx++
	}
	return logLines
}

// numLines calculates and returns the actual number of lines needed to print the given string slice based on the terminal width.
func (t *TermPrinter) numLines(lines []string) int {
	var numLines float64
	for _, line := range lines {
		// Empty line should be considered as a new line
		if line == "" {
			numLines = numLines + 1
		}
		numLines += math.Ceil(float64(len(line)) / float64(t.TermWidth))
	}
	return int(numLines)
}

// TerminalWidth returns the width of the terminal or an error if failed to get the width of terminal.
func (t *TermPrinter) TerminalWidth() (int, error) {
	width, _, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil {
		return 0, err
	}
	return width, nil
}

// PrintAll writes the entire contents of the buffer to the file writer.
func (t *TermPrinter) PrintAll() {
	outputLogs := t.Buf.strings()
	for _, logLine := range outputLogs {
		fmt.Fprintln(t.Term, logLine)
	}
}

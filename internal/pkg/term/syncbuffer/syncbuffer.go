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

// SyncBuffer is a synchronized buffer used to store the output of build and push operations.
type SyncBuffer struct {
	BufMu sync.Mutex   // bufMu is a mutex to protect access to the buffer.
	Buf   bytes.Buffer // buf is the buffer containing the output of build and push.

	Done chan struct{} // done is a channel indicating whether the build and push is completed.
}

// Write appends the given bytes to the buffer.
func (b *SyncBuffer) Write(p []byte) (n int, err error) {
	b.BufMu.Lock()
	defer b.BufMu.Unlock()
	return b.Buf.Write(p)
}

// Strings returns the label (i.e., the first line of the buffer) and the last five lines of the buffer.
func (b *SyncBuffer) strings() []string {
	b.BufMu.Lock()
	defer b.BufMu.Unlock()

	// Split the buffer bytes into lines.
	lines := strings.Split(strings.TrimSpace(b.Buf.String()), "\n")

	return lines
}

// IsDone returns true if the Done channel has been closed, indicating that the build and push is completed.
func (b *SyncBuffer) IsDone() bool {
	select {
	case <-b.Done:
		return true
	default:
		return false
	}
}

// MarkDone closes the Done channel, indicating that the build and push is completed.
func (b *SyncBuffer) MarkDone() {
	close(b.Done)
}

// fileWriter is the interface to write to a file.
type fileWriter interface {
	io.Writer
}

// TermPrinter is a printer to display logs in the terminal.
type TermPrinter struct {
	Term             fileWriter
	Buf              *SyncBuffer
	PrevWrittenLines int
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

// Print prints the last five lines of logs to the terminal.
func (t *TermPrinter) Print() error {
	logs := t.Buf.strings()
	outputLogs := t.lastFiveLogLines(logs)
	if len(outputLogs) > 0 {
		fmt.Fprintln(t.Term, logs[0])
		for _, logLine := range outputLogs {
			fmt.Fprintln(t.Term, logLine)
		}
		writtenLines, err := t.numLines(append(outputLogs[:], logs[0]))
		if err != nil {
			return fmt.Errorf("get terminal size: %w", err)
		}
		t.PrevWrittenLines = writtenLines
	}
	return nil
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

// numLines calculates the actual number of lines needed to print the given string slice based on the terminal width
// It returns the sum of these line counts or an error occurs while getting the terminal size.
func (t *TermPrinter) numLines(lines []string) (int, error) {
	// Get the terminal width
	width, _, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil {
		return 0, err
	}

	// Calculate the number of lines needed to print the given lines.
	var numLines float64
	for _, line := range lines {
		// Empty line should be considered as a new line
		if line == "" {
			numLines = numLines + 1
		}
		numLines += math.Ceil(float64(len(line)) / float64(width))
	}

	return int(numLines), nil
}

// PrintAll writes the entire contents of the buffer to the file writer if the build and push operation is completed.
func (t *TermPrinter) PrintAll() {
	outputLogs := t.Buf.strings()
	for _, logLine := range outputLogs {
		fmt.Fprintln(t.Term, logLine)
	}
}

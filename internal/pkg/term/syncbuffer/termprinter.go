// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package syncbuffer provides a goroutine safe bytes.Buffer as well printing functionality to the terminal.
package syncbuffer

import (
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/term/cursor"
	"golang.org/x/term"
)

// printAllLinesInBuf represents to print the entire contents in the buffer.
const (
	printAllLinesInBuf = -1
)

// FileWriter is the interface to write to a file.
type FileWriter interface {
	io.Writer
	Fd() uintptr
}

// LabeledTermPrinter is a printer to display label and logs to the terminal.
type LabeledTermPrinter struct {
	term             FileWriter           // term writes logs to the terminal FileWriter.
	buffers          []*LabeledSyncBuffer // buffers stores logs before writing to the terminal.
	numLines         int                  // number of lines that has to be written from each buffer.
	padding          int                  // Leading spaces before rendering to terminal.
	prevWrittenLines int                  // number of lines written from all the buffers.

	// override in unit tests.
	terminalWidth func(fw FileWriter) (int, error)
}

// LabeledTermPrinterOption is a type alias to configure LabeledTermPrinter.
type LabeledTermPrinterOption func(ltp *LabeledTermPrinter)

// NewLabeledTermPrinter returns a LabeledTermPrinter that can print to the terminal filewriter from buffers.
func NewLabeledTermPrinter(fw FileWriter, bufs []*LabeledSyncBuffer, opts ...LabeledTermPrinterOption) *LabeledTermPrinter {
	ltp := &LabeledTermPrinter{
		term:          fw,
		buffers:       bufs,
		numLines:      printAllLinesInBuf, // By default set numlines to -1 to print all from buffers.
		terminalWidth: terminalWidth,
	}
	for _, opt := range opts {
		opt(ltp)
	}
	return ltp
}

// WithNumLines sets the numlines of LabeledTermPrinter.
func WithNumLines(n int) LabeledTermPrinterOption {
	return func(ltp *LabeledTermPrinter) {
		ltp.numLines = n
	}
}

// WithPadding sets the padding of LabeledTermPrinter.
func WithPadding(n int) LabeledTermPrinterOption {
	return func(ltp *LabeledTermPrinter) {
		ltp.padding = n
	}
}

// IsDone returns true if all the buffers are done.
func (ltp *LabeledTermPrinter) IsDone() bool {
	for _, buf := range ltp.buffers {
		if !buf.IsDone() {
			return false
		}
	}
	return true
}

// Print prints the label and the last N lines of logs from each buffer
// to the LabeledTermPrinter fileWriter and erases the previous output.
// If numLines is -1 then print all the values from buffers.
func (ltp *LabeledTermPrinter) Print() error {
	if ltp.numLines == printAllLinesInBuf {
		ltp.printAll()
		return nil
	}
	if ltp.prevWrittenLines > 0 {
		cursor.EraseLinesAbove(ltp.term, ltp.prevWrittenLines)
	}
	ltp.prevWrittenLines = 0
	for _, buf := range ltp.buffers {
		logs := buf.lines()
		if len(logs) == 0 {
			continue
		}
		outputLogs := ltp.lastNLines(logs)
		ltp.writeLines(buf.label, outputLogs)
		writtenLines, err := ltp.calculateLinesCount(buf.label, outputLogs)
		if err != nil {
			return fmt.Errorf("get terminal width: %w", err)
		}
		ltp.prevWrittenLines += writtenLines
	}
	return nil
}

// printAll writes the entire contents of all the buffers to the file writer.
// If one of the buffer gets done then print entire content of the buffer.
// Until all the buffers are written to file writer.
func (ltp *LabeledTermPrinter) printAll() {
	for idx, buf := range ltp.buffers {
		if !buf.IsDone() {
			continue
		}
		outputLogs := ltp.buffers[idx].lines()
		ltp.writeLines(buf.label, outputLogs)
	}
}

// lastNLines returns the last N lines of the given logs where N is the value of tp.numLines.
// If the logs slice contains fewer than N lines, all lines are returned.
// If the given input logs are empty then return slice of empty strings.
func (ltp *LabeledTermPrinter) lastNLines(logs []string) []string {
	var start int
	if len(logs) > ltp.numLines {
		start = len(logs) - ltp.numLines
	}
	end := len(logs)

	// Extract the last N lines
	logLines := make([]string, ltp.numLines)
	idx := 0
	for start < end {
		logLines[idx] = logs[start]
		start++
		idx++
	}
	return logLines
}

// writeLines writes a label and output logs to the terminal associated with the TermPrinter.
func (ltp *LabeledTermPrinter) writeLines(label string, outputLogs []string) {
	fmt.Fprintln(ltp.term, label)
	for _, logLine := range outputLogs {
		fmt.Fprintf(ltp.term, "%s%s\n", strings.Repeat(" ", ltp.padding), logLine)
	}
}

// calculateLinesCount returns the number of lines needed to print the given string slice based on the terminal width
// or an error when failed to fetch terminal width.
func (ltp *LabeledTermPrinter) calculateLinesCount(label string, lines []string) (int, error) {
	width, err := ltp.terminalWidth(ltp.term)
	if err != nil {
		return 0, fmt.Errorf("get terminal width: %w", err)
	}
	numLines := float64(len(label))
	for _, line := range lines {
		// Empty line should be considered as a new line
		if line == "" {
			numLines += 1
		}
		numLines += math.Ceil(float64(len(line)) + float64(ltp.padding)/float64(width))
	}
	return int(numLines), nil
}

// terminalWidth returns the width of the terminal or an error if failed to get the width of terminal.
func terminalWidth(fw FileWriter) (int, error) {
	width, _, err := term.GetSize(int(fw.Fd()))
	if err != nil {
		return 0, err
	}
	return width, nil
}

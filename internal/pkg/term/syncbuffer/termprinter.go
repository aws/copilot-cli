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

const (
	defaultTerminalWidth = 80
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
}

// LabeledTermPrinterOption is a type alias to configure LabeledTermPrinter.
type LabeledTermPrinterOption func(ltp *LabeledTermPrinter)

// NewLabeledTermPrinter returns a LabeledTermPrinter that can print to the terminal filewriter from buffers.
func NewLabeledTermPrinter(fw FileWriter, bufs []*LabeledSyncBuffer, opts ...LabeledTermPrinterOption) *LabeledTermPrinter {
	ltp := &LabeledTermPrinter{
		term:     fw,
		buffers:  bufs,
		numLines: printAllLinesInBuf, // By default set numlines to -1 to print all from buffers.
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
func (ltp *LabeledTermPrinter) Print() {
	if ltp.numLines == printAllLinesInBuf {
		ltp.printAll()
		return
	}
	if ltp.prevWrittenLines > 0 {
		cursor.EraseLinesAbove(ltp.term, ltp.prevWrittenLines)
	}
	ltp.prevWrittenLines = 0
	for _, buf := range ltp.buffers {
		logs := buf.lines()
		outputLogs := ltp.lastNLines(logs)
		ltp.prevWrittenLines += ltp.writeLines(buf.label, outputLogs)
	}
}

// printAll writes the entire contents of all the buffers to the file writer.
// If one of the buffer gets done then print entire content of the buffer.
// Until all the buffers are written to file writer.
func (ltp *LabeledTermPrinter) printAll() {
	for idx := 0; idx < len(ltp.buffers); idx++ {
		if !ltp.buffers[idx].IsDone() {
			continue
		}
		outputLogs := ltp.buffers[idx].lines()
		ltp.writeLines(ltp.buffers[idx].label, outputLogs)
		ltp.buffers = append(ltp.buffers[:idx], ltp.buffers[idx+1:]...)
		idx--
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
// Returns the number of lines needed to erase based on terminal width.
func (ltp *LabeledTermPrinter) writeLines(label string, lines []string) int {
	var numLines float64
	writeLine := func(line string) {
		fmt.Fprintln(ltp.term, line)
		if len(line) == 0 {
			numLines++
			return
		}
		numLines += math.Ceil(float64(len(line)) / float64(terminalWidth(ltp.term)))
	}
	writeLine(label)
	for _, line := range lines {
		writeLine(fmt.Sprintf("%s%s", strings.Repeat(" ", ltp.padding), line))
	}
	return int(numLines)
}

// terminalWidth returns the width of the terminal associated with the given FileWriter.
// If the FileWriter is not associated with a terminal, it returns a default terminal width.
func terminalWidth(fw FileWriter) int {
	terminalWidth := defaultTerminalWidth
	if term.IsTerminal(int(fw.Fd())) {
		// Swallow the error as we do not want propogate the error up to call stack.
		if width, _, err := term.GetSize(int(fw.Fd())); err == nil {
			terminalWidth = width
		}
	}
	return terminalWidth
}

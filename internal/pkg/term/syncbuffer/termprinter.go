// Package syncbuffer provides a goroutine safe bytes.Buffer as well printing functionality to the terminal.
package syncbuffer

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/term/cursor"
	"golang.org/x/term"
)

// pollIntervalForSyncBuffer is the time Interval to wait between checking whether the output buffers are done.
const pollIntervalForSyncBuffer = 60 * time.Millisecond

// FileWriter is the interface to write to a file.
type FileWriter interface {
	io.Writer
}

// LabeledTermPrinter is a printer to display label and logs to the terminal.
type LabeledTermPrinter struct {
	term             FileWriter           // term writes logs to the terminal FileWriter.
	buffers          []*LabeledSyncBuffer // buffers stores logs before writing to the terminal.
	numLines         int                  // number of lines that has to be written from each buffer.
	padding          int                  // Leading spaces before rendering to terminal.
	prevWrittenLines int                  // number of lines written from all the buffers.
	termWidth        int                  // width of terminal.
}

// LabeledTermPrinterOption is a type alias to configure LabeledTermPrinter.
type LabeledTermPrinterOption func(ltp *LabeledTermPrinter)

// NewLabeledTermPrinter returns a LabeledTermPrinter that can print to the terminal filewriter from buffers.
func NewLabeledTermPrinter(fw FileWriter, bufs []*LabeledSyncBuffer, opts ...LabeledTermPrinterOption) (*LabeledTermPrinter, error) {
	width, err := terminalWidth()
	if err != nil {
		return nil, fmt.Errorf("get terminal width: %w", err)
	}

	ltp := &LabeledTermPrinter{
		term:      fw,
		buffers:   bufs,
		termWidth: width,
		numLines:  -1, // By default set numlines to -1 to print all from buffers.
	}
	for _, opt := range opts {
		opt(ltp)
	}
	return ltp, nil
}

// WithNumLines sets the numlines of LabeledTermPrinter.
func WithNumLines(n int) LabeledTermPrinterOption {
	return func(ltp *LabeledTermPrinter) {
		ltp.numLines = n
	}
}

// Withpadding sets the padding of LabeledTermPrinter.
func WithPadding(n int) LabeledTermPrinterOption {
	return func(ltp *LabeledTermPrinter) {
		ltp.padding = n
	}
}

// isDone returns true if all the buffers are done else returns false.
func (ltp *LabeledTermPrinter) isDone() bool {
	for _, buf := range ltp.buffers {
		if !buf.syncBuf.IsDone() {
			return false
		}
	}
	return true
}

// Print prints the label and the last N lines of logs from each buffer to the termPrinter fileWriter.
// If numLines is -1 then print all the values from buffers.
// It polls each buffer until all the buffers are marked done,
// and erases the previous output after sleeping for a short duration.
func (ltp *LabeledTermPrinter) Print() {
	if ltp.numLines == -1 {
		ltp.PrintAll()
		return
	}
	for {
		ltp.prevWrittenLines = 0
		for _, buf := range ltp.buffers {
			logs := buf.syncBuf.strings()
			if len(logs) == 0 {
				continue
			}
			outputLogs := ltp.lastNLines(logs)
			ltp.writeLines(buf.label, outputLogs)
			ltp.prevWrittenLines += ltp.calculateLinesCount(append(outputLogs, buf.label))
		}
		if ltp.isDone() {
			break
		}
		time.Sleep(pollIntervalForSyncBuffer)
		cursor.EraseLinesAbove(os.Stderr, ltp.prevWrittenLines)
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
		logLines[idx] = strings.TrimSpace(logs[start])
		start++
		idx++
	}
	return logLines
}

// writeLines writes a label and output logs to the terminal associated with the TermPrinter.
func (ltp *LabeledTermPrinter) writeLines(label string, outputLogs []string) {
	padding := ""
	if ltp.padding != 0 {
		padding = strings.Repeat(" ", ltp.padding)
	}
	fmt.Fprintln(ltp.term, label)
	for _, logLine := range outputLogs {
		fmt.Fprintln(ltp.term, padding, logLine)
	}
}

// calculateLinesCount returns the number of lines needed to print the given string slice based on the terminal width.
func (ltp *LabeledTermPrinter) calculateLinesCount(lines []string) int {
	var numLines float64
	for _, line := range lines {
		// Empty line should be considered as a new line
		if line == "" {
			numLines += 1
		}
		numLines += math.Ceil(float64(len(line)) / float64(ltp.termWidth))
	}
	return int(numLines)
}

// PrintAll writes the entire contents of all the buffers to the file writer.
// If one of the buffer gets done then print entire content of the buffer.
// Until all the buffers are written to file writer.
func (ltp *LabeledTermPrinter) PrintAll() {
	doneCount := 0
	for {
		for idx, buf := range ltp.buffers {
			if !buf.syncBuf.IsDone() {
				continue
			}
			outputLogs := ltp.buffers[idx].syncBuf.strings()
			ltp.writeLines(buf.label, outputLogs)
			doneCount++
		}
		if doneCount >= len(ltp.buffers) {
			break
		}
	}
}

// terminalWidth returns the width of the terminal or an error if failed to get the width of terminal.
func terminalWidth() (int, error) {
	width, _, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil {
		return 0, err
	}
	return width, nil
}

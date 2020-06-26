// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/term/cursor"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/briandowns/spinner"
)

// Events display settings.
const (
	minCellWidth           = 20  // minimum number of characters in a table's cell.
	tabWidth               = 4   // number of characters in between columns.
	cellPaddingWidth       = 2   // number of padding characters added by default to a cell.
	paddingChar            = ' ' // character in between columns.
	noAdditionalFormatting = 0
)

// TabRow represents a row in a table where columns are separated with a "\t" character.
type TabRow string

// startStopper is the interface to interact with the spinner.
type startStopper interface {
	Start()
	Stop()
}

// mover is the interface to interact with the cursor.
type mover interface {
	Up(n int)
	Down(n int)
	EraseLine()
}

type writeFlusher interface {
	io.Writer
	Flush() error
}

// Spinner represents an indicator that an asynchronous operation is taking place.
//
// For short operations, less than 4 seconds, display only the spinner with the Start and Stop methods.
// For longer operations, display intermediate progress events using the Events method.
type Spinner struct {
	spin startStopper
	cur  mover

	pastEvents   []TabRow     // Already written entries.
	eventsWriter writeFlusher // Writer to pretty format events in a table.
}

// NewSpinner returns a spinner that outputs to stderr.
func NewSpinner() *Spinner {
	s := spinner.New(charset, 125*time.Millisecond, spinner.WithHiddenCursor(true))
	s.Writer = log.DiagnosticWriter
	return &Spinner{
		spin:         s,
		cur:          cursor.New(),
		eventsWriter: tabwriter.NewWriter(s.Writer, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting),
	}
}

// Start starts the spinner suffixed with a label.
func (s *Spinner) Start(label string) {
	s.suffix(fmt.Sprintf(" %s", label))
	s.spin.Start()
}

// Stop stops the spinner and replaces it with a label.
func (s *Spinner) Stop(label string) {
	s.finalMSG(fmt.Sprintln(label))
	s.spin.Stop()

	// Maintain old progress entries on the screen.
	for _, event := range s.pastEvents {
		fmt.Fprintf(s.eventsWriter, "%s\n", event)
	}
	s.eventsWriter.Flush()
	// Reset event entries once the spinner stops.
	s.pastEvents = nil
}

// Events writes additional information below the spinner while the spinner is still in progress.
// If there are already existing events under the spinner, it replaces them with the new information.
//
// An event is displayed in a table, where columns are separated with the '\t' character.
func (s *Spinner) Events(events []TabRow) {
	done := make(chan struct{})
	go func() {
		s.lock()
		defer s.unlock()
		// Erase previous entries, and move the cursor back to the spinner.
		for i := 0; i < len(s.pastEvents); i++ {
			s.cur.Down(1)
			s.cur.EraseLine()
		}
		if len(s.pastEvents) > 0 {
			s.cur.Up(len(s.pastEvents))
		}

		// Add new status updates, and move cursor back to the spinner.
		for _, event := range events {
			fmt.Fprintf(s.eventsWriter, "\n%s", event)
		}
		s.eventsWriter.Flush()
		if len(events) > 0 {
			s.cur.Up(len(events))
		}
		// Move the cursor to the beginning so the spinner can delete the existing line.
		fmt.Fprintf(s.eventsWriter, "\r")
		s.eventsWriter.Flush()
		s.pastEvents = events
		close(done)
	}()
	<-done
}

func (s *Spinner) lock() {
	if spinner, ok := s.spin.(*spinner.Spinner); ok {
		spinner.Lock()
	}
}

func (s *Spinner) unlock() {
	if spinner, ok := s.spin.(*spinner.Spinner); ok {
		spinner.Unlock()
	}
}

func (s *Spinner) suffix(label string) {
	s.lock()
	defer s.unlock()
	if spinner, ok := s.spin.(*spinner.Spinner); ok {
		spinner.Suffix = label
	}
}

func (s *Spinner) finalMSG(label string) {
	s.lock()
	defer s.unlock()
	if spinner, ok := s.spin.(*spinner.Spinner); ok {
		spinner.FinalMSG = label
	}
}

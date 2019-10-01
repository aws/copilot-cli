// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package spinner provides a simple wrapper around github.combriandowns/spinner to start and stop a spinner in the terminal.
package spinner

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"
	"time"

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/term"
	spin "github.com/briandowns/spinner"
)

// Tip display settings.
const (
	minCellWidth           = 20  // minimum number of characters in a table's cell.
	tabWidth               = 4   // number of characters in between columns.
	cellPaddingWidth       = 2   // number of padding characters added by default to a cell.
	paddingChar            = ' ' // character in between columns.
	noAdditionalFormatting = 0
)

type spinner interface {
	Start()
	Stop()
}

type writeFlusher interface {
	io.Writer
	Flush() error
}

// Spinner is an indicator that a long operation is taking place.
type Spinner struct {
	internal spinner

	tips       []string     // additional information that's already written
	tipsWriter writeFlusher // writer to pretty format tips in a table
}

// New returns a Spinner that outputs to stderr.
func New() *Spinner {
	s := spin.New(charset, 125*time.Millisecond, spin.WithHiddenCursor(true))
	s.Writer = os.Stderr
	return &Spinner{
		internal:   s,
		tipsWriter: tabwriter.NewWriter(s.Writer, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting),
	}
}

// Start starts the spinner suffixed with a label.
func (s *Spinner) Start(label string) {
	s.suffix(fmt.Sprintf(" %s", label))
	s.internal.Start()
}

// Stop stops the spinner and replaces it with a label.
func (s *Spinner) Stop(label string) {
	s.finalMSG(fmt.Sprintf("%s\n", label))
	s.internal.Stop()

	s.lock()
	defer s.unlock()
	for _, tip := range s.tips {
		fmt.Fprintf(s.tipsWriter, "%s\n", tip)
	}
	s.tipsWriter.Flush()
}

// Tips writes additional information below the spinner while the spinner is still in progress.
// If there are already existing tips under the spinner, it replaces them with the new information.
//
// A tip is displayed in a table, where columns are separated with the '\t' character.
func (s *Spinner) Tips(tips []string) {
	done := make(chan struct{})
	go func() {
		s.lock()
		defer s.unlock()
		// Erase previous entries, and move the cursor back to the spinner.
		for i := 0; i < len(s.tips); i++ {
			fmt.Fprintf(s.tipsWriter, term.FmtMoveDown+"\r"+term.EraseLine, 1)
		}
		if len(s.tips) > 0 {
			fmt.Fprintf(s.tipsWriter, term.FmtMoveUp, len(s.tips))
		}

		// Add new status updates, and move cursor back to the spinner.
		for _, tip := range tips {
			fmt.Fprintf(s.tipsWriter, "\n%s", tip)
		}
		if len(tips) > 0 {
			fmt.Fprintf(s.tipsWriter, term.FmtMoveUp, len(tips))
		}
		s.tipsWriter.Flush()
		s.tips = tips
		close(done)
	}()
	<-done
}

func (s *Spinner) lock() {
	if spinner, ok := s.internal.(*spin.Spinner); ok {
		spinner.Lock()
	}
}

func (s *Spinner) unlock() {
	if spinner, ok := s.internal.(*spin.Spinner); ok {
		spinner.Unlock()
	}
}

func (s *Spinner) suffix(label string) {
	s.lock()
	defer s.unlock()
	if spinner, ok := s.internal.(*spin.Spinner); ok {
		spinner.Suffix = label
	}
}

func (s *Spinner) finalMSG(label string) {
	s.lock()
	defer s.unlock()
	if spinner, ok := s.internal.(*spin.Spinner); ok {
		spinner.FinalMSG = label
	}
}

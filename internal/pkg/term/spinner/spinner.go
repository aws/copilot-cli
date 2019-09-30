// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package spinner provides a simple wrapper around github.combriandowns/spinner to start and stop a spinner in the terminal.
package spinner

import (
	"fmt"
	"io"
	"os"
	"time"

	spin "github.com/briandowns/spinner"
)

type spinner interface {
	Start()
	Stop()
}

// Spinner wraps the spinner interface.
type Spinner struct {
	internal   spinner
	tipsWriter io.Writer
}

// New returns a Spinner that outputs to stderr.
func New() *Spinner {
	s := spin.New(charset, 125*time.Millisecond)
	s.Writer = os.Stderr
	return &Spinner{
		internal: s,
	}
}

// Start starts the spinner suffixed with a label.
func (s *Spinner) Start(label string) {
	if i, ok := s.internal.(*spin.Spinner); ok {
		i.Suffix = fmt.Sprintf(" %s", label)
	}
	s.internal.Start()
}

// Stop stops the spinner suffixed with a label.
func (s *Spinner) Stop(label string) {
	if i, ok := s.internal.(*spin.Spinner); ok {
		i.Lock()
		i.FinalMSG = fmt.Sprintf("%s\n", label)
		i.Unlock()
	}
	s.internal.Stop()
}

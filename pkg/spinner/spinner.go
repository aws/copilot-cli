// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package spinner provides a simple wrapper around github.combriandowns/spinner to start and stop a spinner in the terminal.
package spinner

import (
	"fmt"
	"time"

	spin "github.com/briandowns/spinner"
)

type spinner interface {
	Start()
	Stop()
}

// Spinner wraps the spinner interface.
type Spinner struct {
	internal spinner
}

// New returns a Spinner.
func New() Spinner {
	return Spinner{
		internal: spin.New(spin.CharSets[14], 125*time.Millisecond),
	}
}

// Start starts the spinner with the input message.
func (s Spinner) Start(msg string) {
	if i, ok := s.internal.(*spin.Spinner); ok {
		i.Suffix = fmt.Sprintf(" %s", msg)
	}

	s.internal.Start()
}

// Stop stops the spinner with the input message.
func (s Spinner) Stop(msg string) {
	if i, ok := s.internal.(*spin.Spinner); ok {
		i.Lock()
		i.FinalMSG = fmt.Sprintf("%s\n", msg)
		i.Unlock()
	}

	s.internal.Stop()
}

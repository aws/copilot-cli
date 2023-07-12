// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package signal provides a way to capture and handle operating system signals.
package signal

import (
	"os"
	"os/signal"
)

// Signal represents a signal handler for capturing and handling operating system signals.
type Signal struct {
	signalCh chan os.Signal
	sigs     []os.Signal
}

// New creates and returns new Signal object with the specified signals.
func New(signals ...os.Signal) *Signal {
	return &Signal{
		signalCh: make(chan os.Signal),
		sigs:     signals,
	}
}

// NotifySignals starts capturing the specified signals and returns a channel for receiving them.
func (s *Signal) NotifySignals() <-chan os.Signal {
	signal.Notify(s.signalCh, s.sigs...)
	return s.signalCh
}

// StopCatchSignals stops capturing signals and closes the signalChannel.
func (s *Signal) StopCatchSignals() {
	signal.Stop(s.signalCh)
	close(s.signalCh)
}

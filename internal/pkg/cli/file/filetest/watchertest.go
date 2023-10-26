// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package filetest

import "github.com/fsnotify/fsnotify"

// Double is a test double for file.RecursiveWatcher
type Double struct {
	EventsFn func() <-chan fsnotify.Event
	ErrorsFn func() <-chan error
}

// Add is a no-op for Double.
func (d *Double) Add(string) error {
	return nil
}

// Close is a no-op for Double.
func (d *Double) Close() error {
	return nil
}

// Events calls the stubbed function.
func (d *Double) Events() <-chan fsnotify.Event {
	if d.EventsFn == nil {
		return nil
	}
	return d.EventsFn()
}

// Errors calls the stubbed function.
func (d *Double) Errors() <-chan error {
	if d.ErrorsFn == nil {
		return nil
	}
	return d.ErrorsFn()
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package syncbuffer provides a goroutine safe bytes.Buffer as well printing functionality to the terminal.
package syncbuffer

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
)

// SyncBuffer is a synchronized buffer that can be used to store output data and coordinate between multiple goroutines.
type SyncBuffer struct {
	bufMu sync.Mutex    // bufMu is a mutex protects buf.
	buf   bytes.Buffer  // buf is the buffer that stores the data.
	done  chan struct{} // is closed after MarkDone() is called.
}

// New creates and returns a new SyncBuffer object with an initialized 'done' channel.
func New() *SyncBuffer {
	return &SyncBuffer{
		done: make(chan struct{}),
	}
}

// Write appends the given bytes to the buffer.
func (b *SyncBuffer) Write(p []byte) (n int, err error) {
	b.bufMu.Lock()
	defer b.bufMu.Unlock()
	return b.buf.Write(p)
}

// IsDone returns true if the Done channel has been closed, otherwise return false.
func (b *SyncBuffer) IsDone() bool {
	select {
	case <-b.done:
		return true
	default:
		return false
	}
}

// MarkDone closes the Done channel.
func (b *SyncBuffer) MarkDone() {
	close(b.done)
}

// LabeledSyncBuffer is a struct that combines a SyncBuffer with a string label.
type LabeledSyncBuffer struct {
	label string
	*SyncBuffer
}

// WithLabel creates and returns a new LabeledSyncBuffer with the given label and SyncBuffer.
func (buf *SyncBuffer) WithLabel(label string) *LabeledSyncBuffer {
	return &LabeledSyncBuffer{
		label:      label,
		SyncBuffer: buf,
	}
}

// Copy reads all the content of an io.Reader into a SyncBuffer and an error if copy is failed.
func (buf *SyncBuffer) Copy(r io.Reader) error {
	defer buf.MarkDone()
	_, err := io.Copy(buf, r)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// lines returns an empty slice if the buffer is empty.
// Otherwise, it returns a slice of all the lines stored in the buffer.
func (b *SyncBuffer) lines() []string {
	b.bufMu.Lock()
	defer b.bufMu.Unlock()
	lines := b.buf.String()
	if len(lines) == 0 {
		return nil
	}
	return splitLinesAndTrimSpaces(lines)
}

// splitLinesAndTrimSpaces splits the input string into lines
// and trims the leading and trailing spaces and returns slice of strings.
func splitLinesAndTrimSpaces(input string) []string {
	lines := strings.Split(input, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return lines
}

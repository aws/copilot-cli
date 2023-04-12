// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package syncbuffer provides a goroutine safe bytes.Buffer as well printing functionality to the terminal.
package syncbuffer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
)

// syncBuffer is a synchronized buffer that can be used to store output data and coordinate between multiple goroutines.
type syncBuffer struct {
	bufMu sync.Mutex    // bufMu is a mutex protects buf.
	buf   bytes.Buffer  // buf is the buffer that stores the data.
	done  chan struct{} // is closed after MarkDone() is called.
}

// New creates and returns a new syncBuffer object with an initialized 'done' channel.
func New() *syncBuffer {
	return &syncBuffer{
		done: make(chan struct{}),
	}
}

// Write appends the given bytes to the buffer.
func (b *syncBuffer) Write(p []byte) (n int, err error) {
	b.bufMu.Lock()
	defer b.bufMu.Unlock()
	return b.buf.Write(p)
}

// IsDone returns true if the Done channel has been closed, otherwise return false.
func (b *syncBuffer) IsDone() bool {
	select {
	case <-b.done:
		return true
	default:
		return false
	}
}

// MarkDone closes the Done channel.
func (b *syncBuffer) MarkDone() {
	close(b.done)
}

// LabeledSyncBuffer is a struct that combines a SyncBuffer with a string label.
type LabeledSyncBuffer struct {
	label string
	*syncBuffer
}

// WithLabel creates and returns a new LabeledSyncBuffer with the given label and SyncBuffer.
func (buf *syncBuffer) WithLabel(label string) *LabeledSyncBuffer {
	return &LabeledSyncBuffer{
		label:      label,
		syncBuffer: buf,
	}
}

// Copy reads all the content of an io.Reader into a SyncBuffer and returns it.
func Copy(syncBuf *syncBuffer, r io.Reader) error {
	defer syncBuf.MarkDone()
	_, err := io.Copy(&syncBuf.buf, r)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to copy to buffer: %w", err)
	}
	return nil
}

// lines returns an empty slice if the buffer is empty.
// Otherwise, it returns a slice of all the lines stored in the buffer.
func (b *syncBuffer) lines() []string {
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

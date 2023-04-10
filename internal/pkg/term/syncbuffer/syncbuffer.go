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

// The fileReader interface that can read data from a file.
// It extends the io.Reader interface, which provides a method for reading bytes into a buffer.
type fileReader interface {
	io.Reader
}

// SyncBuffer is a synchronized buffer that can be used to store output data and coordinate between multiple goroutines.
type SyncBuffer struct {
	bufMu sync.Mutex    // bufMu is a mutex that can be used to protect access to the buffer.
	buf   bytes.Buffer  // buf is the buffer that stores the data.
	done  chan struct{} // done is a channel that can be used to signal when the operations are complete.
}

// NewSyncBuffer creates and returns a new SyncBuffer object with an initialized 'done' channel.
func NewSyncBuffer() *SyncBuffer {
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

// strings returns an empty slice if the buffer is empty.
// Otherwise, it returns a slice of all the lines stored in the buffer.
func (b *SyncBuffer) strings() []string {
	b.bufMu.Lock()
	defer b.bufMu.Unlock()
	lines := b.buf.String()
	if len(lines) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSpace(lines), "\n")
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

// CopyToBuffer reads from a fileReader and writes the data to the buffer.
func (b *SyncBuffer) CopyToBuffer(r fileReader) error {
	return copyFileToBuffer(b, r)
}

// LabeledSyncBuffer is a struct that combines a SyncBuffer with a string label.
type LabeledSyncBuffer struct {
	label   string
	syncBuf *SyncBuffer
}

// New creates and returns a new LabeledSyncBuffer without label.
func New(syncBuf *SyncBuffer) *LabeledSyncBuffer {
	return &LabeledSyncBuffer{
		syncBuf: syncBuf,
	}
}

// NewLabeledSyncBufferWithLabel creates and returns a new LabeledSyncBuffer with the given label and SyncBuffer.
func NewWithLabel(label string, syncBuf *SyncBuffer) *LabeledSyncBuffer {
	return &LabeledSyncBuffer{
		label:   label,
		syncBuf: syncBuf,
	}
}

// CopyToLabeledBuffer reads from a fileReader and writes the data to the buffer of LabeledSyncBuffer.
func (b *LabeledSyncBuffer) CopyToLabeledBuffer(r fileReader) error {
	return b.syncBuf.CopyToBuffer(r)
}

// copyFileToBuffer copies the contents of the given fileReader into the buffer.
// It returns an error if the copy operation fails or encounters a non-EOF error.
func copyFileToBuffer(b *SyncBuffer, r fileReader) error {
	defer func() {
		b.MarkDone()
	}()
	_, err := io.Copy(&b.buf, r)
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to copy to buffer: %w", err)
	}
	return nil
}

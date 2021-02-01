// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"io"
	"text/tabwriter"
)

// FileWriter is the interface grouping an io.Writer with the file descriptor method Fd.
// Files in the OS, like os.Stderr, implement the FileWriter interface.
type FileWriter interface {
	io.Writer
	Fd() uintptr
}

// WriteFlusher is the interface grouping an io.Writer with the Flush method.
// Flush allows writing buffered writes from Writer all at once.
type WriteFlusher interface {
	io.Writer
	Flush() error
}

// FileWriteFlusher is the interface that groups a FileWriter and WriteFlusher.
type FileWriteFlusher interface {
	FileWriter
	WriteFlusher
}

// TabbedFileWriter is a FileWriter that also implements the WriteFlusher interface.
// A TabbedFileWriter can properly align text separated by the '\t' character.
type TabbedFileWriter struct {
	FileWriter
	WriteFlusher
}

func (w *TabbedFileWriter) Write(p []byte) (n int, err error) {
	return w.WriteFlusher.Write(p)
}

// NewTabbedFileWriter takes a file as input and returns a FileWriteFlusher that can
// properly write tab-separated text to it.
func NewTabbedFileWriter(fw FileWriter) *TabbedFileWriter {
	return &TabbedFileWriter{
		FileWriter:   fw,
		WriteFlusher: tabwriter.NewWriter(fw, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting),
	}
}

// suffixWriter is an io.Writer that adds the suffix before a new line character.
type suffixWriter struct {
	buf    io.Writer
	suffix []byte
}

// Write adds a suffix before each new line character in p.
func (w *suffixWriter) Write(p []byte) (n int, err error) {
	var withSuffix []byte
	for _, b := range p {
		suffix := []byte{b}
		if b == '\n' {
			suffix = append(w.suffix, b)
		}
		withSuffix = append(withSuffix, suffix...)
	}
	if _, err := w.buf.Write(withSuffix); err != nil {
		return 0, err
	}
	return len(p), nil
}

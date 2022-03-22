// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cursor provides functionality to interact with the terminal cursor.
package cursor

import (
	"io"
	"os"

	"github.com/AlecAivazis/survey/v2/terminal"
)

type cursor interface {
	Up(n int) error
	Down(n int) error
	Hide() error
	Show() error
}

// fakeFileWriter is a terminal.FileWriter.
// If the underlying writer w does not implement Fd() then a dummy value is returned.
type fakeFileWriter struct {
	w io.Writer
}

// Write delegates to the internal writer.
func (w *fakeFileWriter) Write(p []byte) (int, error) {
	return w.w.Write(p)
}

// Fd is required to be implemented to satisfy the terminal.FileWriter interface.
// If the underlying writer is a file, like os.Stdout, then invoke it. Otherwise, this method allows us to create
// a Cursor that can write to any io.Writer like a bytes.Buffer by returning a dummy value.
func (w *fakeFileWriter) Fd() uintptr {
	if v, ok := w.w.(terminal.FileWriter); ok {
		return v.Fd()
	}
	return 0
}

// Cursor represents the terminal's cursor.
type Cursor struct {
	c cursor
}

// New creates a new cursor that writes to stderr.
func New() *Cursor {
	return &Cursor{
		c: &terminal.Cursor{
			Out: os.Stderr,
		},
	}
}

// NewWithWriter creates a new cursor that writes to the given out writer.
func NewWithWriter(out io.Writer) *Cursor {
	return &Cursor{
		c: &terminal.Cursor{
			Out: &fakeFileWriter{w: out},
		},
	}
}

// Hide best-effort makes the cursor invisible.
func (c *Cursor) Hide() {
	_ = c.c.Hide()
}

// Show best-effort makes the cursor visible.
func (c *Cursor) Show() {
	_ = c.c.Show()
}

// EraseLine best-effort deletes the contents of the current line.
func (c *Cursor) EraseLine() {
	if cur, ok := c.c.(*terminal.Cursor); ok {
		_ = terminal.EraseLine(cur.Out, terminal.ERASE_LINE_ALL)
	}
}

// EraseLine best-effort erases a line from a FileWriter.
func EraseLine(fw terminal.FileWriter) {
	_ = terminal.EraseLine(fw, terminal.ERASE_LINE_ALL)
}

// EraseLinesAbove erases a line and moves the cursor up from fw, repeated n times.
func EraseLinesAbove(fw terminal.FileWriter, n int) {
	c := Cursor{
		c: &terminal.Cursor{
			Out: fw,
		},
	}
	for i := 0; i < n; i += 1 {
		EraseLine(fw)
		c.Up(1)
	}
	EraseLine(fw) // Erase the nth line as well.
}

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
	Up(n int)
	Down(n int)
	Hide()
	Show()
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

// New creates a new cursor that writes to the given out writer.
func NewWithWriter(out io.Writer) *Cursor {
	return &Cursor{
		c: &terminal.Cursor{
			Out: &fakeFileWriter{w: out},
		},
	}
}

// Up moves the cursor n lines.
func (c *Cursor) Up(n int) {
	c.c.Up(n)
}

// Down moves the cursor n lines.
func (c *Cursor) Down(n int) {
	c.c.Down(n)
}

// Hide makes the cursor invisible.
func (c *Cursor) Hide() {
	c.c.Hide()
}

// Show makes the cursor visible.
func (c *Cursor) Show() {
	c.c.Show()
}

// EraseLine deletes the contents of the current line.
func (c *Cursor) EraseLine() {
	if cur, ok := c.c.(*terminal.Cursor); ok {
		terminal.EraseLine(cur.Out, terminal.ERASE_LINE_ALL)
	}
}

// EraseLine erases a line from the writer w.
func EraseLine(w io.Writer) {
	var out terminal.FileWriter = &fakeFileWriter{w: w}
	if term, ok := w.(terminal.FileWriter); ok { // If w is a file, like Stdout, then use the fileWriter instead.
		out = term
	}
	terminal.EraseLine(out, terminal.ERASE_LINE_ALL)
}

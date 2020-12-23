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

// fakeFileWriter is a terminal.FileWriter that delegates all writes to w and returns a dummy value for the FileDescriptor.
type fakeFileWriter struct {
	w io.Writer
}

// Write delegates to the internal writer.
func (w *fakeFileWriter) Write(p []byte) (int, error) {
	return w.w.Write(p)
}

// Fd is required to be implemented to satisfy the terminal.FileWriter interface.
// Unfortunately, the terminal.Cursor struct requires an input that satisfies this method although it does not call it
// internally, so we can return whatever value that we want.
// They should have instead taken a dependency only on io.Writer.
func (w *fakeFileWriter) Fd() uintptr {
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

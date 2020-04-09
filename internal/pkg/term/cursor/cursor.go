// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cursor provides functionality to interact with the terminal cursor.
package cursor

import (
	"os"

	"github.com/AlecAivazis/survey/v2/terminal"
)

type cursor interface {
	Up(n int)
	Down(n int)
}

// Cursor represents the terminal's cursor.
type Cursor struct {
	c cursor
}

// New creates a new cursor that writes to stderr and reads from stdin.
func New() *Cursor {
	return &Cursor{
		c: &terminal.Cursor{
			In:  os.Stdin,
			Out: os.Stderr,
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

// EraseLine deletes the contents of the current line.
func (c *Cursor) EraseLine() {
	if cur, ok := c.c.(*terminal.Cursor); ok {
		terminal.EraseLine(cur.Out, terminal.ERASE_LINE_ALL)
	}
}

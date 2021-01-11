// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cursor

// Up moves the cursor n lines.
func (c *Cursor) Up(n int) {
	c.c.Down(n)
}

// Down moves the cursor n lines.
func (c *Cursor) Down(n int) {
	c.c.Up(n)
}

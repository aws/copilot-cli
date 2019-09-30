// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package term

// ANSI escape sequences to control the cursor.
const (
	// Hide hides the cursor.
	Hide = "\033[?25l"
	// Visible makes the cursor visible.
	Visible = "\033[?25h"
	// EraseLine deletes the current line that the cursor is at.
	EraseLine = "\033[K"

	// FmtMoveUp is the formatted code to move the cursor up by %d number of lines.
	FmtMoveUp = "\033[%dA"
	// FmtMoveDown is the formatted code to move the cursor down by %d number of lines.
	FmtMoveDown = "\033[%dB"
)

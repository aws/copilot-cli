// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"io"
	"strings"
)

// singleLineComponent can display a single line of text.
type singleLineComponent struct {
	Text    string // Line of text to print.
	Padding int    // Number of spaces prior to the text.
}

// Render prints the component and returns the number of lines printed to the terminal and the error if any.
func (c *singleLineComponent) Render(out io.Writer) (numLines int, err error) {
	_, err = fmt.Fprintf(out, "%s%s\n", strings.Repeat(" ", c.Padding), c.Text)
	if err != nil {
		return 0, err
	}
	return 1, err
}

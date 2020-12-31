// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"context"
	"io"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/term/cursor"
)

const (
	renderInterval = 100 * time.Millisecond // How frequently Render should be invoked.
)

// Renderer is the interface to print a component to a writer.
// It returns the number of lines printed and the error if any.
type Renderer interface {
	Render(out io.Writer) (numLines int, err error)
}

// Render renders r periodically to out until the ctx is canceled or an error occurs.
// While Render is executing, the terminal cursor is hidden and updates are written in-place.
func Render(ctx context.Context, out WriteFlusher, r Renderer) error {
	cursor := cursor.NewWithWriter(out)
	cursor.Hide()
	defer cursor.Show()

	for {
		select {
		case <-ctx.Done():
			if _, err := r.Render(out); err != nil {
				return err
			}
			return out.Flush()
		case <-time.After(renderInterval):
			nl, err := r.Render(out)
			if err != nil {
				return err
			}
			if err := out.Flush(); err != nil {
				return err
			}
			cursor.Up(nl) // move the cursor back up to the starting line so that the Renderer is rendered in-place.
		}
	}
}

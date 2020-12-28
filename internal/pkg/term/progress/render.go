// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"bytes"
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
// While Render is executing, the terminal cursor is hidden and moved up after each call so that
// the updates are written in-place.
func Render(ctx context.Context, out io.Writer, r Renderer) error {
	cursor := cursor.NewWithWriter(out)
	cursor.Hide()
	defer cursor.Show()
	for {
		select {
		case <-ctx.Done():
			_, err := r.Render(out)
			return err
		case <-time.After(renderInterval):
			nl, err := r.Render(out)
			if err != nil {
				return err
			}
			cursor.Up(nl + 1) // move the cursor back up to the starting line so that the Renderer is rendered in-place.
		}
	}
}

// allOrNothingRenderer renders all partial renders or none of them.
type allOrNothingRenderer struct {
	err      error
	numLines int
	buf      bytes.Buffer
}

func (r *allOrNothingRenderer) Partial(renderer Renderer) {
	if r.err != nil {
		return
	}
	nl, err := renderer.Render(&r.buf)
	if err != nil {
		r.err = err
		return
	}
	r.numLines += nl
}

func (r *allOrNothingRenderer) Render(w io.Writer) (numLines int, err error) {
	if r.err != nil {
		return 0, r.err
	}
	if _, err := r.buf.WriteTo(w); err != nil {
		return 0, err
	}
	return r.numLines, nil
}

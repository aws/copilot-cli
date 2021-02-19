// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"context"
	"io"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/term/cursor"
)

// Renderer is the interface to print a component to a writer.
// It returns the number of lines printed and the error if any.
type Renderer interface {
	Render(out io.Writer) (numLines int, err error)
}

// DynamicRenderer is a Renderer that can notify that its internal states are Done updating.
// DynamicRenderer is implemented by components that listen to events from a streamer and update their state.
type DynamicRenderer interface {
	Renderer
	Done() <-chan struct{}
}

// RenderOptions holds optional style configuration for renderers.
type RenderOptions struct {
	Padding int // Leading spaces before rendering the component.
}

// NestedRenderOptions takes a RenderOptions and returns the same RenderOptions but with additional padding.
func NestedRenderOptions(opts RenderOptions) RenderOptions {
	return RenderOptions{
		Padding: opts.Padding + nestedComponentPadding,
	}
}

// Render renders r periodically to out.
// Render stops when there the ctx is canceled or r is done listening to new events.
// While Render is executing, the terminal cursor is hidden and updates are written in-place.
func Render(ctx context.Context, out FileWriteFlusher, r DynamicRenderer) error {
	defer out.Flush() // Make sure every buffered text in out is written before exiting.

	cursor := cursor.NewWithWriter(out)
	cursor.Hide()
	defer cursor.Show()

	var writtenLines int
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.Done():
			if _, err := eraseAndRender(out, r, writtenLines); err != nil {
				return err
			}
			return nil
		case <-time.After(renderInterval):
			nl, err := eraseAndRender(out, r, writtenLines)
			if err != nil {
				return err
			}
			writtenLines = nl
		}
	}
}

// eraseAndRender erases prevNumLines from out and then renders r.
func eraseAndRender(out FileWriteFlusher, r Renderer, prevNumLines int) (int, error) {
	cursor.EraseLinesAbove(out, prevNumLines)
	if err := out.Flush(); err != nil {
		return 0, err
	}
	nl, err := r.Render(out)
	if err != nil {
		return 0, err
	}
	if err := out.Flush(); err != nil {
		return 0, err
	}
	return nl, err
}

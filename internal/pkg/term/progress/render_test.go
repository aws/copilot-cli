// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/term/cursor"
	"github.com/stretchr/testify/require"
)

type mockRenderer struct {
	content string
	err     error
}

func (m *mockRenderer) Render(out io.Writer) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	out.Write([]byte(m.content))
	return 1, nil
}

func TestRender(t *testing.T) {
	// GIVEN
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	actual := new(strings.Builder)
	r := &mockRenderer{content: "hi\n"}

	// WHEN
	err := Render(ctx, actual, r)

	// THEN
	require.NoError(t, err)

	// We should be doing the following operations in order:
	// 1. Hide the cursor.
	// 2. Write "hi\n" and move the cursor up (Repeated x3 times)
	// 3. The <-ctx.Done() is called so we should write one last time "hi\n" and the cursor should be shown.
	wanted := new(strings.Builder)
	c := cursor.NewWithWriter(wanted)
	c.Hide()
	wanted.WriteString("hi\n")
	c.Up(1)
	wanted.WriteString("hi\n")
	c.Up(1)
	wanted.WriteString("hi\n")
	c.Up(1)
	wanted.WriteString("hi\n")
	c.Show()

	require.Equal(t, wanted.String(), actual.String(), "expected the content printed to match")
}

func TestAllOrNothingRenderer(t *testing.T) {
	t.Run("should render all partials when no partial fails", func(t *testing.T) {
		// GIVEN
		r := new(allOrNothingRenderer)
		buf := new(strings.Builder)

		// WHEN
		r.Partial(&mockRenderer{
			content: "hello\n",
		})
		r.Partial(&mockRenderer{
			content: "world\n",
		})
		numLines, err := r.Render(buf)

		// THEN
		require.NoError(t, err)
		require.Equal(t, "hello\nworld\n", buf.String())
		require.Equal(t, 2, numLines, "expected two entries to be rendered")
	})
	t.Run("should not render any partials if one of them fails", func(t *testing.T) {
		// GIVEN
		r := new(allOrNothingRenderer)
		buf := new(strings.Builder)
		wantedErr := errors.New("some error")

		// WHEN
		r.Partial(&mockRenderer{
			content: "hello\n",
		})
		r.Partial(&mockRenderer{
			err: wantedErr,
		})
		r.Partial(&mockRenderer{
			content: "world\n",
		})
		numLines, err := r.Render(buf)

		// THEN
		require.EqualError(t, err, wantedErr.Error(), "expected partial error to be surfaced")
		require.Equal(t, 0, numLines, "expected no lines to be rendered")
	})
	t.Run("should surface first partial error", func(t *testing.T) {
		// GIVEN
		r := new(allOrNothingRenderer)
		buf := new(strings.Builder)
		firstErr := errors.New("error1")
		secondErr := errors.New("error2")

		// WHEN
		r.Partial(&mockRenderer{
			err: firstErr,
		})
		r.Partial(&mockRenderer{
			err: secondErr,
		})
		numLines, err := r.Render(buf)

		// THEN
		require.EqualError(t, err, firstErr.Error(), "expected only first error to be returned")
		require.Equal(t, 0, numLines, "expected no lines to be rendered")
	})
}

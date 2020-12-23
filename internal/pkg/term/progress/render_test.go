// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"errors"
	"io"
	"strings"
	"testing"

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
	out.Write([]byte(m.content + "\n"))
	return 1, nil
}

func TestAllOrNothingRenderer(t *testing.T) {
	t.Run("should render all partials when no partial fails", func(t *testing.T) {
		// GIVEN
		r := new(allOrNothingRenderer)
		buf := new(strings.Builder)

		// WHEN
		r.Partial(&mockRenderer{
			content: "hello",
		})
		r.Partial(&mockRenderer{
			content: "world",
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
			content: "hello",
		})
		r.Partial(&mockRenderer{
			err: wantedErr,
		})
		r.Partial(&mockRenderer{
			content: "world",
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

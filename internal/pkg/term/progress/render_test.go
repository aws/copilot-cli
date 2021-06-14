// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aws/copilot-cli/internal/pkg/term/cursor"
	"github.com/stretchr/testify/require"
)

type mockDynamicRenderer struct {
	content string
	done    chan struct{}
	err     error
}

func (m *mockDynamicRenderer) Render(out io.Writer) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	out.Write([]byte(m.content))
	return 1, nil
}

func (m *mockDynamicRenderer) Done() <-chan struct{} {
	return m.done
}

type mockFileWriteFlusher struct {
	buf     bytes.Buffer
	wrapper io.Writer
}

func (m *mockFileWriteFlusher) Write(p []byte) (n int, err error) {
	return m.buf.Write(p)
}

func (m *mockFileWriteFlusher) Fd() uintptr {
	return 0
}

func (m *mockFileWriteFlusher) Flush() error {
	if _, err := m.buf.WriteTo(m.wrapper); err != nil {
		return err
	}
	return nil
}

type mockFileWriter struct {
	io.Writer
}

func (m *mockFileWriter) Fd() uintptr {
	return 0
}

func TestRender(t *testing.T) {
	renderInterval = 100 * time.Millisecond // Ensure that even when CI=true we are testing with default interval.

	t.Run("stops the renderer when context is canceled", func(t *testing.T) {
		t.Parallel()
		// GIVEN
		ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
		defer cancel()
		actual := new(strings.Builder)
		r := &mockDynamicRenderer{
			content: "hi\n",
			done:    make(chan struct{}),
		}
		out := &mockFileWriteFlusher{
			wrapper: actual,
		}

		// WHEN
		err := Render(ctx, out, r)

		// THEN
		require.EqualError(t, err, ctx.Err().Error(), "expected the context to be canceled")
		require.Contains(t, actual.String(), "hi\n", "expected Render to be invoked until the context was canceled")
	})
	t.Run("keeps rendering until the renderer is done", func(t *testing.T) {
		t.Parallel()
		// GIVEN
		actual := new(strings.Builder)
		done := make(chan struct{})
		r := &mockDynamicRenderer{
			content: "hi\n",
			done:    done,
		}
		out := &mockFileWriteFlusher{
			wrapper: actual,
		}
		go func() {
			<-time.After(350 * time.Millisecond)
			close(done)
		}()

		// WHEN
		err := Render(context.Background(), out, r)

		// THEN
		require.NoError(t, err)

		// We should be doing the following operations in order:
		// 1. Hide the cursor.
		// 2. Write "hi\n", erase the line, and move the cursor up (Repeated x3 times)
		// 3. The <-ctx.Done() is called so we should write one last time "hi\n" and the cursor should be shown.
		wanted := new(strings.Builder)
		wantedFW := &mockFileWriter{
			Writer: wanted,
		}
		c := cursor.NewWithWriter(wantedFW)
		c.Hide()
		cursor.EraseLine(wantedFW)
		wanted.WriteString("hi\n")
		cursor.EraseLine(wantedFW)
		c.Up(1)
		cursor.EraseLine(wantedFW)
		wanted.WriteString("hi\n")
		cursor.EraseLine(wantedFW)
		c.Up(1)
		cursor.EraseLine(wantedFW)
		wanted.WriteString("hi\n")
		cursor.EraseLine(wantedFW)
		c.Up(1)
		cursor.EraseLine(wantedFW)
		wanted.WriteString("hi\n")
		c.Show()

		require.Equal(t, wanted.String(), actual.String(), "expected the content printed to match")
	})

}

func TestNestedRenderOptions(t *testing.T) {
	// GIVEN
	opts := RenderOptions{}

	// WHEN
	actual := NestedRenderOptions(opts)

	// THEN
	require.Equal(t, RenderOptions{
		Padding: 2,
	}, actual)
}

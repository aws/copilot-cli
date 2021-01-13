// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"context"
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

type mockFileWriteFlusher struct {
	io.Writer
}

func (m *mockFileWriteFlusher) Fd() uintptr {
	return 0
}

func (m *mockFileWriteFlusher) Flush() error {
	return nil
}

func TestRender(t *testing.T) {
	// GIVEN
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	defer cancel()
	actual := new(strings.Builder)
	r := &mockRenderer{content: "hi\n"}
	out := &mockFileWriteFlusher{
		Writer: actual,
	}

	// WHEN
	err := Render(ctx, out, r)

	// THEN
	require.NoError(t, err)

	// We should be doing the following operations in order:
	// 1. Hide the cursor.
	// 2. Write "hi\n", erase the line, and move the cursor up (Repeated x3 times)
	// 3. The <-ctx.Done() is called so we should write one last time "hi\n" and the cursor should be shown.
	wanted := new(strings.Builder)
	wantedFW := &mockFileWriteFlusher{
		Writer: wanted,
	}
	c := cursor.NewWithWriter(wantedFW)
	c.Hide()
	wanted.WriteString("hi\n")
	cursor.EraseLine(wantedFW)
	c.Up(1)
	wanted.WriteString("hi\n")
	cursor.EraseLine(wantedFW)
	c.Up(1)
	wanted.WriteString("hi\n")
	cursor.EraseLine(wantedFW)
	c.Up(1)
	wanted.WriteString("hi\n")
	c.Show()

	require.Equal(t, wanted.String(), actual.String(), "expected the content printed to match")
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

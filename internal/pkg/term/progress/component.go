// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

const (
	nestedComponentPadding = 2 // Leading space characters for rendering a nested component.
)

// noopComponent satisfies the Renderer interface but does not write anything.
type noopComponent struct{}

// Render does not do anything.
// It returns 0 and nil for the error.
func (c *noopComponent) Render(_ io.Writer) (numLines int, err error) {
	return 0, nil
}

// LineRenderer returns a Renderer that can display a single line of text.
func LineRenderer(text string, padding int) Renderer {
	return &singleLineComponent{
		Text:    text,
		Padding: padding,
	}
}

// singleLineComponent can display a single line of text.
type singleLineComponent struct {
	Text    string // Line of text to print.
	Padding int    // Number of spaces prior to the text.
}

// Render writes the Text with a newline to out and returns 1 for the number of lines written.
// In case of an error, returns 0 and the error.
func (c *singleLineComponent) Render(out io.Writer) (numLines int, err error) {
	_, err = fmt.Fprintf(out, "%s%s\n", strings.Repeat(" ", c.Padding), c.Text)
	if err != nil {
		return 0, err
	}
	return 1, err
}

// treeComponent can display a node and its Children.
type treeComponent struct {
	Root     Renderer
	Children []Renderer
}

// Render writes the Root and its Children in order to out. Returns the total number of lines written.
// In case of an error, returns 0 and the error.
func (c *treeComponent) Render(out io.Writer) (numLines int, err error) {
	return renderComponents(out, append([]Renderer{c.Root}, c.Children...))
}

// dynamicTreeComponent is a treeComponent that can notify that it's done updating once the Root node is done.
type dynamicTreeComponent struct {
	Root     DynamicRenderer
	Children []Renderer
}

// Render creates a treeComponent and renders it.
func (c *dynamicTreeComponent) Render(out io.Writer) (numLines int, err error) {
	comp := &treeComponent{
		Root:     c.Root,
		Children: c.Children,
	}
	return comp.Render(out)
}

// Done return a channel that is closed when the children and root are done.
func (c *dynamicTreeComponent) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		for _, child := range c.Children {
			if dr, ok := child.(DynamicRenderer); ok {
				<-dr.Done() // Wait for children to be closed.
			}
		}
		<-c.Root.Done() // Then wait for the root to be closed.
		close(done)
	}()
	return done
}

// tableComponent can display a table.
type tableComponent struct {
	Title  string     // The table's label.
	Header []string   // Titles for the columns.
	Rows   [][]string // The table's data.

	Padding      int  // Number of leading spaces before writing the Title.
	MinCellWidth int  // Minimum number of characters per table cell.
	GapWidth     int  // Number of characters between columns.
	ColumnChar   byte // Character that separates columns.
}

// newTableComponent returns a small table component with no padding, that uses the space character ' ' to
// separate columns, and has two spaces between columns.
func newTableComponent(title string, header []string, rows [][]string) *tableComponent {
	return &tableComponent{
		Title:        title,
		Header:       header,
		Rows:         rows,
		Padding:      0,
		MinCellWidth: 2,
		GapWidth:     2,
		ColumnChar:   ' ',
	}
}

// Render writes the table to out.
// If there are no rows, the table is not rendered.
func (c *tableComponent) Render(out io.Writer) (numLines int, err error) {
	if len(c.Rows) == 0 {
		return 0, nil
	}

	// Write the table's title.
	buf := new(bytes.Buffer)
	if _, err := buf.WriteString(fmt.Sprintf("%s%s\n", strings.Repeat(" ", c.Padding), c.Title)); err != nil {
		return 0, fmt.Errorf("write title %s to buffer: %w", c.Title, err)
	}
	numLines += 1

	// Write rows.
	tw := tabwriter.NewWriter(buf, c.MinCellWidth, c.GapWidth, c.MinCellWidth, c.ColumnChar, noAdditionalFormatting)
	rows := append([][]string{c.Header}, c.Rows...)
	for _, row := range rows {
		// Pad the table to the right under the Title.
		line := fmt.Sprintf("%s%s\n", strings.Repeat(" ", c.Padding+nestedComponentPadding), strings.Join(row, "\t"))
		if _, err := tw.Write([]byte(line)); err != nil {
			return 0, fmt.Errorf("write row %s to tabwriter: %w", line, err)
		}
	}
	if err := tw.Flush(); err != nil {
		return 0, fmt.Errorf("flush tabwriter: %w", err)
	}
	numLines += len(rows)

	// Flush everything to out.
	if _, err := buf.WriteTo(out); err != nil {
		return 0, fmt.Errorf("write buffer to out: %w", err)
	}
	return numLines, nil
}

func renderComponents(out io.Writer, components []Renderer) (numLines int, err error) {
	buf := new(bytes.Buffer)
	for _, comp := range components {
		nl, err := comp.Render(buf)
		if err != nil {
			return 0, err
		}
		numLines += nl
	}
	if _, err := buf.WriteTo(out); err != nil {
		return 0, err
	}
	return numLines, nil
}

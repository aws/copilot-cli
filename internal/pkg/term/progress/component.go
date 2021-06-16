// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/dustin/go-humanize/english"
)

const (
	nestedComponentPadding = 2 // Leading space characters for rendering a nested component.
)

// noopComponent satisfies the Renderer interface but does not write anything.
type noopComponent struct{}

// Render does not do anything.
// It returns 0 and nil for the error.
func (c *noopComponent) Render(out io.Writer) (numLines int, err error) {
	return 0, nil
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

// SummaryBarComponent returns a summary bar given data and the string representations of each data category.
// For example, data[0] will be represented by representations[0] in the summary bar.
// If len(representations) < len(data), the default representation "□" is used for all data category with missing representation.
type SummaryBarComponent struct {
	Length              int      // Length of the summary bar.
	Data                []int    // Data to draw using the summary bar. The order matters, e.g. the value in position 0 will be the leftmost portion of the bar.
	Representations     []string // Representations of each data value. Must be at least as long as `Data`. For example, Data[0] will be represented by Representations[0] in the summary bar.
	EmptyRepresentation string   // Representation to use for an empty bar.
}

// NewSummaryBarComponent returns a SummaryBarComponent.
func NewSummaryBarComponent(length int, data []int, representations []string, emptyRepresentation string) (*SummaryBarComponent, error) {
	if length <= 0 {
		return nil, fmt.Errorf("invalid length %d for summary bar", length)
	}

	if len(representations) < len(data) {
		return nil, fmt.Errorf("not enough representations: %s for %s",
			english.Plural(len(representations), "representation", "representations"),
			english.Plural(len(data), "data value", "data values"),
		)
	}

	if hasNegativeValue(data) {
		return nil, fmt.Errorf("input data contains negative values")
	}
	return &SummaryBarComponent{
		Length:              length,
		Data:                data,
		Representations:     representations,
		EmptyRepresentation: emptyRepresentation,
	}, nil
}

// Render writes the summary bar to out， without a new line.
func (c *SummaryBarComponent) Render(out io.Writer) (numLines int, err error) {
	buf := new(bytes.Buffer)
	portions, err := c.calculatePortions()
	if err != nil {
		if !errors.Is(err, &errTotalIsZero{}) {
			return 0, err
		}
		if _, err := buf.WriteString(fmt.Sprint(strings.Repeat(c.EmptyRepresentation, c.Length))); err != nil {
			return 0, fmt.Errorf("write empty bar to buffer: %w", err)
		}
		if _, err := buf.WriteTo(out); err != nil {
			return 0, fmt.Errorf("write buffer to out: %w", err)
		}
		return 0, nil
	}

	var bar string
	for idx, p := range portions {
		bar += fmt.Sprint(strings.Repeat(c.Representations[idx], p))
	}
	if _, err := buf.WriteString(bar); err != nil {
		return 0, fmt.Errorf("write bar to buffer: %w", err)
	}
	if _, err := buf.WriteTo(out); err != nil {
		return 0, fmt.Errorf("write buffer to out: %w", err)
	}
	return 0, nil
}

func (c *SummaryBarComponent) calculatePortions() ([]int, error) {
	type estimation struct {
		index   int
		dec     float64
		portion int
	}

	var sum int
	for _, v := range c.Data {
		sum += v
	}
	if sum <= 0 {
		return nil, &errTotalIsZero{}
	}

	// We first underestimate how many units each data value would take in the summary bar of length Length.
	// Then we distribute the rest of the units to each estimation.
	var underestimations []estimation
	for idx, v := range c.Data {
		rawFraction := (float64)(v) / (float64)(sum) * (float64)(c.Length)
		_, decPart := math.Modf(rawFraction)

		underestimations = append(underestimations, estimation{
			dec:     decPart,
			portion: (int)(math.Max(math.Floor(rawFraction), 0)),
			index:   idx,
		})
	}

	// Calculate the sum of the underestimated units and see how far we are from filling the bar of length `Length`.
	var currLength int
	for _, underestimated := range underestimations {
		currLength += underestimated.portion
	}
	unitsLeft := c.Length - currLength

	// Sort by decimal places from larger to smaller.
	sort.SliceStable(underestimations, func(i, j int) bool {
		return underestimations[i].dec > underestimations[j].dec
	})

	// Distribute extra values first to portions with larger decimal places.
	out := make([]int, len(c.Data))
	for _, d := range underestimations {
		if unitsLeft > 0 {
			d.portion += 1
			unitsLeft -= 1
		}
		out[d.index] = d.portion
	}

	return out, nil
}

type errTotalIsZero struct{}

func (e *errTotalIsZero) Error() string {
	return "The data sums up to zero"
}

func hasNegativeValue(data []int) bool {
	for _, d := range data {
		if d < 0 {
			return true
		}
	}
	return false
}

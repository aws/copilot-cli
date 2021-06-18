// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package summarybar provides renderers for summary bar.
package summarybar

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/term/progress"
)

var errTotalIsZero = errors.New("the data sums up to zero")

// summaryBarComponent returns a summary bar given data and the string representations of each data category.
type summaryBarComponent struct {
	data     []Datum
	width    int
	emptyRep string
}

// Datum is the basic unit of summary bar.
// Each Datum is composed of a value and a representation. The value will be represented with the representation in the
// rendered summary bar.
type Datum struct {
	Representation string
	Value          int
}

// Opt configures an option for summaryBarComponent.
type Opt func(*summaryBarComponent)

// WithWidth is an opt that configures the width for summaryBarComponent.
func WithWidth(width int) Opt {
	return func(c *summaryBarComponent) {
		c.width = width
	}
}

// WithEmptyRep is an opt that configures the empty representation for summaryBarComponent.
func WithEmptyRep(representation string) Opt {
	return func(c *summaryBarComponent) {
		c.emptyRep = representation
	}
}

// New returns a summaryBarComponent configured against opts.
func New(data []Datum, opts ...Opt) progress.Renderer {
	component := &summaryBarComponent{
		data: data,
	}
	for _, opt := range opts {
		opt(component)
	}
	return component
}

// Render writes the summary bar to ouT without a new line.
func (c *summaryBarComponent) Render(out io.Writer) (numLines int, err error) {
	if c.width <= 0 {
		return 0, fmt.Errorf("invalid width %d for summary bar", c.width)
	}

	if hasNegativeValue(c.data) {
		return 0, fmt.Errorf("input data contains negative values")
	}

	var data []int
	var representations []string
	for _, d := range c.data {
		data = append(data, d.Value)
		representations = append(representations, d.Representation)
	}

	buf := new(bytes.Buffer)
	portions, err := c.calculatePortions(data)
	if err != nil {
		if !errors.Is(err, errTotalIsZero) {
			return 0, err
		}
		if _, err := buf.WriteString(fmt.Sprint(strings.Repeat(c.emptyRep, c.width))); err != nil {
			return 0, fmt.Errorf("write empty bar to buffer: %w", err)
		}
		if _, err := buf.WriteTo(out); err != nil {
			return 0, fmt.Errorf("write buffer to out: %w", err)
		}
		return 0, nil
	}

	var bar string
	for idx, p := range portions {
		bar += fmt.Sprint(strings.Repeat(representations[idx], p))
	}
	if _, err := buf.WriteString(bar); err != nil {
		return 0, fmt.Errorf("write bar to buffer: %w", err)
	}
	if _, err := buf.WriteTo(out); err != nil {
		return 0, fmt.Errorf("write buffer to out: %w", err)
	}
	return 0, nil
}

func (c *summaryBarComponent) calculatePortions(data []int) ([]int, error) {
	type estimation struct {
		index   int
		dec     float64
		portion int
	}

	var sum int
	for _, v := range data {
		sum += v
	}
	if sum <= 0 {
		return nil, errTotalIsZero
	}

	// We first underestimate how many units each data value would take in the summary bar of length Length.
	// Then we distribute the rest of the units to each estimation.
	var underestimations []estimation
	for idx, v := range data {
		rawFraction := (float64)(v) / (float64)(sum) * (float64)(c.width)
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
	unitsLeft := c.width - currLength

	// Sort by decimal places from larger to smaller.
	sort.SliceStable(underestimations, func(i, j int) bool {
		return underestimations[i].dec > underestimations[j].dec
	})

	// Distribute extra values first to portions with larger decimal places.
	out := make([]int, len(data))
	for _, d := range underestimations {
		if unitsLeft > 0 {
			d.portion += 1
			unitsLeft -= 1
		}
		out[d.index] = d.portion
	}

	return out, nil
}

func hasNegativeValue(data []Datum) bool {
	for _, d := range data {
		if d.Value < 0 {
			return true
		}
	}
	return false
}

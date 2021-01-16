// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"fmt"
	"io"
	"strings"
)

const (
	nestedComponentPadding = 2 // Leading space characters for rendering a nested component.
)

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

// Done delegates to Root's Done.
func (c *dynamicTreeComponent) Done() <-chan struct{} {
	return c.Root.Done()
}

func renderComponents(out io.Writer, components []Renderer) (numLines int, err error) {
	for _, comp := range components {
		nl, err := comp.Render(out)
		if err != nil {
			return 0, err
		}
		numLines += nl
	}
	return numLines, nil
}

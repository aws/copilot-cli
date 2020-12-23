// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package progress

import (
	"bytes"
	"io"
)

// Renderer is the interface to print a component to a writer.
// It returns the number of lines printed and the error if any.
type Renderer interface {
	Render(out io.Writer) (numLines int, err error)
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

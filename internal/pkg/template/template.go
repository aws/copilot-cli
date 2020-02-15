// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package template renders the static files under the "/templates/" directory.
package template

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/aws/amazon-ecs-cli-v2/templates"
	"github.com/gobuffalo/packd"
)

var box = templates.Box()

// Parser is the interface that wraps the Parse method.
type Parser interface {
	Parse(path string, data interface{}, options ...ParseOption) (*Content, error)
}

// ReadParser is the interface that wraps the Read and Parse methods.
type ReadParser interface {
	Read(path string) (*Content, error)
	Parser
}

// Template represents the "/templates/" directory that holds static files to be embedded in the binary.
type Template struct {
	box packd.Box
}

// Content represents the parsed template.
type Content struct {
	*bytes.Buffer
}

// ParseOption represents a functional option for the Parse method.
type ParseOption func(t *template.Template) *template.Template

// New returns a Template object that can be used to parse files under the "/templates/" directory.
func New() *Template {
	return &Template{
		box: box,
	}
}

// Read returns the contents of the template under "/templates/{path}".
func (t *Template) Read(path string) (*Content, error) {
	s, err := t.read(path)
	if err != nil {
		return nil, err
	}
	return &Content{
		Buffer: bytes.NewBufferString(s),
	}, nil
}

// Parse parses the template under "/templates/{path}" with the specified data object and returns its content.
func (t *Template) Parse(path string, data interface{}, options ...ParseOption) (*Content, error) {
	content, err := t.read(path)
	if err != nil {
		return nil, err
	}

	tpl := template.New("template")
	for _, opt := range options {
		tpl = opt(tpl)
	}
	parsedTpl, err := tpl.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", path, err)
	}

	buf := &bytes.Buffer{}
	if err := parsedTpl.Execute(buf, data); err != nil {
		return nil, fmt.Errorf("execute template %s with data %v: %w", path, data, err)
	}
	return &Content{buf}, nil
}

// WithFuncs returns a template that can parse additional custom functions.
func WithFuncs(fns map[string]interface{}) ParseOption {
	return func(t *template.Template) *template.Template {
		return t.Funcs(fns)
	}
}

// MarshalBinary returns the contents as binary and implements the encoding.BinaryMarshaler interface.
func (c *Content) MarshalBinary() ([]byte, error) {
	return c.Bytes(), nil
}

func (t *Template) read(path string) (string, error) {
	s, err := t.box.FindString(path)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", path, err)
	}
	return s, nil
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package template renders the static files under the "/templates/" directory.
package template

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"path"
	"strings"
	"text/template"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
	"github.com/aws/copilot-cli/templates"
	"github.com/gobuffalo/packd"
)

const (
	customResourceRootPath         = "custom-resources"
	customResourceZippedScriptName = "index.js"
	scriptDirName                  = "scripts"
)

var box = templates.Box()

var envCustomResourceFiles = []string{
	"dns-cert-validator",
	"dns-delegation",
	"enable-long-arns",
}

// Parser is the interface that wraps the Parse method.
type Parser interface {
	Parse(path string, data interface{}, options ...ParseOption) (*Content, error)
}

// ReadParser is the interface that wraps the Read and Parse methods.
type ReadParser interface {
	Read(path string) (*Content, error)
	Parser
}

// Uploadable is an uploadable file.
type Uploadable struct {
	name    string
	content []byte
	path    string
}

// Name returns the name of the custom resource script.
func (e Uploadable) Name() string {
	return e.name
}

// Content returns the content of the custom resource script.
func (e Uploadable) Content() []byte {
	return e.content
}

type compressedFile struct {
	name        string
	uploadables []Uploadable
}

// Template represents the "/templates/" directory that holds static files to be embedded in the binary.
type Template struct {
	box packd.Box
}

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
	tpl, err := t.parse("template", path, options...)
	if err != nil {
		return nil, err
	}
	buf := new(bytes.Buffer)
	if err := tpl.Execute(buf, data); err != nil {
		return nil, fmt.Errorf("execute template %s with data %v: %w", path, data, err)
	}
	return &Content{buf}, nil
}

// UploadEnvironmentCustomResources uploads the environment custom resource scripts.
func (t *Template) UploadEnvironmentCustomResources(upload s3.UploadFunc) ([]string, error) {
	return t.uploadCustomResources(upload, envCustomResourceFiles)
}

func (t *Template) uploadCustomResources(upload s3.UploadFunc, fileNames []string) ([]string, error) {
	var urls []string
	for _, name := range fileNames {
		url, err := t.uploadCompressedFile(upload, compressedFile{
			name: path.Join(scriptDirName, name),
			uploadables: []Uploadable{
				{
					name: customResourceZippedScriptName,
					path: path.Join(customResourceRootPath, fmt.Sprintf("%s.js", name)),
				},
			},
		})
		if err != nil {
			return nil, err
		}
		urls = append(urls, url)
	}
	return urls, nil
}

func (t *Template) uploadCompressedFile(upload s3.UploadFunc, file compressedFile) (string, error) {
	var contents []byte
	for _, uploadable := range file.uploadables {
		content, err := t.Read(uploadable.path)
		if err != nil {
			return "", err
		}
		uploadable.content = content.Bytes()
		contents = append(contents, uploadable.content...)
	}
	// Golang limit. See https://stackoverflow.com/questions/12990338/cannot-convert-string-to-interface/12990540#12990540
	nameBinaries := make([]s3.NamedBinary, len(file.uploadables))
	for idx, file := range file.uploadables {
		nameBinaries[idx] = s3.NamedBinary(file)
	}
	// Suffix with a SHA256 checksum of the compressedFile so that
	// only new content gets a new URL. Otherwise, if two compressedFiles have the
	// same content then the URL generated will be identical.
	url, err := upload(fmt.Sprintf("%s/%x", file.name, sha256.Sum256(contents)), nameBinaries...)
	if err != nil {
		return "", fmt.Errorf("upload %s: %w", file.name, err)
	}
	return url, nil
}

// ParseOption represents a functional option for the Parse method.
type ParseOption func(t *template.Template) *template.Template

// WithFuncs returns a template that can parse additional custom functions.
func WithFuncs(fns map[string]interface{}) ParseOption {
	return func(t *template.Template) *template.Template {
		return t.Funcs(fns)
	}
}

// Content represents the parsed template.
type Content struct {
	*bytes.Buffer
}

// MarshalBinary returns the contents as binary and implements the encoding.BinaryMarshaler interface.
func (c *Content) MarshalBinary() ([]byte, error) {
	return c.Bytes(), nil
}

// newTextTemplate returns a named text/template with the "indent" and "include" functions.
func newTextTemplate(name string) *template.Template {
	t := template.New(name)
	t.Funcs(map[string]interface{}{
		"include": func(name string, data interface{}) (string, error) {
			// Taken from https://github.com/helm/helm/blob/8648ccf5d35d682dcd5f7a9c2082f0aaf071e817/pkg/engine/engine.go#L147-L154
			buf := bytes.NewBuffer(nil)
			if err := t.ExecuteTemplate(buf, name, data); err != nil {
				return "", err
			}
			return buf.String(), nil
		},
		"indent": func(spaces int, s string) string {
			// Taken from https://github.com/Masterminds/sprig/blob/48e6b77026913419ba1a4694dde186dc9c4ad74d/strings.go#L109-L112
			pad := strings.Repeat(" ", spaces)
			return pad + strings.Replace(s, "\n", "\n"+pad, -1)
		},
	})
	return t
}

func (t *Template) read(path string) (string, error) {
	s, err := t.box.FindString(path)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", path, err)
	}
	return s, nil
}

// parse reads the file at path and returns a parsed text/template object with the given name.
func (t *Template) parse(name, path string, options ...ParseOption) (*template.Template, error) {
	content, err := t.read(path)
	if err != nil {
		return nil, err
	}

	emptyTextTpl := newTextTemplate(name)
	for _, opt := range options {
		emptyTextTpl = opt(emptyTextTpl)
	}
	parsedTpl, err := emptyTextTpl.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", path, err)
	}
	return parsedTpl, nil
}

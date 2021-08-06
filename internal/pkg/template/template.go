// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package template renders the static files under the "/templates/" directory.
package template

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/aws/copilot-cli/internal/pkg/aws/s3"
)

//go:embed templates
var templateFS embed.FS

// File names under "templates/".
const (
	DNSCertValidatorFileName            = "dns-cert-validator"
	DNSDelegationFileName               = "dns-delegation"
	EnableLongARNsFileName              = "enable-long-arns"
	CustomDomainFileName                = "custom-domain"
	AppRunnerCustomDomainLambdaFileName = "custom-domain-app-runner"
	AWSSDKLayerFileName                 = "aws-sdk-layer"

	customResourceRootPath         = "custom-resources"
	customResourceZippedScriptName = "index.js"
	scriptDirName                  = "scripts"
	layerDirName                   = "layers"
)

// Groups of files that belong to the same stack.
var (
	envCustomResourceFiles = []string{
		DNSCertValidatorFileName,
		DNSDelegationFileName,
		EnableLongARNsFileName,
		CustomDomainFileName,
	}
	rdWkldCustomResourceFiles = []string{
		AppRunnerCustomDomainLambdaFileName,
	}
	rdWkldCustomResourceLayers = []string{
		AWSSDKLayerFileName,
	}
)

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

type fileToCompress struct {
	name        string
	uploadables []Uploadable
}

// Template represents the "/templates/" directory that holds static files to be embedded in the binary.
type Template struct {
	fs fs.ReadFileFS
}

// New returns a Template object that can be used to parse files under the "/templates/" directory.
func New() *Template {
	return &Template{
		fs: templateFS,
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
func (t *Template) UploadEnvironmentCustomResources(upload s3.CompressAndUploadFunc) (map[string]string, error) {
	return t.uploadCustomResources(upload, envCustomResourceFiles)
}

//UploadRequestDrivenWebServiceCustomResources uploads the request driven web service custom resource scripts.
func (t *Template) UploadRequestDrivenWebServiceCustomResources(upload s3.CompressAndUploadFunc) (map[string]string, error) {
	return t.uploadCustomResources(upload, rdWkldCustomResourceFiles)
}

// UploadRequestDrivenWebServiceLayers uploads already-zipped layers for a request driven web service.
func (t *Template) UploadRequestDrivenWebServiceLayers(upload s3.UploadFunc) (map[string]string, error) {
	urls := make(map[string]string)
	for _, layerName := range rdWkldCustomResourceLayers {
		content, err := t.Read(path.Join(customResourceRootPath, fmt.Sprintf("%s.zip", layerName)))
		if err != nil {
			return nil, err
		}
		name := path.Join(layerDirName, layerName)
		url, err := upload(fmt.Sprintf("%s/%x", name, sha256.Sum256(content.Bytes())), Uploadable{
			name:    layerName,
			content: content.Bytes(),
		})
		if err != nil {
			return nil, fmt.Errorf("upload %s: %w", name, err)
		}

		urls[layerName] = url
	}

	return urls, nil
}

func (t *Template) uploadCustomResources(upload s3.CompressAndUploadFunc, fileNames []string) (map[string]string, error) {
	urls := make(map[string]string)
	for _, name := range fileNames {
		url, err := t.uploadFileToCompress(upload, fileToCompress{
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
		urls[name] = url
	}
	return urls, nil
}

func (t *Template) uploadFileToCompress(upload s3.CompressAndUploadFunc, file fileToCompress) (string, error) {
	var contents []byte
	var nameBinaries []s3.NamedBinary
	for _, uploadable := range file.uploadables {
		content, err := t.Read(uploadable.path)
		if err != nil {
			return "", err
		}
		uploadable.content = content.Bytes()
		contents = append(contents, uploadable.content...)
		nameBinaries = append(nameBinaries, uploadable)
	}
	// Suffix with a SHA256 checksum of the fileToCompress so that
	// only new content gets a new URL. Otherwise, if two fileToCompresss have the
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
	dat, err := t.fs.ReadFile(filepath.Join("templates", path))
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", path, err)
	}
	return string(dat), nil
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

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package manifest provides functionality to render a manifest file and transform it to a CloudFormation template.
package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"
	"path"
	"strings"

	"github.com/gobuffalo/packr/v2"
)

// TemplateNames is the list of valid infrastructure-as-code template names.
var TemplateNames = []string{
	"Load Balanced Web App",
	"Empty",
}

// ErrInvalidApp occurs if the application to render does not exist.
var ErrInvalidApp = errors.New("app cannot be nil")

// ErrInvalidTemplate occurs when a user requested a manifest template name that doesn't exist.
type ErrInvalidTemplate struct {
	tpl string
}

func (e *ErrInvalidTemplate) Error() string {
	return fmt.Sprintf("invalid manifest template: %s, must be one of: %s",
		e.tpl,
		strings.Join(TemplateNames, ", "))
}

// Manifest is a infrastructure-as-code template to represent applications.
type Manifest struct {
	tpl string // name of the template, must be one of TemplateNames.

	wc io.WriteCloser // interface to write the Manifest file.
}

// New creates a new Manifest given a template name.
//
// If the template name doesn't exist, it returns an ErrInvalidTemplate.
func New(tpl string) (*Manifest, error) {
	for _, name := range TemplateNames {
		if tpl == name {
			return &Manifest{
				tpl: tpl,
			}, nil
		}
	}

	return nil, &ErrInvalidTemplate{tpl: tpl}
}

// Render evaluates the manifest's template with the data and then writes it to a file under ./ecs/{filePrefix}-app.yaml.
func (m *Manifest) Render(filePrefix string, data interface{}) error {
	if err := m.setWriter(filePrefix); err != nil {
		return err
	}

	defer m.wc.Close()
	box := packr.New("templates", "./template")
	manifest, err := box.FindString("manifest/" + m.templateFile())
	if err != nil {
		return err
	}
	tpl, err := template.New(m.tpl).Parse(manifest)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return err
	}
	if _, err := m.wc.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}

// CFNTemplate returns the CloudFormation template from the manifest file.
func (m *Manifest) CFNTemplate() string {
	return ""
}

func (m *Manifest) setWriter(name string) error {
	if m.wc != nil {
		return nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	projectDir := path.Join(wd, "ecs")
	if err := os.MkdirAll(projectDir, os.ModePerm); err != nil {
		return err
	}
	f, err := os.Create(path.Join(projectDir, fmt.Sprintf("%s-app.yaml", name)))
	if err != nil {
		return err
	}
	m.wc = f
	return nil
}

// templateFile returns the name of the file matching the template, if the name is not found returns an empty string.
func (m *Manifest) templateFile() string {
	templateFileNames := []string{
		"load-balanced-fargate-service.yml",
		"empty.yml",
	}
	for i, name := range TemplateNames {
		if m.tpl == name {
			return templateFileNames[i]
		}
	}
	return ""
}

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"
	"path"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/gobuffalo/packr/v2"
)

// Errors that can occur while initializing an application.
var (
	// ErrInvalidManifestType occurs when a user enters a manifest type that doesn't have a matching template.
	ErrInvalidManifestType = fmt.Errorf("invalid manifest type, must be one of: %s", strings.Join(manifestTypes, ","))
)

var (
	manifestTypes = []string{
		"Load Balanced Web App",
		"Empty",
	}
	manifestFileNames = []string{
		"load-balanced-fargate-service.yml",
		"empty.yml",
	}
)

// InitOpts holds additional fields needed to initialize an application.
type InitOpts struct {
	ManifestType string         // must be one of ManifestTypes.
	wc           io.WriteCloser // interface to write the Manifest file.
}

// Validate returns nil if the flags set by the user have valid values. Otherwise returns an error.
func (opts *InitOpts) Validate() error {
	if opts.ManifestType != "" {
		for _, t := range manifestTypes {
			if t == opts.ManifestType {
				return nil
			}
		}
		return ErrInvalidManifestType
	}
	return nil
}

func (opts *InitOpts) askManifestType(prompt terminal.Stdio) error {
	if opts.ManifestType != "" {
		// A validated manifest type is already set.
		return nil
	}
	return survey.AskOne(&survey.Select{
		Message: "What type of application is this?",
		Help:    "Pre-defined infrastructure templates.",
		Options: manifestTypes,
		Default: manifestTypes[0],
	}, &opts.ManifestType, survey.WithStdio(prompt.In, prompt.Out, prompt.Err))
}

// setManifestWriter creates a Manifest file under ./ecs/ if there is no existing Writer.
func (opts *InitOpts) setManifestWriter(appName string) error {
	if opts.wc != nil {
		// A manifest writer is already set.
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
	f, err := os.Create(path.Join(projectDir, fmt.Sprintf("%s-app.yaml", appName)))
	if err != nil {
		return err
	}
	opts.wc = f
	return nil
}

func (opts *InitOpts) manifestFileName() string {
	for i, t := range manifestTypes {
		if opts.ManifestType == t {
			return manifestFileNames[i]
		}
	}
	// This should never happen, see TestManifestTypeFileNamePairs
	return ""
}

// Init creates a new application.
//
// It assumes that the opts with non-zero values have already been validated.
// It prompts the user for any missing options fields, and then writes the manifest file to ./ecs/.
func (a *App) Init(opts *InitOpts) error {
	if err := opts.askManifestType(a.prompt); err != nil {
		return err
	}
	if err := opts.setManifestWriter(a.Name); err != nil {
		return err
	}

	defer opts.wc.Close()
	box := packr.New("templates", "./template")
	manifest, err := box.FindString("manifest/" + opts.manifestFileName())
	if err != nil {
		return err
	}
	tpl, err := template.New(opts.ManifestType).Parse(manifest)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, a); err != nil {
		return err
	}
	if _, err := opts.wc.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}

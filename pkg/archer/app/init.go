// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer/manifest"
)

type renderer interface {
	Render(filePrefix string, data interface{}) error
}

// InitOpts holds additional fields needed to initialize an application.
type InitOpts struct {
	ManifestTemplate string // name of the Manifest template.

	m renderer // interface for Manifest operations.
}

func (opts *InitOpts) askManifestTemplate(prompt terminal.Stdio) error {
	if opts.ManifestTemplate != "" {
		// User already set a manifest template name.
		return nil
	}
	return survey.AskOne(&survey.Select{
		Message: "Which template would you like to use?",
		Help:    "Pre-defined infrastructure templates.",
		Options: manifest.TemplateNames,
		Default: manifest.TemplateNames[0],
	}, &opts.ManifestTemplate, survey.WithStdio(prompt.In, prompt.Out, prompt.Err))
}

func (opts *InitOpts) renderManifest(a *App) error {
	if opts.m == nil {
		m, err := manifest.New(opts.ManifestTemplate)
		if err != nil {
			return err
		}
		opts.m = m
	}
	return opts.m.Render(a.Name, a)
}

// Init creates a new application.
//
// It prompts the user for any missing options fields, and then writes the manifest file to ./ecs/.
func (a *App) Init(opts *InitOpts) error {
	if err := opts.askManifestTemplate(a.prompt); err != nil {
		return err
	}
	return opts.renderManifest(a)
}

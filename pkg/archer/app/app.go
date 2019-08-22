// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package app provides functionality to handle the life cycle of an application.
package app

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
)

// App represents an ECS Service or Task and any related AWS Infrastructure.
type App struct {
	Project string `survey:"project"` // namespace that this application belongs to.
	Name    string `survey:"name"`    // unique identifier to logically group AWS resources together.

	// prompt holds the interfaces to receive and output app configuration data to the terminal.
	prompt terminal.Stdio
}

// New creates a new application that prompts the user on Stderr and receives their input on Stdin.
func New() *App {
	return &App{
		prompt: terminal.Stdio{
			In:  os.Stdin,
			Out: os.Stderr,
			Err: os.Stderr,
		},
	}
}

// Ask prompts the user for the value of any required fields that are not already provided.
func (a *App) Ask() error {
	var qs []*survey.Question
	if a.Project == "" {
		qs = append(qs, &survey.Question{
			Name: "project",
			Prompt: &survey.Input{
				Message: "What is your project's name?",
				Help:    "Applications under the same project share the same VPC and ECS Cluster and are discoverable via service discovery.",
			},
			Validate: survey.Required,
		})
	}
	if a.Name == "" {
		qs = append(qs, &survey.Question{
			Name: "name",
			Prompt: &survey.Input{
				Message: "What is your application's name?",
				Help:    "Collection of AWS services to achieve a business capability. Must be unique within a project.",
			},
			Validate: survey.Required,
		})
	}
	return survey.Ask(qs, a, survey.WithStdio(a.prompt.In, a.prompt.Out, a.prompt.Err))
}

// String returns a human readable representation of an App.
func (a *App) String() string {
	return fmt.Sprintf("name=%s, project=%s", a.Name, a.Project)
}

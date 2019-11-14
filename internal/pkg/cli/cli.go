// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cli contains the archer subcommands.
package cli

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/prompt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/spf13/viper"
)

func init() {
	// Store the local project's name in viper.
	bindProjectName()
}

// GlobalOpts holds fields that are used across multiple commands.
type GlobalOpts struct {
	projectName string
	prompt      prompter
}

// NewGlobalOpts returns a GlobalOpts with the project name retrieved from viper.
func NewGlobalOpts() *GlobalOpts {
	return &GlobalOpts{
		projectName: viper.GetString(projectFlag),
		prompt:      prompt.New(),
	}
}

// ProjectName returns the project name.
// If the name is empty, it caches it after querying viper.
func (o *GlobalOpts) ProjectName() string {
	if o.projectName != "" {
		return o.projectName
	}
	o.projectName = viper.GetString(projectFlag)
	return o.projectName
}

// actionCommand is the interface that every command that creates a resource implements.
type actionCommand interface {
	Ask() error
	Validate() error
	Execute() error
	RecommendedActions() []string
}

// bindProjectName loads the project's name to viper.
// If there is an error, we swallow the error and leave the default value as empty string.
func bindProjectName() {
	name, err := loadProjectName()
	if err != nil {
		return
	}
	viper.SetDefault(projectFlag, name)
}

// loadProjectName retrieves the project's name from the workspace if it exists and returns it.
// If there is an error, it returns an empty string and the error.
func loadProjectName() (string, error) {
	// Load the workspace and set the project flag.
	ws, err := workspace.New()
	if err != nil {
		// If there's an error fetching the workspace, fall back to requiring
		// the project flag be set.
		return "", fmt.Errorf("fetching workspace: %w", err)
	}

	summary, err := ws.Summary()
	if err != nil {
		// If there's an error reading from the workspace, fall back to requiring
		// the project flag be set.
		return "", fmt.Errorf("reading from workspace: %w", err)
	}
	return summary.ProjectName, nil
}

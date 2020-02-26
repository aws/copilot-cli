// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addons contains the service to manage addons.
package addons

import (
	"fmt"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
)

const (
	addonsTemplatePath = "addons/cf.yml"
)

type workspaceService interface {
	ReadAddonFiles(appName string) (*workspace.AddonFiles, error)
}

// Addons represents additional resources for an application.
type Addons struct {
	appName string

	parser template.Parser
	ws     workspaceService
}

// New creates an Addons object given an application name.
func New(appName string) (*Addons, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}
	return &Addons{
		appName: appName,
		parser:  template.New(),
		ws:      ws,
	}, nil
}

// Template merges the files under the "addons/" directory of an application
// into a single CloudFormation template and returns it.
func (a *Addons) Template() (string, error) {
	out, err := a.ws.ReadAddonFiles(a.appName)
	if err != nil {
		return "", err
	}
	content, err := a.parser.Parse(addonsTemplatePath, struct {
		AppName      string
		AddonContent *workspace.AddonFiles
	}{
		AppName:      a.appName,
		AddonContent: out,
	})
	if err != nil {
		return "", err
	}
	return content.String(), nil
}

// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addons contains the service to manage addons.
package addons

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/aws/amazon-ecs-cli-v2/templates"
	"github.com/gobuffalo/packd"
)

const (
	addonsTemplatePath = "addons/cf.yml"
)

type workspaceService interface {
	ReadAddonFiles(appName string) (*workspace.ReadAddonFilesOutput, error)
}

// Addons represent additional resources for an application.
type Addons struct {
	appName string

	box packd.Box
	ws  workspaceService
}

// New creates an Addons struct given an application name.
func New(appName string) (*Addons, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}
	return &Addons{
		appName: appName,
		box:     templates.Box(),
		ws:      ws,
	}, nil
}

// Template parses params.yml, policy.yml, {resource}.yml, and outputs.yml to generate
// the addons CloudFormation template.
func (a *Addons) Template() (string, error) {
	addonContent, err := a.ws.ReadAddonFiles(a.appName)
	if err != nil {
		return "", err
	}
	content, err := a.box.FindString(addonsTemplatePath)
	if err != nil {
		return "", fmt.Errorf("failed to find the cloudformation template at %s", addonsTemplatePath)
	}
	tpl, err := template.New("template").Parse(content)
	if err != nil {
		return "", fmt.Errorf("parse CloudFormation template for %s: %w", a.appName, err)
	}
	var buf bytes.Buffer
	templateData := struct {
		AppName      string
		AddonContent *workspace.ReadAddonFilesOutput
	}{
		AppName:      a.appName,
		AddonContent: addonContent,
	}
	if err := tpl.Execute(&buf, templateData); err != nil {
		return "", fmt.Errorf("execute CloudFormation template for %s: %w", a.appName, err)
	}
	return buf.String(), nil
}

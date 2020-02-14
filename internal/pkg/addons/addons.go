// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addons contains the service to manage addons.
package addons

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
)

const (
	paramsFileName  = "params"
	outputsFileName = "outputs"
	indent          = "  " // two spaces
)

type workspaceService interface {
	ReadAddonsFile(appName, fileName string) ([]byte, error)
	ListAddonsFiles(appName string) ([]string, error)
}

// Addons represent additional resources for an application.
type Addons struct {
	appName string

	ws workspaceService
}

// New creates an Addons struct given an application name.
func New(appName string) (*Addons, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}
	return &Addons{
		appName,
		ws,
	}, nil
}

// Template concatenates params.yml, policy.yml, {resource}.yml, and outputs.yml to generate
// the CloudFormation template.
func (a *Addons) Template() (string, error) {
	addonFiles, err := a.ws.ListAddonsFiles(a.appName)
	if err != nil {
		return "", fmt.Errorf("list addon files: %w", err)
	}
	var resources, params, outputs string
	for _, fileName := range addonFiles {
		content, err := a.ws.ReadAddonsFile(a.appName, fileName)
		if err != nil {
			return "", fmt.Errorf("read addon file %s: %w", fileName, err)
		}
		switch name := strings.TrimSuffix(fileName, filepath.Ext(fileName)); name {
		case paramsFileName:
			params += fmt.Sprintf("Parameters:%s", toYAML(content))
		case outputsFileName:
			outputs += fmt.Sprintf("Outputs:%s", toYAML(content))
		default:
			if resources == "" {
				resources = "Resources:"
			}
			resources += toYAML(content)
		}
	}
	return fmt.Sprintf("%s%s%s", params, resources, outputs), nil
}

func toYAML(b []byte) string {
	strYAML := ""
	for _, line := range strings.Split(string(b), "\n") {
		strYAML += fmt.Sprintf("%s%s\n", indent, line)
	}
	return strYAML
}

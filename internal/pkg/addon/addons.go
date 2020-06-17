// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addon contains the service to manage addons.
package addon

import (
	"fmt"
	"path/filepath"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"gopkg.in/yaml.v3"
)

const (
	// StackName is the name of the addons nested stack resource.
	StackName = "AddonsStack"
)

type workspaceReader interface {
	ReadAddonsDir(svcName string) ([]string, error)
	ReadAddon(svcName, fileName string) ([]byte, error)
}

// Addons represents additional resources for a service.
type Addons struct {
	svcName string

	parser template.Parser
	ws     workspaceReader
}

// New creates an Addons object given an service name.
func New(svcName string) (*Addons, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}
	return &Addons{
		svcName: svcName,
		parser:  template.New(),
		ws:      ws,
	}, nil
}

// Template merges CloudFormation templates under the "addons/" directory of a service
// into a single CloudFormation template and returns it.
//
// If the addons directory doesn't exist, it returns the empty string and ErrDirNotExist.
func (a *Addons) Template() (string, error) {
	fnames, err := a.ws.ReadAddonsDir(a.svcName)
	if err != nil {
		return "", &ErrDirNotExist{
			SvcName:   a.svcName,
			ParentErr: err,
		}
	}

	mergedTemplate := newCFNTemplate("merged")
	for _, fname := range filterYAMLfiles(fnames) {
		out, err := a.ws.ReadAddon(a.svcName, fname)
		if err != nil {
			return "", fmt.Errorf("read addon %s under service %s: %w", fname, a.svcName, err)
		}
		tpl := newCFNTemplate(fname)
		if err := yaml.Unmarshal(out, tpl); err != nil {
			return "", fmt.Errorf("unmarshal addon %s under service %s: %w", fname, a.svcName, err)
		}
		if err := mergedTemplate.merge(tpl); err != nil {
			return "", err
		}
	}
	out, err := yaml.Marshal(mergedTemplate)
	if err != nil {
		return "", fmt.Errorf("marshal merged addons template: %w", err)
	}
	return string(out), nil
}

func filterYAMLfiles(files []string) []string {
	yamlExtensions := []string{".yaml", ".yml"}

	var yamlFiles []string
	for _, f := range files {
		if !contains(yamlExtensions, filepath.Ext(f)) {
			continue
		}
		yamlFiles = append(yamlFiles, f)
	}
	return yamlFiles
}

func contains(arr []string, el string) bool {
	for _, item := range arr {
		if item == el {
			return true
		}
	}
	return false
}

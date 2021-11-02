// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addon contains the service to manage addons.
package addon

import (
	"fmt"
	"path/filepath"

	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"gopkg.in/yaml.v3"
)

const (
	// StackName is the name of the addons nested stack resource.
	StackName = "AddonsStack"
)

var (
	yamlExtensions     = []string{".yaml", ".yml"}
	parameterFileNames = []string{"addons.parameters.yml", "addons.parameters.yaml"}
)

type workspaceReader interface {
	ReadAddonsDir(svcName string) ([]string, error)
	ReadAddon(svcName, fileName string) ([]byte, error)
}

// Addons represents additional resources for a workload.
type Addons struct {
	wlName string

	parser template.Parser
	ws     workspaceReader
}

// New creates an Addons object given a workload name.
func New(wlName string) (*Addons, error) {
	ws, err := workspace.New()
	if err != nil {
		return nil, fmt.Errorf("workspace cannot be created: %w", err)
	}
	return &Addons{
		wlName: wlName,
		parser: template.New(),
		ws:     ws,
	}, nil
}

// Template merges CloudFormation templates under the "addons/" directory of a workload
// into a single CloudFormation template and returns it.
//
// If the addons directory doesn't exist, it returns the empty string and
// ErrAddonsDirNotExist.
func (a *Addons) Template() (string, error) {
	fnames, err := a.ws.ReadAddonsDir(a.wlName)
	if err != nil {
		return "", &ErrAddonsNotFound{
			WlName:    a.wlName,
			ParentErr: err,
		}
	}

	yamlFiles := filterFiles(fnames, yamlMatcher, nonParamsMatcher)
	if len(yamlFiles) == 0 {
		return "", &ErrAddonsNotFound{
			WlName: a.wlName,
		}
	}

	mergedTemplate := newCFNTemplate("merged")
	for _, fname := range yamlFiles {
		out, err := a.ws.ReadAddon(a.wlName, fname)
		if err != nil {
			return "", fmt.Errorf("read addon %s under %s: %w", fname, a.wlName, err)
		}
		tpl := newCFNTemplate(fname)
		if err := yaml.Unmarshal(out, tpl); err != nil {
			return "", fmt.Errorf("unmarshal addon %s under %s: %w", fname, a.wlName, err)
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

func filterFiles(files []string, matchers ...func(string) bool) []string {
	var matchedFiles []string
	for _, f := range files {
		matches := true
		for _, match := range matchers {
			if !match(f) {
				matches = false
				break
			}
		}
		if matches {
			matchedFiles = append(matchedFiles, f)
		}
	}
	return matchedFiles
}

func yamlMatcher(fileName string) bool {
	return contains(yamlExtensions, filepath.Ext(fileName))
}

func paramsMatcher(fileName string) bool {
	return contains(parameterFileNames, fileName)
}

func nonParamsMatcher(fileName string) bool {
	return !paramsMatcher(fileName)
}

func contains(arr []string, el string) bool {
	for _, item := range arr {
		if item == el {
			return true
		}
	}
	return false
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addon contains the service to manage addons.
package addon

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/template"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/dustin/go-humanize/english"
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

	templateFiles := filterFiles(fnames, yamlMatcher, nonParamsMatcher)
	if len(templateFiles) == 0 {
		return "", &ErrAddonsNotFound{
			WlName: a.wlName,
		}
	}

	mergedTemplate := newCFNTemplate("merged")
	for _, fname := range templateFiles {
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

// Parameters returns the content of user-defined additional CloudFormation Parameters
// to pass from the parent stack to Template.
//
// If there is no addons/ directory defined, then returns "" and ErrAddonsNotFound.
// If there are addons but no parameters file defined, then returns "" and nil for error.
// If there are multiple parameters files, then returns "" and cannot define multiple parameter files error.
// If the addons parameters use the reserved parameter names, then returns "" and a reserved parameter error.
func (a *Addons) Parameters() (string, error) {
	fnames, err := a.ws.ReadAddonsDir(a.wlName)
	if err != nil {
		return "", &ErrAddonsNotFound{
			WlName:    a.wlName,
			ParentErr: err,
		}
	}
	paramFiles := filterFiles(fnames, paramsMatcher)
	if len(paramFiles) == 0 {
		return "", nil
	}
	if len(paramFiles) > 1 {
		return "", fmt.Errorf("defining multiple %s is not allowed under %s addons", english.WordSeries(parameterFileNames, "and"), a.wlName)
	}
	raw, err := a.ws.ReadAddon(a.wlName, paramFiles[0])
	if err != nil {
		return "", fmt.Errorf("read parameter file %s under %s addons: %w", paramFiles[0], a.wlName, err)
	}
	content := struct {
		Parameters yaml.Node `yaml:"Parameters"`
	}{}
	if err := yaml.Unmarshal(raw, &content); err != nil {
		return "", fmt.Errorf("unmarshal 'Parameters' in file %s under %s addons: %w", paramFiles[0], a.wlName, err)
	}
	if content.Parameters.IsZero() {
		return "", fmt.Errorf("must define field 'Parameters' in file %s under %s addons", paramFiles[0], a.wlName)
	}
	if err := a.validateReservedParameters(content.Parameters, paramFiles[0]); err != nil {
		return "", err
	}
	buf := new(strings.Builder)
	encoder := yaml.NewEncoder(buf)
	encoder.SetIndent(2 /* 2 spaces to indent */)
	if err := encoder.Encode(content.Parameters); err != nil {
		return "", fmt.Errorf("marshal contents of 'Parameters' in file %s under %s addons", paramFiles[0], a.wlName)
	}
	return buf.String(), nil
}

func (a *Addons) validateReservedParameters(params yaml.Node, fname string) error {
	content := struct {
		App  yaml.Node `yaml:"App"`
		Env  yaml.Node `yaml:"Env"`
		Name yaml.Node `yaml:"Name"`
	}{}
	if err := params.Decode(&content); err != nil {
		return fmt.Errorf("decode content of parameters file %s under %s addons", fname, a.wlName)
	}

	for _, field := range []yaml.Node{content.App, content.Env, content.Name} {
		if !field.IsZero() {
			return fmt.Errorf("reserved parameters 'App', 'Env', and 'Name' cannot be declared in %s under %s addons", fname, a.wlName)
		}
	}
	return nil
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

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addon contains the service to manage addons.
package addon

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dustin/go-humanize/english"
	"gopkg.in/yaml.v3"
)

const (
	// StackName is the name of the addons nested stack resource.
	StackName = "AddonsStack"
)

var (
	yamlExtensions     = []string{".yaml", ".yml"}
	parameterFileNames = func() []string {
		const paramFilePrefix = "addons.parameters"
		var fnames []string
		for _, ext := range yamlExtensions {
			fnames = append(fnames, fmt.Sprintf("%s%s", paramFilePrefix, ext))
		}
		return fnames
	}()
)

type workspaceReader interface {
	ReadAddonsDir(svcName string) ([]string, error)
	ReadAddon(svcName, fileName string) ([]byte, error)
}

// Stack represents a CloudFormation stack.
type Stack struct {
	template     *cfnTemplate
	parameters   yaml.Node
	workloadName string
}

// Parse parses the 'addon/' directory for the given workload
// and returns a Stack created by merging the CloudFormation templates
// files found there. If no addons are found, Parse returns a nil
// Stack and ErrAddonsNotFound.
func Parse(workloadName string, ws workspaceReader) (*Stack, error) {
	fnames, err := ws.ReadAddonsDir(workloadName)
	if err != nil {
		return nil, &ErrAddonsNotFound{
			WlName:    workloadName,
			ParentErr: err,
		}
	}

	template, err := parseTemplate(fnames, workloadName, ws)
	if err != nil {
		return nil, err
	}

	params, err := parseParameters(fnames, workloadName, ws)
	if err != nil {
		return nil, err
	}

	return &Stack{
		template:     template,
		parameters:   params,
		workloadName: workloadName,
	}, nil
}

// Template returns Stack's CloudFormation template as a yaml string.
func (s *Stack) Template() (string, error) {
	if s.template == nil {
		return "", nil
	}

	return s.encode(s.template)
}

// Template returns Stack's CloudFormation parameters as a yaml string.
func (s *Stack) Parameters() (string, error) {
	if s.parameters.IsZero() {
		return "", nil
	}

	return s.encode(s.parameters)
}

// encode encodes v as a yaml string indented with 2 spaces.
func (s *Stack) encode(v any) (string, error) {
	str := &strings.Builder{}
	enc := yaml.NewEncoder(str)
	enc.SetIndent(2)

	if err := enc.Encode(v); err != nil {
		return "", err
	}

	return str.String(), nil
}

// parseTemplate merges CloudFormation templates under the "addons/" directory of a workload
// into a single CloudFormation template and returns it.
//
// If the addons directory doesn't exist or no yaml files are found in
// the addons directory, it returns the empty string and
// ErrAddonsNotFound.
func parseTemplate(fnames []string, workloadName string, ws workspaceReader) (*cfnTemplate, error) {
	templateFiles := filterFiles(fnames, yamlMatcher, nonParamsMatcher)
	if len(templateFiles) == 0 {
		return nil, &ErrAddonsNotFound{
			WlName: workloadName,
		}
	}

	mergedTemplate := newCFNTemplate("merged")
	for _, fname := range templateFiles {
		out, err := ws.ReadAddon(workloadName, fname)
		if err != nil {
			return nil, fmt.Errorf("read addon %s under %s: %w", fname, workloadName, err)
		}
		tpl := newCFNTemplate(fname)
		if err := yaml.Unmarshal(out, tpl); err != nil {
			return nil, fmt.Errorf("unmarshal addon %s under %s: %w", fname, workloadName, err)
		}
		if err := mergedTemplate.merge(tpl); err != nil {
			return nil, err
		}
	}

	return mergedTemplate, nil
}

// parseParameters returns the content of user-defined additional CloudFormation Parameters
// to pass from the parent stack to Template.
//
// If there are addons but no parameters file defined, then returns "" and nil for error.
// If there are multiple parameters files, then returns "" and cannot define multiple parameter files error.
// If the addons parameters use the reserved parameter names, then returns "" and a reserved parameter error.
func parseParameters(fnames []string, workloadName string, ws workspaceReader) (yaml.Node, error) {
	paramFiles := filterFiles(fnames, paramsMatcher)
	if len(paramFiles) == 0 {
		return yaml.Node{}, nil
	}
	if len(paramFiles) > 1 {
		return yaml.Node{}, fmt.Errorf("defining %s is not allowed under %s addons/", english.WordSeries(parameterFileNames, "and"), workloadName)
	}
	paramFile := paramFiles[0]
	raw, err := ws.ReadAddon(workloadName, paramFile)
	if err != nil {
		return yaml.Node{}, fmt.Errorf("read parameter file %s under %s addons/: %w", paramFile, workloadName, err)
	}
	content := struct {
		Parameters yaml.Node `yaml:"Parameters"`
	}{}
	if err := yaml.Unmarshal(raw, &content); err != nil {
		return yaml.Node{}, fmt.Errorf("unmarshal 'Parameters' in file %s under %s addons/: %w", paramFile, workloadName, err)
	}
	if content.Parameters.IsZero() {
		return yaml.Node{}, fmt.Errorf("must define field 'Parameters' in file %s under %s addons/", paramFile, workloadName)
	}
	if err := validateReservedParameters(content.Parameters, paramFile, workloadName); err != nil {
		return yaml.Node{}, err
	}

	return content.Parameters, nil
}

func validateReservedParameters(params yaml.Node, fname, workloadName string) error {
	content := struct {
		App  yaml.Node `yaml:"App"`
		Env  yaml.Node `yaml:"Env"`
		Name yaml.Node `yaml:"Name"`
	}{}
	if err := params.Decode(&content); err != nil {
		return fmt.Errorf("decode content of parameters file %s under %s addons/", fname, workloadName)
	}

	for _, field := range []yaml.Node{content.App, content.Env, content.Name} {
		if !field.IsZero() {
			return fmt.Errorf("reserved parameters 'App', 'Env', and 'Name' cannot be declared in %s under %s addons/", fname, workloadName)
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

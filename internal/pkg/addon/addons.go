// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addon contains the service to manage addons.
package addon

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize/english"
	"gopkg.in/yaml.v3"
)

const (
	// StackName is the name of the addons nested stack resource.
	StackName = "AddonsStack"
)

var (
	wkldAddonsParameterReservedKeys = []string{"App", "Env", "Name"}
	envAddonsParameterReservedKeys  = []string{"App", "Env"}
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

// WorkspaceAddonsReader finds and reads addons from a workspace.
type WorkspaceAddonsReader interface {
	WorkloadAddonsPath(name string) string
	WorkloadAddonFilePath(wkldName, fName string) string
	EnvAddonsPath() string
	EnvAddonFilePath(fName string) string
	ListFiles(dirPath string) ([]string, error)
	ReadFile(fPath string) ([]byte, error)
}

// WorkloadStack represents a CloudFormation stack for workload addons.
type WorkloadStack struct {
	stack
	workloadName string
}

// EnvironmentStack represents a CloudFormation stack for environment addons.
type EnvironmentStack struct {
	stack
}

type stack struct {
	template   *cfnTemplate
	parameters yaml.Node
}

type parser struct {
	ws                 WorkspaceAddonsReader
	addonsDirPath      func() string
	addonsFilePath     func(fName string) string
	validateParameters func(params yaml.Node) error
}

// ParseFromWorkload parses the 'addon/' directory for the given workload
// and returns a Stack created by merging the CloudFormation templates
// files found there. If no addons are found, ParseFromWorkload returns a nil
// Stack and ErrAddonsNotFound.
func ParseFromWorkload(workloadName string, ws WorkspaceAddonsReader) (*WorkloadStack, error) {
	parser := parser{
		ws: ws,
		addonsDirPath: func() string {
			return ws.WorkloadAddonsPath(workloadName)
		},
		addonsFilePath: func(fName string) string {
			return ws.WorkloadAddonFilePath(workloadName, fName)
		},
		validateParameters: func(params yaml.Node) error {
			return validateReservedParameters(params, wkldAddonsParameterReservedKeys)
		},
	}
	stack, err := parser.stack()
	if err != nil {
		return nil, err
	}
	return &WorkloadStack{
		stack:        *stack,
		workloadName: workloadName,
	}, nil
}

// ParseFromEnv parses the 'addon/' directory for environments
// and returns a Stack created by merging the CloudFormation templates
// files found there. If no addons are found, ParseFromWorkload returns a nil
// Stack and ErrAddonsNotFound.
func ParseFromEnv(ws WorkspaceAddonsReader) (*EnvironmentStack, error) {
	parser := parser{
		ws:             ws,
		addonsDirPath:  ws.EnvAddonsPath,
		addonsFilePath: ws.EnvAddonFilePath,
		validateParameters: func(params yaml.Node) error {
			return validateReservedParameters(params, envAddonsParameterReservedKeys)
		},
	}
	stack, err := parser.stack()
	if err != nil {
		return nil, err
	}
	return &EnvironmentStack{
		stack: *stack,
	}, nil
}

// Template returns Stack's CloudFormation template as a yaml string.
func (s *stack) Template() (string, error) {
	if s.template == nil {
		return "", nil
	}

	return s.encode(s.template)
}

// Parameters returns Stack's CloudFormation parameters as a yaml string.
func (s *stack) Parameters() (string, error) {
	if s.parameters.IsZero() {
		return "", nil
	}

	return s.encode(s.parameters)
}

// encode encodes v as a yaml string indented with 2 spaces.
func (s *stack) encode(v any) (string, error) {
	str := &strings.Builder{}
	enc := yaml.NewEncoder(str)
	enc.SetIndent(2)

	if err := enc.Encode(v); err != nil {
		return "", err
	}

	return str.String(), nil
}

func (p *parser) stack() (*stack, error) {
	path := p.addonsDirPath()
	fNames, err := p.ws.ListFiles(path)
	if err != nil {
		return nil, fmt.Errorf("list addons under path %s: %w", path, &ErrAddonsNotFound{
			ParentErr: err,
		})
	}
	template, err := p.parseTemplate(fNames)
	if err != nil {
		return nil, err
	}
	params, err := p.parseParameters(fNames)
	if err != nil {
		return nil, err
	}
	return &stack{
		template:   template,
		parameters: params,
	}, nil
}

// parseTemplate merges CloudFormation templates under the "addons/" directory  into a single CloudFormation 
// template and returns it.
//
// If the addons directory doesn't exist or no yaml files are found in
// the addons directory, it returns the empty string and
// ErrAddonsNotFound.
func (p *parser) parseTemplate(fNames []string) (*cfnTemplate, error) {
	templateFiles := filterFiles(fNames, yamlMatcher, nonParamsMatcher)
	if len(templateFiles) == 0 {
		return nil, &ErrAddonsNotFound{}
	}

	mergedTemplate := newCFNTemplate("merged")
	for _, fname := range templateFiles {
		path := p.addonsFilePath(fname)
		out, err := p.ws.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read addons file %q under path %s: %w", fname, path, err)
		}
		tpl := newCFNTemplate(fname)
		if err := yaml.Unmarshal(out, tpl); err != nil {
			return nil, fmt.Errorf("unmarshal addon %s under path %s: %w", fname, path, err)
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
func (p *parser) parseParameters(fNames []string) (yaml.Node, error) {
	paramFiles := filterFiles(fNames, paramsMatcher)
	if len(paramFiles) == 0 {
		return yaml.Node{}, nil
	}
	if len(paramFiles) > 1 {
		return yaml.Node{}, fmt.Errorf("defining %s is not allowed under addons/", english.WordSeries(parameterFileNames, "and"))
	}
	paramFile := paramFiles[0]
	path := p.addonsFilePath(paramFile)
	raw, err := p.ws.ReadFile(path)
	if err != nil {
		return yaml.Node{}, fmt.Errorf("read parameter file %s under path %s: %w", paramFile, path, err)
	}
	content := struct {
		Parameters yaml.Node `yaml:"Parameters"`
	}{}
	if err := yaml.Unmarshal(raw, &content); err != nil {
		return yaml.Node{}, fmt.Errorf("unmarshal 'Parameters' in file %s: %w", paramFile, err)
	}
	if content.Parameters.IsZero() {
		return yaml.Node{}, fmt.Errorf("must define field 'Parameters' in file %s under path %s", paramFile, path)
	}
	if err := p.validateParameters(content.Parameters); err != nil {
		return yaml.Node{}, err
	}

	return content.Parameters, nil
}

func validateReservedParameters(params yaml.Node, reservedKeys []string) error {
	content := make(map[string]yaml.Node, len(reservedKeys))
	if err := params.Decode(&content); err != nil {
		return fmt.Errorf("decode \"Parameters\" section of the parameters file: %w", err)
	}
	for _, key := range reservedKeys {
		if _, ok := content[key]; ok {
			return fmt.Errorf("reserved parameters %s cannot be declared", english.WordSeries(quoteSlice(reservedKeys), "and"))
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

func quoteSlice(elems []string) []string {
	if len(elems) == 0 {
		return nil
	}
	quotedElems := make([]string, len(elems))
	for i, el := range elems {
		quotedElems[i] = strconv.Quote(el)
	}
	return quotedElems
}

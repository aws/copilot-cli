// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package override defines functionality to interact with the "overrides/" directory
// for accessing and mutating the Copilot generated AWS CloudFormation templates.
package override

import (
	"fmt"
	"path/filepath"

	"github.com/aws/copilot-cli/internal/pkg/template"

	"gopkg.in/yaml.v3"

	"github.com/spf13/afero"
)

// Info holds metadata about an overrider.
type Info struct {
	path string
	mode overriderMode
}

func cdkInfo(path string) Info {
	return Info{
		path: path,
		mode: cdkOverrider,
	}
}

func yamlPatchInfo(path string) Info {
	return Info{
		path: path,
		mode: yamlPatchOverrider,
	}
}

type overriderMode int

const (
	cdkOverrider overriderMode = iota + 1
	yamlPatchOverrider
)

var templates = template.New()

// Path returns the path to the overrider.
// For CDK applications, returns the root of the CDK directory.
// For YAML patch documents, returns the path to the file.
func (i Info) Path() string {
	return i.path
}

// IsCDK returns true if the overrider is a CDK application.
func (i Info) IsCDK() bool {
	return i.mode == cdkOverrider
}

// IsYAMLPatch returns true if the overrider is a YAML patch document.
func (i Info) IsYAMLPatch() bool {
	return i.mode == yamlPatchOverrider
}

// Lookup returns information indicating if the overrider is a CDK application or YAML Patch document.
// If path does not exist or is an empty directory, then return an ErrNotExist.
// If path is a YAML patch document, then IsYAMLPatch evaluates to true.
// If path is a directory that contains a cdk.json file, then IsCDK evaluates to true.
func Lookup(path string, fs afero.Fs) (Info, error) {
	stat, err := fs.Stat(path)
	if err != nil {
		return Info{}, &ErrNotExist{parent: err}
	}

	if !stat.IsDir() {
		return lookupYAMLPatch(path, fs)
	}

	files, err := afero.ReadDir(fs, path)
	if err != nil {
		return Info{}, fmt.Errorf("read directory %q: %w", path, err)
	}
	var info Info
	switch n := len(files); n {
	case 0:
		return Info{}, fmt.Errorf(`directory at %q is empty`, path)
	case 1:
		info, err = lookupYAMLPatch(filepath.Join(path, files[0].Name()), fs)
		if err != nil {
			return Info{}, fmt.Errorf("look up YAML patch document when directory contains a single file: %w", err)
		}
	default:
		info, err = lookupCDK(path, fs)
		if err != nil {
			return Info{}, fmt.Errorf("look up CDK project for directories with multiple files: %w", err)
		}
	}
	return info, nil
}

type extension string

func (ext extension) isYAML() bool { return ext == ".yml" || ext == ".yaml" }

func lookupYAMLPatch(path string, fs afero.Fs) (Info, error) {
	if ext := extension(filepath.Ext(path)); !ext.isYAML() {
		return Info{}, fmt.Errorf(`YAML patch documents require a ".yml" or ".yaml" extension: %q has a %q extension`, path, ext)
	}

	content, err := afero.ReadFile(fs, path)
	if err != nil {
		return Info{}, fmt.Errorf("read file at %q: %w", path, err)
	}
	var doc []yamlPatch
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return Info{}, fmt.Errorf("file at %q does not conform to the YAML patch document schema: %w", path, err)
	}
	if len(doc) == 0 {
		return Info{}, fmt.Errorf("YAML patch document at %q does not contain any operations", path)
	}
	return yamlPatchInfo(path), nil
}

func lookupCDK(path string, fs afero.Fs) (Info, error) {
	ok, _ := afero.Exists(fs, filepath.Join(path, "cdk.json"))
	if !ok {
		return Info{}, fmt.Errorf(`"cdk.json" does not exist under %q`, path)
	}
	return cdkInfo(path), nil
}

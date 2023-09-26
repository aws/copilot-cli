// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package override defines functionality to interact with the "overrides/" directory
// for accessing and mutating the Copilot generated AWS CloudFormation templates.
package override

import (
	"fmt"
	"path/filepath"

	"github.com/aws/copilot-cli/internal/pkg/template"

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

// Lookup returns information indicating if the overrider is a CDK application or YAML Patches.
// If path does not exist, then return an ErrNotExist.
// If path is a directory that contains cfn.patches.yml, then IsYAMLPatch evaluates to true.
// If path is a directory that contains a cdk.json file, then IsCDK evaluates to true.
func Lookup(path string, fs afero.Fs) (Info, error) {
	_, err := fs.Stat(path)
	if err != nil {
		return Info{}, &ErrNotExist{parent: err}
	}

	files, err := afero.ReadDir(fs, path)
	switch {
	case err != nil:
		return Info{}, fmt.Errorf("read directory %q: %w", path, err)
	case len(files) == 0:
		return Info{}, fmt.Errorf(`directory at %q is empty`, path)
	}

	info, err := lookupYAMLPatch(path, fs)
	if err == nil { // return yaml info if no error
		return info, nil
	}

	return lookupCDK(path, fs)
}

func lookupYAMLPatch(path string, fs afero.Fs) (Info, error) {
	ok, _ := afero.Exists(fs, filepath.Join(path, YAMLPatchFile))
	if !ok {
		return Info{}, fmt.Errorf(`%s does not exist under %q`, YAMLPatchFile, path)
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

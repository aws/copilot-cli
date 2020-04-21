// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package addons contains the service to manage addons.
package addons

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/template"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
)

const (
	// StackName is the name of the addons nested stack resource.
	StackName = "AddonsStack"

	addonsTemplatePath = "addons/cf.yml"

	paramsFileWithoutExt  = "params"
	outputsFileWithoutExt = "outputs"
	resourcesFiles        = "resources"
)

type workspaceService interface {
	ReadAddonsDir(appName string) ([]string, error)
	ReadAddonsFile(appName, fileName string) ([]byte, error)
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
	fileNames, err := a.ws.ReadAddonsDir(a.appName)
	if err != nil {
		return "", &ErrDirNotExist{
			AppName:   a.appName,
			ParentErr: err,
		}
	}

	addonFiles := make(map[string]string)
	for _, fileName := range filterYAMLfiles(fileNames) {
		content, err := a.ws.ReadAddonsFile(a.appName, fileName)
		if err != nil {
			return "", fmt.Errorf("read addons file %s under application %s: %w", fileName, a.appName, err)
		}
		trimmedContent := strings.TrimSpace(string(content))
		switch nameWithoutExt := strings.TrimSuffix(fileName, filepath.Ext(fileName)); nameWithoutExt {
		case paramsFileWithoutExt:
			addonFiles[paramsFileWithoutExt] = trimmedContent
		case outputsFileWithoutExt:
			addonFiles[outputsFileWithoutExt] = trimmedContent
		default:
			addonFiles[resourcesFiles] += trimmedContent + "\n"
		}
	}
	if err := validateNoMissingFiles(addonFiles); err != nil {
		return "", err
	}

	content, err := a.parser.Parse(addonsTemplatePath, struct {
		AppName    string
		Parameters []string
		Resources  []string
		Outputs    []string
	}{
		AppName:    a.appName,
		Parameters: strings.Split(strings.TrimSpace(addonFiles[paramsFileWithoutExt]), "\n"),
		Resources:  strings.Split(strings.TrimSpace(addonFiles[resourcesFiles]), "\n"),
		Outputs:    strings.Split(strings.TrimSpace(addonFiles[outputsFileWithoutExt]), "\n"),
	})
	if err != nil {
		return "", err
	}
	return content.String(), nil
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

func validateNoMissingFiles(f map[string]string) error {
	var missingFiles []string
	if f[paramsFileWithoutExt] == "" {
		missingFiles = append(missingFiles, fmt.Sprintf("%s.yaml", paramsFileWithoutExt))
	}
	if f[outputsFileWithoutExt] == "" {
		missingFiles = append(missingFiles, fmt.Sprintf("%s.yaml", outputsFileWithoutExt))
	}
	if f[resourcesFiles] == "" {
		missingFiles = append(missingFiles, `at least one resource YAML file such as "s3-bucket.yaml"`)
	}

	if missingFiles != nil {
		return fmt.Errorf("addons directory has missing file(s): %s", strings.Join(missingFiles, ", "))
	}
	return nil
}

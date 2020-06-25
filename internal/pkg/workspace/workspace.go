// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package workspace contains functionality to manage a user's local workspace. This includes
// creating an application directory, reading and writing a summary file to associate the workspace with the application,
// and managing infrastructure-as-code files. The typical workspace will be structured like:
//  .
//  ├── copilot                        (application directory)
//  │   ├── .workspace                 (workspace summary)
//  │   └── my-service
//  │   │   └── manifest.yml           (service manifest)
//  │   ├── buildspec.yml              (buildspec for the pipeline's build stage)
//  │   └── pipeline.yml               (pipeline manifest)
//  └── my-service-src                 (customer service code)
package workspace

import (
	"encoding"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

const (
	// CopilotDirName is the name of the directory where generated infrastructure code for an application will be stored.
	CopilotDirName = "copilot"
	// SummaryFileName is the name of the file that is associated with the application.
	SummaryFileName = ".workspace"

	addonsDirName             = "addons"
	maximumParentDirsToSearch = 5
	pipelineFileName          = "pipeline.yml"
	manifestFileName          = "manifest.yml"
	buildspecFileName         = "buildspec.yml"

	ymlFileExtension = ".yml"
)

// Summary is a description of what's associated with this workspace.
type Summary struct {
	Application string `yaml:"application"` // Name of the application.
}

// Workspace typically represents a Git repository where the user has its infrastructure-as-code files as well as source files.
type Workspace struct {
	workingDir string
	copilotDir string
	fsUtils    *afero.Afero
}

// New returns a workspace, used for reading and writing to user's local workspace.
func New() (*Workspace, error) {
	fs := afero.NewOsFs()
	fsUtils := &afero.Afero{Fs: fs}

	workingDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ws := Workspace{
		workingDir: workingDir,
		fsUtils:    fsUtils,
	}

	return &ws, nil
}

// Create creates the copilot directory (if it doesn't already exist) in the current working directory,
// and saves a summary with the application name.
func (ws *Workspace) Create(appName string) error {
	// Create an application directory, if one doesn't exist
	if err := ws.createCopilotDir(); err != nil {
		return err
	}

	// Grab an existing workspace summary, if one exists.
	summary, err := ws.Summary()
	if err == nil {
		// If a summary exists, but is registered to a different application, throw an error.
		if summary.Application != appName {
			return &errHasExistingApplication{existingAppName: summary.Application}
		}
		// Otherwise our work is all done.
		return nil
	}

	// If there isn't an existing workspace summary, create it.
	var notFound *errNoAssociatedApplication
	if errors.As(err, &notFound) {
		return ws.writeSummary(appName)
	}

	return err
}

// Summary returns a summary of the workspace - including the application name.
func (ws *Workspace) Summary() (*Summary, error) {
	summaryPath, err := ws.summaryPath()
	if err != nil {
		return nil, err
	}
	summaryFileExists, err := ws.fsUtils.Exists(summaryPath)
	if summaryFileExists {
		value, err := ws.fsUtils.ReadFile(summaryPath)
		if err != nil {
			return nil, err
		}
		wsSummary := Summary{}
		return &wsSummary, yaml.Unmarshal(value, &wsSummary)
	}
	return nil, &errNoAssociatedApplication{}
}

// ServiceNames returns the names of the services in the workspace.
func (ws *Workspace) ServiceNames() ([]string, error) {
	copilotPath, err := ws.copilotDirPath()
	if err != nil {
		return nil, err
	}
	files, err := ws.fsUtils.ReadDir(copilotPath)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", copilotPath, err)
	}
	var names []string
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		if exists, _ := ws.fsUtils.Exists(filepath.Join(copilotPath, f.Name(), manifestFileName)); !exists {
			// Swallow the error because we don't want to include any services that we don't have permissions to read.
			continue
		}
		names = append(names, f.Name())
	}
	return names, nil
}

// ReadServiceManifest returns the contents of the service manifest under copilot/{name}/manifest.yml.
func (ws *Workspace) ReadServiceManifest(name string) ([]byte, error) {
	return ws.read(name, manifestFileName)
}

// ReadPipelineManifest returns the contents of the pipeline manifest under copilot/pipeline.yml.
func (ws *Workspace) ReadPipelineManifest() ([]byte, error) {
	pmPath, err := ws.pipelineManifestPath()
	if err != nil {
		return nil, err
	}
	manifestExists, err := ws.fsUtils.Exists(pmPath)

	if err != nil {
		return nil, err
	}
	if !manifestExists {
		return nil, ErrNoPipelineInWorkspace
	}
	return ws.read(pipelineFileName)
}

// WriteServiceManifest writes the service's manifest under the copilot/{name}/ directory.
func (ws *Workspace) WriteServiceManifest(marshaler encoding.BinaryMarshaler, name string) (string, error) {
	data, err := marshaler.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal service %s manifest to binary: %w", name, err)
	}
	return ws.write(data, name, manifestFileName)
}

// WritePipelineBuildspec writes the pipeline buildspec under the copilot/ directory.
// If successful returns the full path of the file, otherwise returns an empty string and the error.
func (ws *Workspace) WritePipelineBuildspec(marshaler encoding.BinaryMarshaler) (string, error) {
	data, err := marshaler.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal pipeline buildspec to binary: %w", err)
	}
	return ws.write(data, buildspecFileName)
}

// WritePipelineManifest writes the pipeline manifest under the copilot directory.
// If successful returns the full path of the file, otherwise returns an empty string and the error.
func (ws *Workspace) WritePipelineManifest(marshaler encoding.BinaryMarshaler) (string, error) {
	data, err := marshaler.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal pipeline manifest to binary: %w", err)
	}
	return ws.write(data, pipelineFileName)
}

// DeleteWorkspaceFile removes the .workspace file under copilot/ directory.
// This will be called during app delete, we do not want to delete any other generated files
func (ws *Workspace) DeleteWorkspaceFile() error {
	return ws.fsUtils.Remove(filepath.Join(CopilotDirName, SummaryFileName))
}

// ReadAddonsDir returns a list of file names under a service's "addons/" directory.
func (ws *Workspace) ReadAddonsDir(svcName string) ([]string, error) {
	copilotPath, err := ws.copilotDirPath()
	if err != nil {
		return nil, err
	}

	var names []string
	files, err := ws.fsUtils.ReadDir(filepath.Join(copilotPath, svcName, addonsDirName))
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		names = append(names, f.Name())
	}
	return names, nil
}

// ReadAddon returns the contents of a file under the service's "addons/" directory.
func (ws *Workspace) ReadAddon(svc, fname string) ([]byte, error) {
	return ws.read(svc, addonsDirName, fname)
}

// WriteAddon writes the content of an addon file under "{svc}/addons/{name}.yml".
// If successful returns the full path of the file, otherwise an empty string and an error.
func (ws *Workspace) WriteAddon(content encoding.BinaryMarshaler, svc, name string) (string, error) {
	data, err := content.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal binary addon content: %w", err)
	}
	fname := name + ymlFileExtension
	return ws.write(data, svc, addonsDirName, fname)
}

func (ws *Workspace) writeSummary(appName string) error {
	summaryPath, err := ws.summaryPath()
	if err != nil {
		return err
	}

	workspaceSummary := Summary{
		Application: appName,
	}

	serializedWorkspaceSummary, err := yaml.Marshal(workspaceSummary)

	if err != nil {
		return err
	}
	return ws.fsUtils.WriteFile(summaryPath, serializedWorkspaceSummary, 0644)
}

func (ws *Workspace) pipelineManifestPath() (string, error) {
	copilotPath, err := ws.copilotDirPath()
	if err != nil {
		return "", err
	}
	pipelineManifestPath := filepath.Join(copilotPath, pipelineFileName)
	return pipelineManifestPath, nil
}

func (ws *Workspace) summaryPath() (string, error) {
	copilotPath, err := ws.copilotDirPath()
	if err != nil {
		return "", err
	}
	workspaceSummaryPath := filepath.Join(copilotPath, SummaryFileName)
	return workspaceSummaryPath, nil
}

func (ws *Workspace) createCopilotDir() error {
	// First check to see if a manifest directory already exists
	existingWorkspace, _ := ws.copilotDirPath()
	if existingWorkspace != "" {
		return nil
	}
	return ws.fsUtils.Mkdir(CopilotDirName, 0755)
}

func (ws *Workspace) copilotDirPath() (string, error) {
	if ws.copilotDir != "" {
		return ws.copilotDir, nil
	}
	// Are we in the application directory?
	inCopilotDir := filepath.Base(ws.workingDir) == CopilotDirName
	if inCopilotDir {
		ws.copilotDir = ws.workingDir
		return ws.copilotDir, nil
	}

	searchingDir := ws.workingDir
	for try := 0; try < maximumParentDirsToSearch; try++ {
		currentDirectoryPath := filepath.Join(searchingDir, CopilotDirName)
		inCurrentDirPath, err := ws.fsUtils.DirExists(currentDirectoryPath)
		if err != nil {
			return "", err
		}
		if inCurrentDirPath {
			ws.copilotDir = currentDirectoryPath
			return ws.copilotDir, nil
		}
		searchingDir = filepath.Dir(searchingDir)
	}
	return "", &errWorkspaceNotFound{
		CurrentDirectory:      ws.workingDir,
		ManifestDirectoryName: CopilotDirName,
		NumberOfLevelsChecked: maximumParentDirsToSearch,
	}
}

// write flushes the data to a file under the copilot directory joined by path elements.
func (ws *Workspace) write(data []byte, elem ...string) (string, error) {
	copilotPath, err := ws.copilotDirPath()
	if err != nil {
		return "", err
	}
	pathElems := append([]string{copilotPath}, elem...)
	filename := filepath.Join(pathElems...)

	if err := ws.fsUtils.MkdirAll(filepath.Dir(filename), 0755 /* -rwxr-xr-x */); err != nil {
		return "", fmt.Errorf("create directories for file %s: %w", filename, err)
	}
	exist, err := ws.fsUtils.Exists(filename)
	if err != nil {
		return "", fmt.Errorf("check if manifest file %s exists: %w", filename, err)
	}
	if exist {
		return "", &ErrFileExists{FileName: filename}
	}
	if err := ws.fsUtils.WriteFile(filename, data, 0644 /* -rw-r--r-- */); err != nil {
		return "", fmt.Errorf("write manifest file: %w", err)
	}
	return filename, nil
}

// read returns the contents of the file under the copilot directory joined by path elements.
func (ws *Workspace) read(elem ...string) ([]byte, error) {
	copilotPath, err := ws.copilotDirPath()
	if err != nil {
		return nil, err
	}
	pathElems := append([]string{copilotPath}, elem...)
	return ws.fsUtils.ReadFile(filepath.Join(pathElems...))
}

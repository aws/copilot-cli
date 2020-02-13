// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package workspace contains functionality to manage a user's local workspace. This includes
// creating a project directory, reading and writing a summary file
// to that directory and managing (reading, writing, and listing) infrastructure-as-code files.
// The typical workspace will be structured like:
//  .
//  ├── ecs-project                    (project directory)
//  │   ├── .ecs-workspace             (workspace summary)
//  │   └── my-app
//  │   │   └── manifest.yml             (application manifest)
//  │   ├── buildspec.yml              (buildspec for the pipeline's build stage)
//  │   └── pipeline.yml               (pipeline manifest)
//  └── my-app                         (customer application)
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

const (
	// ProjectDirectoryName is the name of the directory where generated infrastructure code will be stored.
	ProjectDirectoryName = "ecs-project"
	// PipelineFileName is the name of the pipeline manifest file.
	PipelineFileName = "pipeline.yml"
	// BuildspecFileName is the name of the CodeBuild build specification for the "build" stage of the pipeline.
	BuildspecFileName = "buildspec.yml"
	// ManifestFileName is the name of the file that describes the architecture of an application.
	ManifestFileName = "manifest.yml"

	workspaceSummaryFileName  = ".ecs-workspace"
	maximumParentDirsToSearch = 5
	appManifestFileSuffix     = "-app.yml"
	fmtAppManifestFileName    = "%s" + appManifestFileSuffix
)

// Summary is a description of what's associated with this workspace.
type Summary struct {
	ProjectName string `yaml:"project"`
}

// Workspace manages a local workspace, including creating and managing manifest files.
type Workspace struct {
	workingDir string
	projectDir string
	fsUtils    *afero.Afero
}

// New returns a workspace, used for reading and writing to
// user's local workspace.
func New() (*Workspace, error) {
	appFs := afero.NewOsFs()
	fsUtils := &afero.Afero{Fs: appFs}

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

// Create creates the manifest directory (if it doesn't already exist) in
// the current working directory, and saves a summary in the manifest
// directory with the project name.
func (ws *Workspace) Create(projectName string) error {
	// Create a manifest directory, if one doesn't exist
	if createDirErr := ws.createProjectDirectory(); createDirErr != nil {
		return createDirErr
	}

	// Grab an existing workspace summary, if one exists.
	existingWorkspaceSummary, existingWorkspaceSummaryErr := ws.Summary()

	if existingWorkspaceSummaryErr == nil {
		// If a summary exists, but is registered to a different project, throw an error.
		if existingWorkspaceSummary.ProjectName != projectName {
			return &ErrWorkspaceHasExistingProject{ExistingProjectName: existingWorkspaceSummary.ProjectName}
		}
		// Otherwise our work is all done.
		return nil
	}

	// If there isn't an existing workspace summary, create it.
	var noProjectFound *ErrNoProjectAssociated
	if errors.As(existingWorkspaceSummaryErr, &noProjectFound) {
		return ws.writeSummary(projectName)
	}

	return existingWorkspaceSummaryErr
}

// Summary returns a summary of the workspace - including the project name.
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
	return nil, &ErrNoProjectAssociated{}
}

// AppNames returns the application names in the workspace.
func (ws *Workspace) AppNames() ([]string, error) {
	projectPath, err := ws.projectDirPath()
	if err != nil {
		return nil, err
	}
	files, err := ws.fsUtils.ReadDir(projectPath)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", projectPath, err)
	}
	var names []string
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		names = append(names, filepath.Base(f.Name()))
	}
	return names, nil
}

// Read returns the contents of the file under the project directory joined by path elements.
func (ws *Workspace) Read(elem ...string) ([]byte, error) {
	projectPath, err := ws.projectDirPath()
	if err != nil {
		return nil, err
	}
	pathElems := append([]string{projectPath}, elem...)
	return ws.fsUtils.ReadFile(filepath.Join(pathElems...))
}

// Write writes the data to a file under the project directory joined by path elements.
// If successful returns the full path of the file, otherwise returns an empty string and the error.
func (ws *Workspace) Write(data []byte, elem ...string) (string, error) {
	projectPath, err := ws.projectDirPath()
	if err != nil {
		return "", err
	}
	pathElems := append([]string{projectPath}, elem...)
	filename := filepath.Join(pathElems...)

	if err := ws.fsUtils.MkdirAll(filepath.Dir(filename), 0755 /* -rwxr-xr-x */); err != nil {
		return "", fmt.Errorf("failed to create directories for file %s: %w", filename, err)
	}
	if err := ws.fsUtils.WriteFile(filename, data, 0644 /* -rw-r--r-- */); err != nil {
		return "", fmt.Errorf("failed to write manifest file: %w", err)
	}
	return filename, nil
}

// DeleteApp removes the application directory from the project directory.
func (ws *Workspace) DeleteApp(name string) error {
	projectPath, err := ws.projectDirPath()
	if err != nil {
		return err
	}
	return ws.fsUtils.RemoveAll(filepath.Join(projectPath, name))
}

// DeleteAll removes the local project directory.
func (ws *Workspace) DeleteAll() error {
	return ws.fsUtils.RemoveAll(ProjectDirectoryName)
}

func (ws *Workspace) writeSummary(projectName string) error {
	summaryPath, err := ws.summaryPath()
	if err != nil {
		return err
	}

	workspaceSummary := Summary{
		ProjectName: projectName,
	}

	serializedWorkspaceSummary, err := yaml.Marshal(workspaceSummary)

	if err != nil {
		return err
	}
	return ws.fsUtils.WriteFile(summaryPath, serializedWorkspaceSummary, 0644)
}

func (ws *Workspace) summaryPath() (string, error) {
	manifestPath, err := ws.projectDirPath()
	if err != nil {
		return "", err
	}
	workspaceSummaryPath := filepath.Join(manifestPath, workspaceSummaryFileName)
	return workspaceSummaryPath, nil
}

func (ws *Workspace) createProjectDirectory() error {
	// First check to see if a manifest directory already exists
	existingWorkspace, _ := ws.projectDirPath()
	if existingWorkspace != "" {
		return nil
	}
	return ws.fsUtils.Mkdir(ProjectDirectoryName, 0755)
}

func (ws *Workspace) projectDirPath() (string, error) {
	if ws.projectDir != "" {
		return ws.projectDir, nil
	}
	// Are we in the project directory?
	inEcsDir := filepath.Base(ws.workingDir) == ProjectDirectoryName
	if inEcsDir {
		ws.projectDir = ws.workingDir
		return ws.projectDir, nil
	}

	searchingDir := ws.workingDir
	for try := 0; try < maximumParentDirsToSearch; try++ {
		currentDirectoryPath := filepath.Join(searchingDir, ProjectDirectoryName)
		inCurrentDirPath, err := ws.fsUtils.DirExists(currentDirectoryPath)
		if err != nil {
			return "", err
		}
		if inCurrentDirPath {
			ws.projectDir = currentDirectoryPath
			return ws.projectDir, nil
		}
		searchingDir = filepath.Dir(searchingDir)
	}
	return "", &ErrWorkspaceNotFound{
		CurrentDirectory:      ws.workingDir,
		ManifestDirectoryName: ProjectDirectoryName,
		NumberOfLevelsChecked: maximumParentDirsToSearch,
	}
}

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
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
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
func (ws *Workspace) Summary() (*archer.WorkspaceSummary, error) {
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
		wsSummary := archer.WorkspaceSummary{}
		return &wsSummary, yaml.Unmarshal(value, &wsSummary)
	}
	return nil, &ErrNoProjectAssociated{}
}

// Apps returns all the applications in the workspace
func (ws *Workspace) Apps() ([]archer.Manifest, error) {
	manifestFiles, err := ws.ListManifestFiles()
	if err != nil {
		return nil, err
	}
	apps := make([]archer.Manifest, 0, len(manifestFiles))
	for _, file := range manifestFiles {
		manifestBytes, err := ws.ReadFile(file)
		if err != nil {
			return nil, err
		}

		mf, err := manifest.UnmarshalApp(manifestBytes)
		if err != nil {
			return nil, err
		}
		apps = append(apps, mf)
	}
	return apps, nil
}

// ListManifestFiles returns a list of all the local application manifest filenames.
// TODO add pipeline manifest ls?
func (ws *Workspace) ListManifestFiles() ([]string, error) {
	projectPath, err := ws.projectDirPath()
	if err != nil {
		return nil, err
	}
	manifestDirFiles, err := ws.fsUtils.ReadDir(projectPath)
	if err != nil {
		return nil, err
	}

	var manifestFiles []string
	for _, file := range manifestDirFiles {
		if !file.IsDir() && strings.HasSuffix(file.Name(), appManifestFileSuffix) {
			manifestFiles = append(manifestFiles, file.Name())
		}
	}

	return manifestFiles, nil
}

// ReadFile takes in a file name under the project directory (e.g. frontend-app.yml) and returns the read bytes.
func (ws *Workspace) ReadFile(filename string) ([]byte, error) {
	projectPath, err := ws.projectDirPath()
	if err != nil {
		return nil, err
	}
	manifestPath := filepath.Join(projectPath, filename)
	manifestFileExists, err := ws.fsUtils.Exists(manifestPath)

	if err != nil {
		return nil, err
	}

	if !manifestFileExists {
		return nil, &ErrManifestNotFound{ManifestName: filename}
	}

	value, err := ws.fsUtils.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// AppManifestFileName returns the manifest's name from an application name.
// TODO extend this to pipeline manifest filenames too
func (ws *Workspace) AppManifestFileName(appName string) string {
	return fmt.Sprintf(fmtAppManifestFileName, appName)
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

func (ws *Workspace) writeSummary(projectName string) error {
	summaryPath, err := ws.summaryPath()
	if err != nil {
		return err
	}

	workspaceSummary := archer.WorkspaceSummary{
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

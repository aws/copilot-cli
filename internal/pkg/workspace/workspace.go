// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package workspace contains the service to manage a user's local workspace. This includes
// creating a manifest directory, reading and writing a summary file
// to that directory and managing manifest files (reading, writing and listing).
// The typical workspace will be structured like:
//  .
//  ├── ecs-project                    (manifest directory)
//  │   ├── .ecs-workspace             (workspace summary)
//  │   ├── my-app.yml                 (application manifest)
//  │   └── pipeline.yml               (pipeline manifest)
//  └── my-app                         (customer application)
//
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

const (
	// ManifestDirectoryName is the name of the directory where application manifests will be stored.
	ManifestDirectoryName = "ecs-project"

	workspaceSummaryFileName  = ".ecs-workspace"
	maximumParentDirsToSearch = 5
	appManifestFileSuffix     = "-app.yml"
	fmtAppManifestFileName    = "%s" + appManifestFileSuffix
)

// Workspace manages a local workspace, including creating and managing manifest files.
type Workspace struct {
	workingDir  string
	manifestDir string
	fsUtils     *afero.Afero
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
	if createDirErr := ws.createManifestDirectory(); createDirErr != nil {
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

// AppNames returns the name of all the local applications. For now it
// extracts the application name from the file name of the corresponding
// application manifest.
func (ws *Workspace) AppNames() ([]string, error) {
	manifests, err := ws.ListManifestFiles()
	if err != nil {
		return nil, err
	}
	var apps []string
	for _, manifest := range manifests {
		appFile := filepath.Base(manifest)
		// TODO: #242 Extract the names of applications from app manifests
		// instead of from file names.
		appName := appFile[0 : len(appFile)-len(appManifestFileSuffix)]
		apps = append(apps, appName)
	}
	return apps, nil
}

func (ws *Workspace) summaryPath() (string, error) {
	manifestPath, err := ws.manifestDirectoryPath()
	if err != nil {
		return "", err
	}
	workspaceSummaryPath := filepath.Join(manifestPath, workspaceSummaryFileName)
	return workspaceSummaryPath, nil
}

func (ws *Workspace) createManifestDirectory() error {
	// First check to see if a manifest directory already exists
	existingWorkspace, _ := ws.manifestDirectoryPath()
	if existingWorkspace != "" {
		return nil
	}
	return ws.fsUtils.Mkdir(ManifestDirectoryName, 0755)
}

func (ws *Workspace) manifestDirectoryPath() (string, error) {
	if ws.manifestDir != "" {
		return ws.manifestDir, nil
	}
	// Are we in the manifest directory?
	inEcsDir := filepath.Base(ws.workingDir) == ManifestDirectoryName
	if inEcsDir {
		ws.manifestDir = ws.workingDir
		return ws.manifestDir, nil
	}

	searchingDir := ws.workingDir
	for try := 0; try < maximumParentDirsToSearch; try++ {
		currentDirectoryPath := filepath.Join(searchingDir, ManifestDirectoryName)
		inCurrentDirPath, err := ws.fsUtils.DirExists(currentDirectoryPath)
		if err != nil {
			return "", err
		}
		if inCurrentDirPath {
			ws.manifestDir = currentDirectoryPath
			return ws.manifestDir, nil
		}
		searchingDir = filepath.Dir(searchingDir)
	}
	return "", &ErrWorkspaceNotFound{
		CurrentDirectory:      ws.workingDir,
		ManifestDirectoryName: ManifestDirectoryName,
		NumberOfLevelsChecked: maximumParentDirsToSearch,
	}
}

// ListManifestFiles returns a list of all the local application manifest filenames.
// TODO add pipeline manifest ls?
func (ws *Workspace) ListManifestFiles() ([]string, error) {
	manifestDir, err := ws.manifestDirectoryPath()
	if err != nil {
		return nil, err
	}
	manifestDirFiles, err := ws.fsUtils.ReadDir(manifestDir)
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

// ReadManifestFile takes in a manifest file (e.g. frontend-app.yml) and returns
// the read bytes.
func (ws *Workspace) ReadManifestFile(manifestFile string) ([]byte, error) {
	manifestDirPath, err := ws.manifestDirectoryPath()
	if err != nil {
		return nil, err
	}
	manifestPath := filepath.Join(manifestDirPath, manifestFile)
	manifestFileExists, err := ws.fsUtils.Exists(manifestPath)

	if err != nil {
		return nil, err
	}

	if !manifestFileExists {
		return nil, &ErrManifestNotFound{ManifestName: manifestFile}
	}

	value, err := ws.fsUtils.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// WriteManifest takes a manifest blob and writes it to the manifest directory.
// If successful returns the path of the manifest file, otherwise returns an empty string and the error.
func (ws *Workspace) WriteManifest(manifestBlob []byte, filename string) (string, error) {
	manifestPath, err := ws.manifestDirectoryPath()
	if err != nil {
		return "", err
	}

	path := filepath.Join(manifestPath, filename)
	if err := ws.fsUtils.WriteFile(path, manifestBlob, 0644); err != nil {
		return "", fmt.Errorf("failed to write manifest file: %w", err)
	}
	return path, nil
}

// ManifestFileName returns the manifest's name from an application name.
// TODO extend this to pipeline manifest filenames too
func (ws *Workspace) AppManifestFileName(appName string) string {
	return fmt.Sprintf(fmtAppManifestFileName, appName)
}

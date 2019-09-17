// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package workspace contains the service to manage a user's local workspace. This includes
// creating a manifest directory, reading and writing a summary file
// to that directory and managing manifest files (reading, writing and listing).
// The typical workspace will be structured like:
//  .
//  ├── ecs             (manifest directory)
//  │   ├── .project    (workspace summary)
//  │   └── my-app.yml  (manifest)
//  └── my-app          (customer application)
//
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/PRIVATE-amazon-ecs-archer/pkg/archer"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

const (
	workspaceSummaryFileName  = ".project"
	manifestDirectoryName     = "ecs"
	maximumParentDirsToSearch = 5
	manifestFileSuffix        = "-app.yml"
)

// Service manages a local workspace, including creating and managing manifest files.
type Service struct {
	workingDir  string
	manifestDir string
	fsUtils     *afero.Afero
}

// New returns a workspace Service, used for reading and writing to
// user's local workspace.
func New() (*Service, error) {
	appFs := afero.NewOsFs()
	fsUtils := &afero.Afero{Fs: appFs}

	workingDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ws := Service{
		workingDir: workingDir,
		fsUtils:    fsUtils,
	}

	return &ws, nil
}

// Create creates the manifest directory (if it doesn't already exist) in
// the current working directory, and saves a summary in the manifest
// directory with the project name.
func (ws *Service) Create(projectName string) error {
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
func (ws *Service) Summary() (*archer.WorkspaceSummary, error) {
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

func (ws *Service) writeSummary(projectName string) error {
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

func (ws *Service) summaryPath() (string, error) {
	manifestPath, err := ws.manifestDirectoryPath()
	if err != nil {
		return "", err
	}
	workspaceSummaryPath := filepath.Join(manifestPath, workspaceSummaryFileName)
	return workspaceSummaryPath, nil
}

func (ws *Service) createManifestDirectory() error {
	// First check to see if a manifest directory already exists
	dirExists, err := ws.fsUtils.DirExists(filepath.Join(ws.workingDir, manifestDirectoryName))
	if err != nil {
		return err
	}
	// If a manifest directory doesn't exist, create it - otherwise fast succeed
	if !dirExists {
		return ws.fsUtils.Mkdir(manifestDirectoryName, 0755)
	}
	return nil
}

func (ws *Service) manifestDirectoryPath() (string, error) {
	if ws.manifestDir != "" {
		return ws.manifestDir, nil
	}
	// Are we in the manifest directory?
	inEcsDir := filepath.Base(ws.workingDir) == manifestDirectoryName
	if inEcsDir {
		ws.manifestDir = ws.workingDir
		return ws.manifestDir, nil
	}

	searchingDir := ws.workingDir
	for try := 0; try < maximumParentDirsToSearch; try++ {
		currentDirectoryPath := filepath.Join(searchingDir, manifestDirectoryName)
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
		ManifestDirectoryName: manifestDirectoryName,
		NumberOfLevelsChecked: maximumParentDirsToSearch,
	}
}

// ListManifestFiles returns a list of all the local manifests filenames.
func (ws *Service) ListManifestFiles() ([]string, error) {
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
		if !file.IsDir() && strings.HasSuffix(file.Name(), manifestFileSuffix) {
			manifestFiles = append(manifestFiles, file.Name())
		}
	}

	return manifestFiles, nil
}

// ReadManifestFile takes in a manifest file (e.g. frontend-app.yml) and returns
// the read bytes.
func (ws *Service) ReadManifestFile(manifestFile string) ([]byte, error) {
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
func (ws *Service) WriteManifest(manifestBlob []byte, applicationName string) error {
	manifestPath, err := ws.manifestDirectoryPath()
	if err != nil {
		return err
	}
	manifestFileName := fmt.Sprintf("%s%s", applicationName, manifestFileSuffix)
	return ws.fsUtils.WriteFile(filepath.Join(manifestPath, manifestFileName), manifestBlob, 0644)
}

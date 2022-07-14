// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package workspace contains functionality to manage a user's local workspace. This includes
// creating an application directory, reading and writing a summary file to associate the workspace with the application,
// and managing infrastructure-as-code files. The typical workspace will be structured like:
//  .
//  ├── copilot                        (application directory)
//  │   ├── .workspace                 (workspace summary)
//  │   ├── my-service
//  │   │   └── manifest.yml           (service manifest)
//  |   |   environments
//  |   |   └── test
//  │   │       └── manifest.yml       (environment manifest for the environment test)
//  │   ├── buildspec.yml              (legacy buildspec for the pipeline's build stage)
//  │   ├── pipeline.yml               (legacy pipeline manifest)
//  │   ├── pipelines
//  │   │   ├── pipeline-app-beta
//  │   │   │   ├── buildspec.yml      (buildspec for the pipeline 'pipeline-app-beta')
//  │   ┴   ┴   └── manifest.yml       (pipeline manifest for the pipeline 'pipeline-app-beta')
//  └── my-service-src                 (customer service code)
package workspace

import (
	"encoding"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aws/copilot-cli/internal/pkg/term/log"

	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

const (
	// CopilotDirName is the name of the directory where generated infrastructure code for an application will be stored.
	CopilotDirName = "copilot"
	// SummaryFileName is the name of the file that is associated with the application.
	SummaryFileName = ".workspace"

	addonsDirName             = "addons"
	pipelinesDirName          = "pipelines"
	environmentsDirName       = "environments"
	maximumParentDirsToSearch = 5
	legacyPipelineFileName    = "pipeline.yml"
	manifestFileName          = "manifest.yml"
	buildspecFileName         = "buildspec.yml"

	ymlFileExtension = ".yml"

	dockerfileName   = "dockerfile"
	dockerignoreName = ".dockerignore"
)

// Summary is a description of what's associated with this workspace.
type Summary struct {
	Application string `yaml:"application"` // Name of the application.

	Path string // absolute path to the summary file.
}

// Workspace typically represents a Git repository where the user has its infrastructure-as-code files as well as source files.
type Workspace struct {
	workingDirAbs string
	copilotDirAbs string
	fs            *afero.Afero
	logger        func(format string, args ...interface{})
}

// New returns a workspace, used for reading and writing to user's local workspace.
func New() (*Workspace, error) {
	fs := afero.NewOsFs()
	fsUtils := &afero.Afero{Fs: fs}
	logger := log.Infof

	workingDirAbs, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	ws := Workspace{
		workingDirAbs: workingDirAbs,
		fs:            fsUtils,
		logger:        logger,
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
			return &errHasExistingApplication{
				existingAppName: summary.Application,
				basePath:        ws.workingDirAbs,
				summaryPath:     summary.Path,
			}
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
	summaryFileExists, _ := ws.fs.Exists(summaryPath) // If an err occurs, return no applications.
	if summaryFileExists {
		value, err := ws.fs.ReadFile(summaryPath)
		if err != nil {
			return nil, err
		}
		wsSummary := Summary{
			Path: summaryPath,
		}
		return &wsSummary, yaml.Unmarshal(value, &wsSummary)
	}
	return nil, &errNoAssociatedApplication{}
}

// ListServices returns the names of the services in the workspace.
func (ws *Workspace) ListServices() ([]string, error) {
	return ws.listWorkloads(func(wlType string) bool {
		for _, t := range manifest.ServiceTypes() {
			if wlType == t {
				return true
			}
		}
		return false
	})
}

// ListJobs returns the names of all jobs in the workspace.
func (ws *Workspace) ListJobs() ([]string, error) {
	return ws.listWorkloads(func(wlType string) bool {
		for _, t := range manifest.JobTypes() {
			if wlType == t {
				return true
			}
		}
		return false
	})
}

// ListWorkloads returns the name of all the workloads in the workspace (could be unregistered in SSM).
func (ws *Workspace) ListWorkloads() ([]string, error) {
	return ws.listWorkloads(func(wlType string) bool {
		return true
	})
}

// PipelineManifest holds identifying information about a pipeline manifest file.
type PipelineManifest struct {
	Name string // Name of the pipeline inside the manifest file.
	Path string // Absolute path to the manifest file for the pipeline.
}

// ListPipelines returns all pipelines in the workspace.
func (ws *Workspace) ListPipelines() ([]PipelineManifest, error) {
	var manifests []PipelineManifest

	addManifest := func(manifestPath string) {
		manifest, err := ws.ReadPipelineManifest(manifestPath)
		switch {
		case errors.Is(err, ErrNoPipelineInWorkspace):
			// no file at manifestPath, ignore it
			return
		case err != nil:
			ws.logger("Unable to read pipeline manifest at '%s': %s\n", manifestPath, err)
			return
		}

		manifests = append(manifests, PipelineManifest{
			Name: manifest.Name,
			Path: manifestPath,
		})
	}

	// add the legacy pipeline
	legacyPath, err := ws.pipelineManifestLegacyPath()
	if err != nil {
		return nil, err
	}
	addManifest(legacyPath)

	// add each file that matches pipelinesDir/*/manifest.yml
	pipelinesDir, err := ws.pipelinesDirPath()
	if err != nil {
		return nil, err
	}

	exists, err := ws.fs.Exists(pipelinesDir)
	switch {
	case err != nil:
		return nil, fmt.Errorf("check if pipelines directory exists at %q: %w", pipelinesDir, err)
	case !exists:
		// there is at most 1 manifest (the legacy one), so we don't need to sort
		return manifests, nil
	}

	files, err := ws.fs.ReadDir(pipelinesDir)
	if err != nil {
		return nil, fmt.Errorf("read directory %q: %w", pipelinesDir, err)
	}

	for _, dir := range files {
		if dir.IsDir() {
			addManifest(filepath.Join(pipelinesDir, dir.Name(), manifestFileName))
		}
	}

	// sort manifests alphabetically by Name
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].Name < manifests[j].Name
	})

	return manifests, nil
}

// listWorkloads returns the name of all workloads (either services or jobs) in the workspace.
func (ws *Workspace) listWorkloads(match func(string) bool) ([]string, error) {
	copilotPath, err := ws.copilotDirPath()
	if err != nil {
		return nil, err
	}
	files, err := ws.fs.ReadDir(copilotPath)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", copilotPath, err)
	}
	var names []string
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		if exists, _ := ws.fs.Exists(filepath.Join(copilotPath, f.Name(), manifestFileName)); !exists {
			// Swallow the error because we don't want to include any services that we don't have permissions to read.
			continue
		}
		manifestBytes, err := ws.ReadWorkloadManifest(f.Name())
		if err != nil {
			return nil, fmt.Errorf("read manifest for workload %s: %w", f.Name(), err)
		}
		wlType, err := manifestBytes.WorkloadType()
		if err != nil {
			return nil, err
		}
		if match(wlType) {
			names = append(names, f.Name())
		}
	}
	return names, nil
}

// ListEnvironments returns the name of the environments in the workspace.
func (ws *Workspace) ListEnvironments() ([]string, error) {
	copilotPath, err := ws.copilotDirPath()
	if err != nil {
		return nil, err
	}
	envPath := filepath.Join(copilotPath, environmentsDirName)
	files, err := ws.fs.ReadDir(envPath)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", envPath, err)
	}
	var names []string
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		if exists, _ := ws.fs.Exists(filepath.Join(copilotPath, environmentsDirName, f.Name(), manifestFileName)); !exists {
			// Swallow the error because we don't want to include any environments that we don't have permissions to read.
			continue
		}
		names = append(names, f.Name())
	}
	return names, nil
}

// ReadWorkloadManifest returns the contents of the workload's manifest under copilot/{name}/manifest.yml.
func (ws *Workspace) ReadWorkloadManifest(mftDirName string) (WorkloadManifest, error) {
	raw, err := ws.read(mftDirName, manifestFileName)
	if err != nil {
		return nil, err
	}
	mft := WorkloadManifest(raw)
	if err := ws.manifestNameMatchWithDir(mft, mftDirName); err != nil {
		return nil, err
	}
	return mft, nil
}

// ReadEnvironmentManifest returns the contents of the environment's manifest under copilot/environments/{name}/manifest.yml.
func (ws *Workspace) ReadEnvironmentManifest(mftDirName string) (EnvironmentManifest, error) {
	raw, err := ws.read(environmentsDirName, mftDirName, manifestFileName)
	if err != nil {
		return nil, err
	}
	mft := EnvironmentManifest(raw)
	if err := ws.manifestNameMatchWithDir(mft, mftDirName); err != nil {
		return nil, err
	}
	typ, err := retrieveTypeFromManifest(mft)
	if err != nil {
		return nil, err
	}
	if typ != manifest.EnvironmentManifestType {
		return nil, fmt.Errorf(`manifest %s has type of "%s", not "%s"`, mftDirName, typ, manifest.EnvironmentManifestType)
	}
	return mft, nil
}

// ReadPipelineManifest returns the contents of the pipeline manifest under the given path.
func (ws *Workspace) ReadPipelineManifest(path string) (*manifest.Pipeline, error) {
	manifestExists, err := ws.fs.Exists(path)
	if err != nil {
		return nil, err
	}
	if !manifestExists {
		return nil, ErrNoPipelineInWorkspace
	}
	data, err := ws.fs.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pipeline manifest: %w", err)
	}
	pipelineManifest, err := manifest.UnmarshalPipeline(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal pipeline manifest: %w", err)
	}
	return pipelineManifest, nil
}

// WriteServiceManifest writes the service's manifest under the copilot/{name}/ directory.
func (ws *Workspace) WriteServiceManifest(marshaler encoding.BinaryMarshaler, name string) (string, error) {
	data, err := marshaler.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal service %s manifest to binary: %w", name, err)
	}
	return ws.write(data, name, manifestFileName)
}

// WriteJobManifest writes the job's manifest under the copilot/{name}/ directory.
func (ws *Workspace) WriteJobManifest(marshaler encoding.BinaryMarshaler, name string) (string, error) {
	data, err := marshaler.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal job %s manifest to binary: %w", name, err)
	}
	return ws.write(data, name, manifestFileName)
}

// WritePipelineBuildspec writes the pipeline buildspec under the copilot/pipelines/{name}/ directory.
// If successful returns the full path of the file, otherwise returns an empty string and the error.
func (ws *Workspace) WritePipelineBuildspec(marshaler encoding.BinaryMarshaler, name string) (string, error) {
	data, err := marshaler.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal pipeline buildspec to binary: %w", err)
	}
	return ws.write(data, pipelinesDirName, name, buildspecFileName)
}

// WritePipelineManifest writes the pipeline manifest under the copilot/pipelines/{name}/ directory.
// If successful returns the full path of the file, otherwise returns an empty string and the error.
func (ws *Workspace) WritePipelineManifest(marshaler encoding.BinaryMarshaler, name string) (string, error) {
	data, err := marshaler.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal pipeline manifest to binary: %w", err)
	}
	return ws.write(data, pipelinesDirName, name, manifestFileName)
}

// WriteEnvironmentManifest writes the environment manifest under the copilot/environments/{name}/ directory.
// If successful returns the full path of the file, otherwise returns an empty string and the error.
func (ws *Workspace) WriteEnvironmentManifest(marshaler encoding.BinaryMarshaler, name string) (string, error) {
	data, err := marshaler.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal environment manifest to binary: %w", err)
	}
	return ws.write(data, environmentsDirName, name, manifestFileName)
}

// DeleteWorkspaceFile removes the .workspace file under copilot/ directory.
// This will be called during app delete, we do not want to delete any other generated files.
func (ws *Workspace) DeleteWorkspaceFile() error {
	return ws.fs.Remove(filepath.Join(CopilotDirName, SummaryFileName))
}

// ReadAddonsDir returns a list of file names under a service's "addons/" directory.
func (ws *Workspace) ReadAddonsDir(svcName string) ([]string, error) {
	copilotPath, err := ws.copilotDirPath()
	if err != nil {
		return nil, err
	}

	var names []string
	files, err := ws.fs.ReadDir(filepath.Join(copilotPath, svcName, addonsDirName))
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

// FileStat wraps the os.Stat function.
type FileStat interface {
	Stat(name string) (os.FileInfo, error)
}

// IsInGitRepository returns true if the current working directory is a git repository.
func IsInGitRepository(fs FileStat) bool {
	_, err := fs.Stat(".git")
	return !os.IsNotExist(err)
}

// pipelineManifestLegacyPath returns the path to pipeline manifests before multiple pipelines (and the copilot/pipelines/ dir) were enabled.
func (ws *Workspace) pipelineManifestLegacyPath() (string, error) {
	copilotPath, err := ws.copilotDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(copilotPath, legacyPipelineFileName), nil
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
	return ws.fs.WriteFile(summaryPath, serializedWorkspaceSummary, 0644)
}

func (ws *Workspace) pipelinesDirPath() (string, error) {
	copilotPath, err := ws.copilotDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(copilotPath, pipelinesDirName), nil
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
	return ws.fs.Mkdir(CopilotDirName, 0755)
}

// Path returns the absolute path to the workspace.
func (ws *Workspace) Path() (string, error) {
	copilotDirPath, err := ws.copilotDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(copilotDirPath), nil
}

func (ws *Workspace) Rel(path string) (string, error) {
	copiDir, err := ws.copilotDirPath()
	if err != nil {
		return "", fmt.Errorf("get path to Copilot dir: %w", err)
	}
	fullPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("make path absolute: %w", err)
	}
	return filepath.Rel(filepath.Dir(copiDir), fullPath)
}

// copilotDirPath tries to find the current app's copilot directory from the workspace working directory.
func (ws *Workspace) copilotDirPath() (string, error) {
	if ws.copilotDirAbs != "" {
		return ws.copilotDirAbs, nil
	}

	// Are we in the application's copilot directory already?
	//
	// Note: This code checks for *any* directory named "copilot", but this might not work
	// correctly if we're in some subdirectory of the app that the user might have happened
	// to name "copilot". It's not clear if there's a good way to avoid that problem, though it
	// doesn't seem to be a terribly likely issue.
	if filepath.Base(ws.workingDirAbs) == CopilotDirName {
		ws.copilotDirAbs = ws.workingDirAbs
		return ws.copilotDirAbs, nil
	}

	// We might be in the application directory or in a subdirectory of the application
	// directory that contains the "copilot" directory.
	//
	// Keep on searching the parent directories for that copilot directory (though only
	// up to a finite limit, to avoid infinite recursion!)
	searchingDir := ws.workingDirAbs
	for try := 0; try < maximumParentDirsToSearch; try++ {
		currentDirectoryPath := filepath.Join(searchingDir, CopilotDirName)
		inCurrentDirPath, err := ws.fs.DirExists(currentDirectoryPath)
		if err != nil {
			return "", err
		}
		if inCurrentDirPath {
			ws.copilotDirAbs = currentDirectoryPath
			return ws.copilotDirAbs, nil
		}
		searchingDir = filepath.Dir(searchingDir)
	}
	return "", &ErrWorkspaceNotFound{
		CurrentDirectory:      ws.workingDirAbs,
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

	if err := ws.fs.MkdirAll(filepath.Dir(filename), 0755 /* -rwxr-xr-x */); err != nil {
		return "", fmt.Errorf("create directories for file %s: %w", filename, err)
	}
	exist, err := ws.fs.Exists(filename)
	if err != nil {
		return "", fmt.Errorf("check if manifest file %s exists: %w", filename, err)
	}
	if exist {
		return "", &ErrFileExists{FileName: filename}
	}
	if err := ws.fs.WriteFile(filename, data, 0644 /* -rw-r--r-- */); err != nil {
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
	filename := filepath.Join(pathElems...)
	exist, err := ws.fs.Exists(filename)
	if err != nil {
		return nil, fmt.Errorf("check if manifest file %s exists: %w", filename, err)
	}
	if !exist {
		return nil, &ErrFileNotExists{FileName: filename}
	}
	return ws.fs.ReadFile(filename)
}

// ListDockerfiles returns the list of Dockerfiles within the current
// working directory and a sub-directory level below. If an error occurs while
// reading directories, or no Dockerfiles found returns the error.
func (ws *Workspace) ListDockerfiles() ([]string, error) {
	wdFiles, err := ws.fs.ReadDir(ws.workingDirAbs)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}
	var dockerfiles = make([]string, 0)
	for _, wdFile := range wdFiles {
		// Add current file if it is a Dockerfile and not a directory; otherwise continue.
		if !wdFile.IsDir() {
			fname := wdFile.Name()
			if strings.Contains(strings.ToLower(fname), dockerfileName) && !strings.HasSuffix(strings.ToLower(fname), dockerignoreName) {
				path := filepath.Dir(fname) + "/" + fname
				dockerfiles = append(dockerfiles, path)
			}
			continue
		}

		// Add sub-directories containing a Dockerfile one level below current directory.
		subFiles, err := ws.fs.ReadDir(wdFile.Name())
		if err != nil {
			// swallow errors for unreadable directories
			continue
		}
		for _, f := range subFiles {
			// NOTE: ignore directories in sub-directories.
			if f.IsDir() {
				continue
			}
			fname := f.Name()
			if strings.Contains(strings.ToLower(fname), dockerfileName) && !strings.HasSuffix(strings.ToLower(fname), dockerignoreName) {
				path := wdFile.Name() + "/" + f.Name()
				dockerfiles = append(dockerfiles, path)
			}
		}
	}
	sort.Strings(dockerfiles)
	return dockerfiles, nil
}

func (ws *Workspace) manifestNameMatchWithDir(mft namedManifest, mftDirName string) error {
	mftName, err := mft.name()
	if err != nil {
		return err
	}
	if mftName != mftDirName {
		return fmt.Errorf("name of the manifest %q and directory %q do not match", mftName, mftDirName)
	}
	return nil
}

// WorkloadManifest represents raw local workload manifest.
type WorkloadManifest []byte

func (w WorkloadManifest) name() (string, error) {
	return retrieveNameFromManifest(w)
}

// WorkloadType returns the workload type of the manifest.
func (w WorkloadManifest) WorkloadType() (string, error) {
	return retrieveTypeFromManifest(w)
}

// EnvironmentManifest represents raw local environment manifest.
type EnvironmentManifest []byte

func (e EnvironmentManifest) name() (string, error) {
	return retrieveNameFromManifest(e)
}

type namedManifest interface {
	name() (string, error)
}

func retrieveNameFromManifest(in []byte) (string, error) {
	wl := struct {
		Name string `yaml:"name"`
	}{}
	if err := yaml.Unmarshal(in, &wl); err != nil {
		return "", fmt.Errorf(`unmarshal manifest file to retrieve "name": %w`, err)
	}
	return wl.Name, nil
}

func retrieveTypeFromManifest(in []byte) (string, error) {
	wl := struct {
		Type string `yaml:"type"`
	}{}
	if err := yaml.Unmarshal(in, &wl); err != nil {
		return "", fmt.Errorf(`unmarshal manifest file to retrieve "type": %w`, err)
	}
	return wl.Type, nil
}

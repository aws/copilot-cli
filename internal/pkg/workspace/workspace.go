// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
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
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	maximumParentDirsToSearch = 5
	pipelineFileName          = "pipeline.yml"
	manifestFileName          = "manifest.yml"
	buildspecFileName         = "buildspec.yml"

	ymlFileExtension = ".yml"

	dockerfileName = "dockerfile"
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
	summaryFileExists, _ := ws.fsUtils.Exists(summaryPath) // If an err occurs, return no applications.
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

// ListWorkloads returns the name of all the workloads in the workspace.
func (ws *Workspace) ListWorkloads() ([]string, error) {
	return ws.listWorkloads(func(wlType string) bool {
		return true
	})
}

// Pipeline holds identifying information about a pipeline.
type Pipeline struct {
	Name string
	Path string
}

// ListPipelines returns all pipelines in the workspace.
func (ws *Workspace) ListPipelines() ([]Pipeline, error) {
	var pipelines []Pipeline
	// Look for legacy pipeline.
	legacyPath, err := ws.PipelineManifestLegacyPath()
	if err != nil {
		return nil, err
	}
	manifest, err := ws.ReadPipelineManifest(legacyPath)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, Pipeline{
		Name: manifest.Name,
		Path: legacyPath,
	})
	// Look for other pipelines.
	pipelinesPath, err := ws.pipelinesDirPath()
	if err != nil {
		return nil, err
	}
	exists, err := ws.fsUtils.Exists(pipelinesPath)
	if err != nil {
		return nil, fmt.Errorf("check if pipeline manifest exists at %s: %w", pipelinesPath, err)
	}
	if exists {
		files, err := ws.fsUtils.ReadDir(pipelinesPath)
		if err != nil {
			return nil, fmt.Errorf("read directory %s: %w", pipelinesPath, err)
		}
		for _, file := range files {
			// Ignore buildspecs.
			if strings.HasSuffix(file.Name(), "buildspec.yml") {
				continue
			}
			// Read manifests of moved legacy pipeline and any other pipelines.
			if strings.HasSuffix(file.Name(), ".yml") {
				path := filepath.Join(pipelinesPath, file.Name())
				manifest, err := ws.ReadPipelineManifest(path)
				if err != nil {
					return nil, err
				}
				pipelines = append(pipelines, Pipeline{
					Name: manifest.Name,
					Path: path,
				})
			}
		}
	}
	return pipelines, nil
}

// listWorkloads returns the name of all workloads (either services or jobs) in the workspace.
func (ws *Workspace) listWorkloads(match func(string) bool) ([]string, error) {
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

// ReadWorkloadManifest returns the contents of the workload's manifest under copilot/{name}/manifest.yml.
func (ws *Workspace) ReadWorkloadManifest(mftDirName string) (WorkloadManifest, error) {
	raw, err := ws.read(mftDirName, manifestFileName)
	if err != nil {
		return nil, err
	}
	mft := WorkloadManifest(raw)
	mftName, err := mft.workloadName()
	if err != nil {
		return nil, err
	}
	if mftName != mftDirName {
		return nil, fmt.Errorf(`name of the manifest "%s" and directory "%s" do not match`, mftName, mftDirName)
	}
	return mft, nil
}

// ReadPipelineManifest returns the contents of the pipeline manifest under the given path.
func (ws *Workspace) ReadPipelineManifest(path string) (*manifest.PipelineManifest, error) {
	manifestExists, err := ws.fsUtils.Exists(path)
	if err != nil {
		return nil, err
	}
	if !manifestExists {
		return nil, ErrNoPipelineInWorkspace
	}
	data, err := ws.fsUtils.ReadFile(path)
	if err != nil {
		log.Infof("Unable to read pipeline manifest file at '%s'", path)
		return nil, nil
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
// This will be called during app delete, we do not want to delete any other generated files.
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

// FileStat wraps the os.Stat function.
type FileStat interface {
	Stat(name string) (os.FileInfo, error)
}

// IsInGitRepository returns true if the current working directory is a git repository.
func IsInGitRepository(fs FileStat) bool {
	_, err := fs.Stat(".git")
	return !os.IsNotExist(err)
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

func (ws *Workspace) PipelineManifestLegacyPath() (string, error) {
	copilotPath, err := ws.copilotDirPath()
	if err != nil {
		return "", err
	}
	pipelineManifestPath := filepath.Join(copilotPath, pipelineFileName)
	return pipelineManifestPath, nil
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
	return ws.fsUtils.Mkdir(CopilotDirName, 0755)
}

// Path returns the absolute path to the workspace.
func (ws *Workspace) Path() (string, error) {
	copilotDirPath, err := ws.copilotDirPath()
	if err != nil {
		return "", err
	}
	return filepath.Dir(copilotDirPath), nil
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
	return "", &ErrWorkspaceNotFound{
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
	filename := filepath.Join(pathElems...)
	exist, err := ws.fsUtils.Exists(filename)
	if err != nil {
		return nil, fmt.Errorf("check if manifest file %s exists: %w", filename, err)
	}
	if !exist {
		return nil, &ErrFileNotExists{FileName: filename}
	}
	return ws.fsUtils.ReadFile(filename)
}

// ListDockerfiles returns the list of Dockerfiles within the current
// working directory and a sub-directory level below. If an error occurs while
// reading directories, or no Dockerfiles found returns the error.
func (ws *Workspace) ListDockerfiles() ([]string, error) {
	wdFiles, err := ws.fsUtils.ReadDir(ws.workingDir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}
	var dockerfiles = make([]string, 0)
	for _, wdFile := range wdFiles {
		// Add current file if it is a Dockerfile and not a directory; otherwise continue.
		if !wdFile.IsDir() {
			fname := wdFile.Name()
			if strings.Contains(strings.ToLower(fname), dockerfileName) {
				path := filepath.Dir(fname) + "/" + fname
				dockerfiles = append(dockerfiles, path)
			}
			continue
		}

		// Add sub-directories containing a Dockerfile one level below current directory.
		subFiles, err := ws.fsUtils.ReadDir(wdFile.Name())
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
			if strings.Contains(strings.ToLower(fname), dockerfileName) {
				path := wdFile.Name() + "/" + f.Name()
				dockerfiles = append(dockerfiles, path)
			}
		}
	}
	sort.Strings(dockerfiles)
	return dockerfiles, nil
}

// WorkloadManifest represents raw local workload manifest.
type WorkloadManifest []byte

func (w WorkloadManifest) workloadName() (string, error) {
	wl := struct {
		Name string `yaml:"name"`
	}{}
	if err := yaml.Unmarshal(w, &wl); err != nil {
		return "", fmt.Errorf(`unmarshal manifest file to retrieve "name": %w`, err)
	}
	return wl.Name, nil
}

// WorkloadType returns the workload type of the manifest.
func (w WorkloadManifest) WorkloadType() (string, error) {
	wl := struct {
		Type string `yaml:"type"`
	}{}
	if err := yaml.Unmarshal(w, &wl); err != nil {
		return "", fmt.Errorf(`unmarshal manifest file to retrieve "type": %w`, err)
	}
	return wl.Type, nil
}

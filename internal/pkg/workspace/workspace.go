// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package workspace contains functionality to manage a user's local workspace. This includes
// creating an application directory, reading and writing a summary file to associate the workspace with the application,
// and managing infrastructure-as-code files. The typical workspace will be structured like:
//
//	.
//	├── copilot                        (application directory)
//	│   ├── .workspace                 (workspace summary)
//	│   ├── my-service
//	│   │   └── manifest.yml           (service manifest)
//	|   |   environments
//	|   |   └── test
//	│   │       └── manifest.yml       (environment manifest for the environment test)
//	│   ├── buildspec.yml              (legacy buildspec for the pipeline's build stage)
//	│   ├── pipeline.yml               (legacy pipeline manifest)
//	│   ├── pipelines
//	│   │   ├── pipeline-app-beta
//	│   │   │   ├── buildspec.yml      (buildspec for the pipeline 'pipeline-app-beta')
//	│   ┴   ┴   └── manifest.yml       (pipeline manifest for the pipeline 'pipeline-app-beta')
//	└── my-service-src                 (customer service code)
package workspace

import (
	"encoding"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
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
	// AddonsParametersFileName is the name of the file that define extra parameters for an addon.
	AddonsParametersFileName = "addons.parameters.yml"

	addonsDirName             = "addons"
	overridesDirName          = "overrides"
	pipelinesDirName          = "pipelines"
	environmentsDirName       = "environments"
	maximumParentDirsToSearch = 5
	legacyPipelineFileName    = "pipeline.yml"
	manifestFileName          = "manifest.yml"
	buildspecFileName         = "buildspec.yml"
)

// ErrTraverseUpShouldStop signals that TraverseUp should stop.
var ErrTraverseUpShouldStop = errors.New("should stop")

// TraverseUpProcessFn represents a function that TraverseUp invokes at each level of traversal.
// If TraverseUpProcessFn returns an ErrTraverseUpShouldStop, TraverseUp will stop traversing, return the result and a nil error.
// If TraverseUpProcessFn returns some other error, TraverseUp will stop traversing, return an empty string and the error.
// If TraverseUpProcessFn returns a nil error, TraverseUp will keep traversing up the directory tree.
type TraverseUpProcessFn func(dir string) (result string, err error)

// TraverseUp traverses at most `maxLevels` up from the starting directory, invoke TraverseUpProcessFn at each level,
// and returns the value that it gets TraverseUpProcessFn process upon receiving an ErrTraverseUpShouldStop signal.
// If after traversing up `maxLevels`, it still hasn't received a ErrTraverseUpShouldStop signal, it will return ErrTargetNotFound.
func TraverseUp(startDir string, maxLevels int, process TraverseUpProcessFn) (string, error) {
	searchingDir := startDir
	for try := 0; try < maxLevels; try++ {
		result, err := process(searchingDir)
		if errors.Is(err, ErrTraverseUpShouldStop) {
			return result, nil
		}
		if err != nil {
			return "", err
		}
		searchingDir = filepath.Dir(searchingDir)
	}
	return "", &ErrTargetNotFound{
		startDir:              startDir,
		numberOfLevelsChecked: maxLevels,
	}
}

// Summary is a description of what's associated with this workspace.
type Summary struct {
	Application string `yaml:"application"` // Name of the application.
	Path        string `yaml:"-"`           // Absolute path to the summary file.
}

// Workspace typically represents a Git repository where the user has its infrastructure-as-code files as well as source files.
type Workspace struct {
	workingDirAbs string
	CopilotDirAbs string // TODO: make private by adding mocks for selector unit testing.

	// These fields should be accessed via the Summary method and not directly.
	summary       *Summary
	summaryErr    error
	summarizeOnce sync.Once

	fs     *afero.Afero
	logger func(format string, args ...interface{})
}

var getWd = os.Getwd

// Use returns an existing workspace, searching for a copilot/ directory from the current wd,
// up to 5 levels above. It returns ErrWorkspaceNotFound if no copilot/ directory is found.
func Use(fs afero.Fs) (*Workspace, error) {
	workingDirAbs, err := getWd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	ws := &Workspace{
		workingDirAbs: workingDirAbs,
		fs:            &afero.Afero{Fs: fs},
		logger:        log.Infof,
	}
	copilotDirPath, err := ws.copilotDirPath()
	if err != nil {
		return nil, err
	}
	ws.CopilotDirAbs = copilotDirPath
	if _, err := ws.Summary(); err != nil {
		// If there is an issue retrieving the summary, then the workspace is not usable.
		return nil, err
	}
	return ws, nil
}

// Create creates a new Workspace in the current working directory for appName with summary if it doesn't already exist.
func Create(appName string, fs afero.Fs) (*Workspace, error) {
	workingDirAbs, err := getWd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}
	ws := &Workspace{
		workingDirAbs: workingDirAbs,
		fs:            &afero.Afero{Fs: fs},
		logger:        log.Infof,
	}

	// Check if a workspace already exists.
	copilotDirPath, err := ws.copilotDirPath()
	var errWSNotFound *ErrWorkspaceNotFound
	if err != nil && !errors.As(err, &errWSNotFound) {
		return nil, err
	}
	if err == nil {
		ws.CopilotDirAbs = copilotDirPath
		// If so, grab the summary.
		summary, err := ws.Summary()
		if err == nil {
			// If a summary exists, but is registered to a different application, throw an error.
			if summary.Application != appName {
				return nil, &errHasExistingApplication{
					existingAppName: summary.Application,
					basePath:        ws.workingDirAbs,
					summaryPath:     summary.Path,
				}
			}
			// Otherwise our work is all done.
			return ws, nil
		}
		var notFound *ErrNoAssociatedApplication
		if !errors.As(err, &notFound) {
			return nil, err
		}
	}

	// Create a workspace, including both the dir and workspace file.
	CopilotDirAbs, err := ws.createCopilotDir()
	if err != nil {
		return nil, err
	}
	ws.CopilotDirAbs = CopilotDirAbs
	ws.summary, ws.summaryErr = ws.writeSummary(appName)
	if ws.summaryErr != nil {
		return nil, err
	}

	return ws, nil
}

// ProjectRoot returns a path to the presumed root of the project, the directory that contains the copilot dir and .workspace file.
func (ws *Workspace) ProjectRoot() string {
	return filepath.Dir(ws.CopilotDirAbs)
}

// Summary returns a summary of the workspace. The method assumes that the workspace exists and the path is known.
func (ws *Workspace) Summary() (*Summary, error) {
	ws.summarizeOnce.Do(func() {
		summaryPath := filepath.Join(ws.CopilotDirAbs, SummaryFileName) // Assume `CopilotDirAbs` is always present.
		if ok, _ := ws.fs.Exists(summaryPath); !ok {
			ws.summaryErr = &ErrNoAssociatedApplication{}
			return
		}
		f, err := ws.fs.ReadFile(summaryPath)
		if err != nil {
			ws.summaryErr = err
			return
		}
		ws.summary = &Summary{
			Path: summaryPath,
		}
		ws.summaryErr = yaml.Unmarshal(f, ws.summary)
	})
	return ws.summary, ws.summaryErr
}

// WorkloadExists returns true if a workload exists in the workspace.
func (ws *Workspace) WorkloadExists(name string) (bool, error) {
	path := filepath.Join(ws.CopilotDirAbs, name, manifestFileName)
	exists, err := ws.fs.Exists(path)
	if err != nil {
		return false, fmt.Errorf("check if %s exists: %w", path, err)
	}
	return exists, nil
}

// HasEnvironments returns true if the workspace manages environments.
func (ws *Workspace) HasEnvironments() (bool, error) {
	path := filepath.Join(ws.CopilotDirAbs, environmentsDirName)
	exists, err := ws.fs.Exists(path)
	if err != nil {
		return false, fmt.Errorf("check if %s exists: %w", path, err)
	}
	return exists, nil
}

// ListServices returns the names of the services in the workspace.
func (ws *Workspace) ListServices() ([]string, error) {
	return ws.listWorkloads(func(wlType string) bool {
		for _, t := range manifestinfo.ServiceTypes() {
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
		for _, t := range manifestinfo.JobTypes() {
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
	addManifest(ws.pipelineManifestLegacyPath())

	// add each file that matches pipelinesDir/*/manifest.yml
	pipelinesDir := ws.pipelinesDirPath()

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
	files, err := ws.fs.ReadDir(ws.CopilotDirAbs)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", ws.CopilotDirAbs, err)
	}
	var names []string
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		if exists, _ := ws.fs.Exists(filepath.Join(ws.CopilotDirAbs, f.Name(), manifestFileName)); !exists {
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
	envPath := filepath.Join(ws.CopilotDirAbs, environmentsDirName)
	files, err := ws.fs.ReadDir(envPath)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", envPath, err)
	}
	var names []string
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		if exists, _ := ws.fs.Exists(filepath.Join(ws.CopilotDirAbs, environmentsDirName, f.Name(), manifestFileName)); !exists {
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
	if typ != manifest.Environmentmanifestinfo {
		return nil, fmt.Errorf(`manifest %s has type of "%s", not "%s"`, mftDirName, typ, manifest.Environmentmanifestinfo)
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
	return ws.fs.Remove(filepath.Join(ws.CopilotDirAbs, SummaryFileName))
}

// EnvAddonsAbsPath returns the absolute path for the addons/ directory of environments.
func (ws *Workspace) EnvAddonsAbsPath() string {
	return filepath.Join(ws.CopilotDirAbs, environmentsDirName, addonsDirName)
}

// EnvAddonFileAbsPath returns the absolute path of an addon file for environments.
func (ws *Workspace) EnvAddonFileAbsPath(fName string) string {
	return filepath.Join(ws.EnvAddonsAbsPath(), fName)
}

// WorkloadAddonsAbsPath returns the absolute path for the addons/ directory file path of a given workload.
func (ws *Workspace) WorkloadAddonsAbsPath(name string) string {
	return filepath.Join(ws.CopilotDirAbs, name, addonsDirName)
}

// WorkloadAddonFileAbsPath returns the absolute path of an addon file for a given workload.
func (ws *Workspace) WorkloadAddonFileAbsPath(wkldName, fName string) string {
	return filepath.Join(ws.WorkloadAddonsAbsPath(wkldName), fName)
}

// WorkloadAddonFilePath returns the path under the workspace of an addon file for a given workload.
func (ws *Workspace) WorkloadAddonFilePath(wkldName, fName string) string {
	return filepath.Join(wkldName, addonsDirName, fName)
}

// EnvAddonFilePath returns the path under the workspace of an addon file for environments.
func (ws *Workspace) EnvAddonFilePath(fName string) string {
	return filepath.Join(environmentsDirName, addonsDirName, fName)
}

// EnvOverridesPath returns the default path to the overrides/ directory for environments.
func (ws *Workspace) EnvOverridesPath() string {
	return filepath.Join(ws.CopilotDirAbs, environmentsDirName, overridesDirName)
}

// WorkloadOverridesPath returns the default path to the overrides/ directory for a given workload.
func (ws *Workspace) WorkloadOverridesPath(name string) string {
	return filepath.Join(ws.CopilotDirAbs, name, overridesDirName)
}

// PipelineOverridesPath returns the default path to the overrides/ directory for a given pipeline.
func (ws *Workspace) PipelineOverridesPath(name string) string {
	return filepath.Join(ws.CopilotDirAbs, pipelinesDirName, name, overridesDirName)
}

// ListFiles returns a list of file paths to all the files under the dir.
func (ws *Workspace) ListFiles(dirPath string) ([]string, error) {
	var names []string
	files, err := ws.fs.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		names = append(names, f.Name())
	}
	return names, nil
}

// ReadFile returns the content of a file.
// Returns ErrFileNotExists if the file does not exist.
func (ws *Workspace) ReadFile(fPath string) ([]byte, error) {
	exist, err := ws.fs.Exists(fPath)
	if err != nil {
		return nil, fmt.Errorf("check if file %s exists: %w", fPath, err)
	}
	if !exist {
		return nil, &ErrFileNotExists{FileName: fPath}
	}
	return ws.fs.ReadFile(fPath)
}

// Write writes the content under the path relative to "copilot/" directory.
// If successful returns the full path of the file, otherwise an empty string and an error.
func (ws *Workspace) Write(content encoding.BinaryMarshaler, path string) (string, error) {
	data, err := content.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal binary content: %w", err)
	}
	return ws.write(data, path)
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
func (ws *Workspace) pipelineManifestLegacyPath() string {
	return filepath.Join(ws.CopilotDirAbs, legacyPipelineFileName)
}

func (ws *Workspace) writeSummary(appName string) (*Summary, error) {
	summaryPath := ws.summaryPath()
	summary := Summary{
		Application: appName,
		Path:        summaryPath,
	}

	serializedWorkspaceSummary, err := yaml.Marshal(summary)
	if err != nil {
		return nil, err
	}
	return &summary, ws.fs.WriteFile(summaryPath, serializedWorkspaceSummary, 0644)
}

func (ws *Workspace) pipelinesDirPath() string {
	return filepath.Join(ws.CopilotDirAbs, pipelinesDirName)
}

func (ws *Workspace) summaryPath() string {
	return filepath.Join(ws.CopilotDirAbs, SummaryFileName)
}

func (ws *Workspace) createCopilotDir() (string, error) {
	// First check to see if a manifest directory already exists
	existingWorkspace, _ := ws.copilotDirPath()
	if existingWorkspace != "" {
		return existingWorkspace, nil
	}
	if err := ws.fs.Mkdir(CopilotDirName, 0755); err != nil {
		return "", err
	}
	return filepath.Join(ws.workingDirAbs, CopilotDirName), nil
}

// Path returns the absolute path to the workspace.
func (ws *Workspace) Path() string {
	return filepath.Dir(ws.CopilotDirAbs)
}

// Rel returns the relative path to path from the workspace copilot directory.
func (ws *Workspace) Rel(path string) (string, error) {
	fullPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("make path absolute: %w", err)
	}
	return filepath.Rel(filepath.Dir(ws.CopilotDirAbs), fullPath)
}

// copilotDirPath tries to find the current app's copilot directory from the workspace working directory.
func (ws *Workspace) copilotDirPath() (string, error) {
	// Are we in the application's copilot directory already?
	//
	// Note: This code checks for *any* directory named "copilot", but this might not work
	// correctly if we're in some subdirectory of the app that the user might have happened
	// to name "copilot". Our docs warn users and suggest creating another "copilot" dir closer to the wd.
	if filepath.Base(ws.workingDirAbs) == CopilotDirName {
		return ws.workingDirAbs, nil
	}

	// We might be in the application directory or in a subdirectory of the application
	// directory that contains the "copilot" directory.
	//
	// Keep on searching the parent directories for that copilot directory (though only
	// up to a finite limit, to avoid infinite recursion!)
	path, err := TraverseUp(ws.workingDirAbs, maximumParentDirsToSearch, func(dir string) (string, error) {
		path := filepath.Join(dir, CopilotDirName)
		exists, err := ws.fs.DirExists(path)
		if err != nil {
			return "", err
		}
		if exists {
			return path, ErrTraverseUpShouldStop
		}
		return "", nil
	})
	if err == nil {
		return path, nil
	}
	var targetNotFoundErr *ErrTargetNotFound
	if errors.As(err, &targetNotFoundErr) {
		return "", &ErrWorkspaceNotFound{
			ErrTargetNotFound: targetNotFoundErr,
			target:            CopilotDirName,
		}
	}
	return "", err
}

// write flushes the data to a file under the copilot directory joined by path elements.
func (ws *Workspace) write(data []byte, elem ...string) (string, error) {
	pathElems := append([]string{ws.CopilotDirAbs}, elem...)
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
	pathElems := append([]string{ws.CopilotDirAbs}, elem...)
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

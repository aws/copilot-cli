// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package archer

// WorkspaceSummary is a description of what's associated with this workspace.
type WorkspaceSummary struct {
	ProjectName string `yaml:"project"`
}

// Workspace can bootstrap a workspace with a manifest directory and workspace summary
// and it can manage manifest files.
type Workspace interface {
	ManifestIO
	Create(projectName string) error
	Summary() (*WorkspaceSummary, error)
	Apps() ([]Manifest, error)
}

// ManifestIO can read, write and list local manifest files.
type ManifestIO interface {
	WorkspaceFileReadWriter
	ListManifestFiles() ([]string, error)
	AppManifestFileName(appName string) string
	DeleteFile(name string) error
}

// WorkspaceFileReadWriter is the interface to read and write files to the project directory in the workspace.
type WorkspaceFileReadWriter interface {
	WriteFile(blob []byte, filename string) (string, error)
	ReadFile(filename string) ([]byte, error)
}

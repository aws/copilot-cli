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
}

// ManifestIO can read, write and list local manifest files.
type ManifestIO interface {
	WriteManifest(manifestBlob []byte, applicationName string) error
	ReadManifestFile(manifestFileName string) ([]byte, error)
	ListManifestFiles() ([]string, error)
}

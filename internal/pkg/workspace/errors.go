// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import "fmt"

// ErrWorkspaceNotFound means we couldn't locate a workspace root.
type ErrWorkspaceNotFound struct {
	CurrentDirectory      string
	ManifestDirectoryName string
	NumberOfLevelsChecked int
}

func (e *ErrWorkspaceNotFound) Error() string {
	return fmt.Sprintf("couldn't find a directory called %s up to %d levels up from %s",
		e.ManifestDirectoryName,
		e.NumberOfLevelsChecked,
		e.CurrentDirectory)
}

// ErrNoProjectAssociated means we couldn't locate a workspace summary file.
type ErrNoProjectAssociated struct{}

func (e *ErrNoProjectAssociated) Error() string {
	return fmt.Sprint("couldn't find a project associated with this workspace")
}

// ErrWorkspaceHasExistingProject means we tried to create a workspace for a project
// but it already belongs to another project.
type ErrWorkspaceHasExistingProject struct {
	ExistingProjectName string
}

func (e *ErrWorkspaceHasExistingProject) Error() string {
	return fmt.Sprintf("this workspace is already registered with project %s", e.ExistingProjectName)
}

// ErrFileExists means we tried to create an existing file.
type ErrFileExists struct {
	FileName string
}

func (e *ErrFileExists) Error() string {
	return fmt.Sprintf("file %s already exists", e.FileName)
}

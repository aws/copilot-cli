// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"errors"
	"fmt"
)

// ErrNoPipelineInWorkspace means there was no pipeline manifest in the workspace dir.
var ErrNoPipelineInWorkspace = errors.New("no pipeline manifest found in the workspace")

// ErrFileExists means we tried to create an existing file.
type ErrFileExists struct {
	FileName string
}

func (e *ErrFileExists) Error() string {
	return fmt.Sprintf("file %s already exists", e.FileName)
}

// ErrFileNotExists means we tried to read a non-existing file.
type ErrFileNotExists struct {
	FileName string
}

func (e *ErrFileNotExists) Error() string {
	return fmt.Sprintf("file %s does not exists", e.FileName)
}

// errWorkspaceNotFound means we couldn't locate a workspace root.
type errWorkspaceNotFound struct {
	CurrentDirectory      string
	ManifestDirectoryName string
	NumberOfLevelsChecked int
}

func (e *errWorkspaceNotFound) Error() string {
	return fmt.Sprintf("couldn't find a directory called %s up to %d levels up from %s",
		e.ManifestDirectoryName,
		e.NumberOfLevelsChecked,
		e.CurrentDirectory)
}

// errNoAssociatedApplication means we couldn't locate a workspace summary file.
type errNoAssociatedApplication struct{}

func (e *errNoAssociatedApplication) Error() string {
	return "couldn't find an application associated with this workspace"
}

// errHasExistingApplication means we tried to create a workspace that belongs to another application.
type errHasExistingApplication struct {
	existingAppName string
}

func (e *errHasExistingApplication) Error() string {
	return fmt.Sprintf("this workspace is already registered with application %s", e.existingAppName)
}

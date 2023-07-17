// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package workspace

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/aws/copilot-cli/internal/pkg/term/color"
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

// ErrTargetNotFound means that we couldn't locate the target file or the target directory.
type ErrTargetNotFound struct {
	startDir              string
	numberOfLevelsChecked int
}

func (e *ErrTargetNotFound) Error() string {
	return fmt.Sprintf("couldn't find a target up to %d levels up from %s",
		e.numberOfLevelsChecked,
		e.startDir)
}

// ErrWorkspaceNotFound means we couldn't locate a workspace root.
type ErrWorkspaceNotFound struct {
	*ErrTargetNotFound
	target string
}

func (e *ErrWorkspaceNotFound) Error() string {
	return fmt.Sprintf("couldn't find a directory called %s up to %d levels up from %s",
		e.target,
		e.numberOfLevelsChecked,
		e.startDir)
}

// RecommendActions suggests steps clients can take to mitigate the copilot/ directory not found error.
func (_ *ErrWorkspaceNotFound) RecommendActions() string {
	return fmt.Sprintf("Run %s to create an application.", color.HighlightCode("copilot app init"))
}

// empty denotes that this error represents an empty workspace.
func (_ *ErrWorkspaceNotFound) empty() {}

// ErrNoAssociatedApplication means we couldn't locate a workspace summary file.
type ErrNoAssociatedApplication struct{}

func (e *ErrNoAssociatedApplication) Error() string {
	return "couldn't find an application associated with this workspace"
}

// RecommendActions suggests steps clients can take to mitigate the .workspace file not found error.
func (_ *ErrNoAssociatedApplication) RecommendActions() string {
	return fmt.Sprintf(`The "copilot" directory is not associated with an application.
Run %s to create or use an application.`, color.HighlightCode("copilot app init"))
}

// empty denotes that this error represents an empty workspace.
func (_ *ErrNoAssociatedApplication) empty() {}

// errHasExistingApplication means we tried to create a workspace that belongs to another application.
type errHasExistingApplication struct {
	existingAppName string
	basePath        string
	summaryPath     string
}

func (e *errHasExistingApplication) Error() string {
	relPath, _ := filepath.Rel(e.basePath, e.summaryPath)
	if relPath == "" {
		relPath = e.summaryPath
	}
	return fmt.Sprintf("workspace is already registered with application %s under %s", e.existingAppName, relPath)
}

// IsEmptyErr returns true if the error is related to an empty workspace.
func IsEmptyErr(err error) bool {
	var emptyWs interface {
		empty()
	}
	return errors.As(err, &emptyWs)
}

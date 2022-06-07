// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package ecs

import "fmt"

type ErrMultipleContainersInTaskDef struct {
	taskDefIdentifier string
}

func (e *ErrMultipleContainersInTaskDef) Error() string {
	return fmt.Sprintf("found more than one container in task definition: %s", e.taskDefIdentifier)
}

type ErrExitCode struct {
	containerName string
	taskId        string
	exitCode      int
}

func (e *ErrExitCode) Error() string {
	return fmt.Sprintf("Container %s in task %s exited with status code %d", e.containerName, e.taskId, e.exitCode)
}

func (e *ErrExitCode) ExitCode() int {
	return e.exitCode
}

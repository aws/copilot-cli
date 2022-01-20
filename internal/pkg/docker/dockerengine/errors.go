// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockerengine

import (
	"fmt"
)

// ErrContainerCommandNotFound means the containerRuntime command is not found.
type ErrContainerCommandNotFound struct {
	containerRuntime string
}

func (e ErrContainerCommandNotFound) Error() string {
	return fmt.Sprintf("%v: command not found", e.containerRuntime)
}

// ErrDockerDaemonNotResponsive means the docker daemon is not responsive.
type ErrDockerDaemonNotResponsive struct {
	msg string
}

func (e ErrDockerDaemonNotResponsive) Error() string {
	return fmt.Sprintf("docker daemon is not responsive: %s", e.msg)
}

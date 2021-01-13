// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"fmt"
)

// ErrInvalidPort means that while there was a port provided, it was out of bounds or unparseable
type ErrInvalidPort struct {
	Match string
}

func (e ErrInvalidPort) Error() string {
	return fmt.Sprintf("port represented at %s is invalid or unparseable", e.Match)
}

// ErrNoExpose means there were no documented EXPOSE statements in the given dockerfile.
type ErrNoExpose struct {
	Dockerfile string
}

func (e ErrNoExpose) Error() string {
	return fmt.Sprintf("no EXPOSE statements in Dockerfile %s", e.Dockerfile)
}

// ErrDockerCommandNotFound means the docker command is not found.
type ErrDockerCommandNotFound struct{}

func (e ErrDockerCommandNotFound) Error() string {
	return "docker: command not found"
}

// ErrDockerDaemonNotResponsive means the docker daemon is not responsive.
type ErrDockerDaemonNotResponsive struct {
	msg string
}

func (e ErrDockerDaemonNotResponsive) Error() string {
	return fmt.Sprintf("docker daemon is not responsive: %s", e.msg)
}

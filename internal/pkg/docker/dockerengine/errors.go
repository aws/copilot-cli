// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package dockerengine

import (
	"errors"
	"fmt"
)

// ErrDockerCommandNotFound means the docker command is not found.
var ErrDockerCommandNotFound = errors.New("docker: command not found")

// ErrDockerDaemonNotResponsive means the docker daemon is not responsive.
type ErrDockerDaemonNotResponsive struct {
	msg string
}

func (e ErrDockerDaemonNotResponsive) Error() string {
	return fmt.Sprintf("docker daemon is not responsive: %s", e.msg)
}

type errEmptyImageTags struct {
	uri string
}

func (e *errEmptyImageTags) Error() string {
	return fmt.Sprintf("tags to reference an image should not be empty for building and pushing into the ECR repository %s", e.uri)
}

// ErrContainerNotExited represents an error when a Docker container has not exited.
type ErrContainerNotExited struct {
	name string
}

// Error returns the error message.
func (e *ErrContainerNotExited) Error() string {
	return fmt.Sprintf("container %q has not exited", e.name)
}

// ErrContainerExited represents an error when a Docker container has exited.
// It includes the container name and exit code in the error message.
type ErrContainerExited struct {
	name     string
	exitcode int
}

// ExitCode returns the OS exit code configured for this error.
func (e *ErrContainerExited) ExitCode() int {
	return e.exitcode
}

// ErrContainerExited represents docker container exited with an exitcode.
func (e *ErrContainerExited) Error() string {
	return fmt.Sprintf("container %q exited with code %d", e.name, e.exitcode)
}

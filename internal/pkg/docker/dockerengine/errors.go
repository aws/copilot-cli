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

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package exec

import (
	"os/exec"
	"syscall"
)

// NewProcessGroup causes the command to be assigned its own
// process group, and not use the parent's (this command) pid.
func NewProcessGroup() CmdOption {
	return func(c *exec.Cmd) {
		c.SysProcAttr = &syscall.SysProcAttr{
			CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		}
	}
}

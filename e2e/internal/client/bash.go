// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	cmd "github.com/aws/copilot-cli/e2e/internal/command"
)

// BashExec execute bash commands.
func BashExec(command string, opts ...cmd.Option) error {
	args := []string{"-c", command}
	err := cmd.Run("bash", args, opts...)
	if err != nil {
		return err
	}
	return nil
}

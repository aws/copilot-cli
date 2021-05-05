// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package exec provides an interface to execute certain commands.
package exec

import (
	"bytes"
)

// ValidateBinary validates if the ssm plugin exists.
func (s SSMPluginCommand) ValidateBinary() error {
	// Hinder output on the screen.
	var b bytes.Buffer
	return s.runner.Run(ssmPluginBinaryName, []string{}, Stdout(&b))
}

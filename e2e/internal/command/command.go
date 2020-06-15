// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"io"
	"os"
	"os/exec"
)

// Option is the function signature for customizing the internal *exec.Cmd.
type Option func(cmd *exec.Cmd)

// Stdin sets the internal *exec.Cmd's Stdin field.
func Stdin(r io.Reader) Option {
	return func(c *exec.Cmd) {
		c.Stdin = r
	}
}

// Stdout sets the internal *exec.Cmd's Stdout field.
func Stdout(writer io.Writer) Option {
	return func(c *exec.Cmd) {
		c.Stdout = writer
	}
}

// Run runs the input command with input args with Stdout and Stderr defaulted to os.Stderr.
// Input options will override these defaults.
func Run(name string, args []string, options ...Option) error {
	cmd := exec.Command(name, args...)

	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	for _, opt := range options {
		opt(cmd)
	}

	return cmd.Run()
}

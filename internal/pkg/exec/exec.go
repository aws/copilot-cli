// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package exec provides an interface to execute certain commands.
package exec

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"
)

type httpClient interface {
	Get(url string) (resp *http.Response, err error)
}

type runner interface {
	Run(name string, args []string, options ...CmdOption) error
	InteractiveRun(name string, args []string) error
}

type cmdRunner interface {
	Run() error
}

// Cmd runs external commands, it wraps the exec.CommandContext function from the stdlib so that
// running external commands can be unit tested.
type Cmd struct {
	command func(ctx context.Context, name string, args []string, opts ...CmdOption) cmdRunner
}

// CmdOption is a type alias to configure a command.
type CmdOption func(cmd *exec.Cmd)

// NewCmd returns a Cmd that can run external commands.
// By default the output of the commands is piped to stderr.
func NewCmd() *Cmd {
	return &Cmd{
		command: func(ctx context.Context, name string, args []string, opts ...CmdOption) cmdRunner {
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Stdout = os.Stderr
			cmd.Stderr = os.Stderr
			for _, opt := range opts {
				opt(cmd)
			}
			return cmd
		},
	}
}

// Stdin sets the internal *exec.Cmd's Stdin field.
func Stdin(r io.Reader) CmdOption {
	return func(c *exec.Cmd) {
		c.Stdin = r
	}
}

// Stdout sets the internal *exec.Cmd's Stdout field.
func Stdout(writer io.Writer) CmdOption {
	return func(c *exec.Cmd) {
		c.Stdout = writer
	}
}

// Stderr sets the internal *exec.Cmd's Stderr field.
func Stderr(writer io.Writer) CmdOption {
	return func(c *exec.Cmd) {
		c.Stderr = writer
	}
}

// Run starts the named command and waits until it finishes.
func (c *Cmd) Run(name string, args []string, opts ...CmdOption) error {
	cmd := c.command(context.Background(), name, args, opts...)
	return cmd.Run()
}

// RunWithContext starts the named command with the given context.
// Command execution process will be killed if the context becomes done before the command completes on its own.
func (c *Cmd) RunWithContext(ctx context.Context, name string, args []string, opts ...CmdOption) error {
	cmd := c.command(ctx, name, args, opts...)
	return cmd.Run()
}

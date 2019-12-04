// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"os"
	"os/exec"
	"strings"
)

type Service struct{}

func New() Service {
	return Service{}
}

type Option func(cmd *exec.Cmd)

func WithStandardInput(input string) Option {
	return func(c *exec.Cmd) {
		c.Stdin = strings.NewReader(input)
	}
}

func (s Service) Run(name string, args []string, options ...Option) ([]byte, error) {
	cmd := exec.Command(name, args...)

	// NOTE: Stdout and Stderr must both be set otherwise command output pipes to os.DevNull
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	for _, opt := range options {
		opt(cmd)
	}

	return cmd.Output()
}

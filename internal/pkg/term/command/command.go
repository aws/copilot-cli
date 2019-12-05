// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"io"
	"os"
	"os/exec"
	"strings"
)

type Service struct{}

func New() Service {
	return Service{}
}

type Option func(cmd *exec.Cmd)

func Stdin(input string) Option {
	return func(c *exec.Cmd) {
		c.Stdin = strings.NewReader(input)
	}
}

func Stdout(writer io.Writer) Option {
	return func(c *exec.Cmd) {
		c.Stdout = writer
	}
}

func (s Service) Run(name string, args []string, options ...Option) error {
	cmd := exec.Command(name, args...)

	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	for _, opt := range options {
		opt(cmd)
	}

	return cmd.Run()
}

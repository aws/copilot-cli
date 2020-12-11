// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package exec provides an interface to execute certain commands.
package exec

import (
	"github.com/aws/copilot-cli/internal/pkg/term/command"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
)

type runner interface {
	Run(name string, args []string, options ...command.Option) error
}

type prompter interface {
	Confirm(message, help string, promptOpts ...prompt.Option) (bool, error)
}

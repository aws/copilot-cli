// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package exec provides an interface to execute certain commands.
package exec

import (
	"github.com/aws/copilot-cli/internal/pkg/term/command"
)

type runner interface {
	Run(name string, args []string, options ...command.Option) error
	InteractiveRun(name string, args []string) error
}

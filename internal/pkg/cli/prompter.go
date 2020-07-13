// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import "github.com/aws/copilot-cli/internal/pkg/term/prompt"

type prompter interface {
	Get(message, help string, validator prompt.ValidatorFunc, promptOpts ...prompt.Option) (string, error)
	GetSecret(message, help string, promptOpts ...prompt.Option) (string, error)
	SelectOne(message, help string, options []string, promptOpts ...prompt.Option) (string, error)
	MultiSelect(message, help string, options []string, promptOpts ...prompt.Option) ([]string, error)
	Confirm(message, help string, promptOpts ...prompt.Option) (bool, error)
}

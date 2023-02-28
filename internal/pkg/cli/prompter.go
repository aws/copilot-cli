// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import "github.com/aws/copilot-cli/internal/pkg/term/prompt"

type prompter interface {
	Get(message, help string, validator prompt.ValidatorFunc, promptOpts ...prompt.PromptConfig) (string, error)
	GetSecret(message, help string, promptOpts ...prompt.PromptConfig) (string, error)
	SelectOne(message, help string, options []string, promptOpts ...prompt.PromptConfig) (string, error)
	MultiSelect(message, help string, options []string, validator prompt.ValidatorFunc, promptOpts ...prompt.PromptConfig) ([]string, error)
	Confirm(message, help string, promptOpts ...prompt.PromptConfig) (bool, error)
	SelectOption(message, help string, opts []prompt.Option, promptCfgs ...prompt.PromptConfig) (value string, err error)
	MultiSelectOptions(message, help string, opts []prompt.Option, promptCfgs ...prompt.PromptConfig) ([]string, error)
}

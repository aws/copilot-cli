// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

// SelectOne prompts the user with a list of options to choose from with the arrow keys.
func (p Prompt) SelectOne(message, help string, options []string, promptCfgs ...PromptConfig) (string, error) {
	if len(options) <= 0 {
		return "", ErrEmptyOptions
	}

	sel := &survey.Select{
		Message: message,
		Options: options,
		Default: options[0],
	}
	if help != "" {
		sel.Help = color.Help(help)
	}

	prompt := &prompt{
		prompter: sel,
	}
	for _, cfg := range promptCfgs {
		cfg(prompt)
	}

	var result string
	err := p(prompt, &result, stdio(), icons())
	return result, err
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
)

// Configuration while spacing text with a tabwriter.
const (
	minCellWidth           = 20  // minimum number of characters in a table's cell.
	tabWidth               = 4   // number of characters in between columns.
	cellPaddingWidth       = 2   // number of padding characters added by default to a cell.
	paddingChar            = ' ' // character in between columns.
	noAdditionalFormatting = 0
)

// Option represents a choice with a hint for clarification.
type Option struct {
	Value string
	Hint  string
}

// String implements the fmt.Stringer interface.
func (o Option) String() string {
	if o.Hint == "" {
		return fmt.Sprintf("%s\t", o.Value)
	}
	return fmt.Sprintf("%s\t%s", o.Value, color.Faint.Sprintf("(%s)", o.Hint))
}

// SelectOption prompts the user to select one option from options and returns the Value of the option.
func (p Prompt) SelectOption(message string, opts []Option, promptCfgs ...PromptConfig) (value string, err error) {
	if len(opts) <= 0 {
		return "", ErrEmptyOptions
	}

	choices, err := stringifyOptions(opts)
	if err != nil {
		return "", err
	}
	result, err := p.SelectOne(message, "", choices, promptCfgs...)
	if err != nil {
		return "", err
	}
	return parseValueFromOptionFmt(result), nil
}

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

func stringifyOptions(opts []Option) ([]string, error) {
	buf := new(strings.Builder)
	tw := tabwriter.NewWriter(buf, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	var lines []string
	for _, opt := range opts {
		lines = append(lines, opt.String())
	}
	if _, err := tw.Write([]byte(strings.Join(lines, "\n"))); err != nil {
		return nil, fmt.Errorf("render options: %v", err)
	}
	if err := tw.Flush(); err != nil {
		return nil, fmt.Errorf("flush tabwriter options: %v", err)
	}
	return strings.Split(buf.String(), "\n"), nil
}

func parseValueFromOptionFmt(formatted string) string {
	if idx := strings.Index(formatted, "("); idx != -1 {
		return strings.TrimSpace(formatted[:idx])
	}
	return strings.TrimSpace(formatted)
}

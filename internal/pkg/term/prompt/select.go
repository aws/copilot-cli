// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"fmt"
	"regexp"
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

// SGR(Select Graphic Rendition) is used to colorize outputs in terminals.
// Example matches:
// '\x1b[2m'        (for Faint output)
// '\x1b[1;31m'     (for bold, red output)
const (
	sgrStart     = "\x1b\\["    // SGR sequences start with "ESC[".
	sgrEnd       = "m"          // SGR sequences end with "m".
	sgrParameter = "[0-9]{1,3}" // SGR parameter values range from 0 to 107.
)

var (
	sgrParameterGroups = fmt.Sprintf("%s(;%s)*", sgrParameter, sgrParameter)            // One or more SGR parameters separated by ";"
	sgr                = fmt.Sprintf("%s(%s)?%s", sgrStart, sgrParameterGroups, sgrEnd) // A complete SGR sequence: \x1b\\[([0-9]{1,2,3}(;[0-9]{1,2,3})*)?m
)
var regexpSGR = regexp.MustCompile(sgr)

// Option represents a choice with a hint for clarification.
type Option struct {
	Value        string // The actual value represented by the option.
	FriendlyText string // An optional FriendlyText displayed in place of Value.
	Hint         string // An optional Hint displayed alongside the Value or FriendlyText.
}

// String implements the fmt.Stringer interface.
func (o Option) String() string {
	text := o.Value
	if o.FriendlyText != "" {
		text = o.FriendlyText
	}
	if o.Hint == "" {
		return fmt.Sprintf("%s\t", text)
	}
	return fmt.Sprintf("%s\t%s", text, color.Faint.Sprintf("(%s)", o.Hint))
}

// SelectOption prompts the user to select one option from options and returns the Value of the option.
func (p Prompt) SelectOption(message, help string, opts []Option, promptCfgs ...PromptConfig) (value string, err error) {
	if len(opts) <= 0 {
		return "", ErrEmptyOptions
	}

	prettified, err := prettifyOptions(opts)
	if err != nil {
		return "", err
	}
	result, err := p.SelectOne(message, help, prettified.choices, promptCfgs...)
	if err != nil {
		return "", err
	}
	return prettified.choice2Value[result], nil
}

// MultiSelectOptions prompts the user to select multiple options and returns the value field from the options.
func (p Prompt) MultiSelectOptions(message, help string, opts []Option, promptCfgs ...PromptConfig) ([]string, error) {
	if len(opts) <= 0 {
		return nil, ErrEmptyOptions
	}

	prettified, err := prettifyOptions(opts)
	if err != nil {
		return nil, err
	}
	choices, err := p.MultiSelect(message, help, prettified.choices, nil, promptCfgs...)
	if err != nil {
		return nil, err
	}
	values := make([]string, len(choices))
	for i, choice := range choices {
		values[i] = prettified.choice2Value[choice]
	}
	return values, nil
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

// MultiSelect prompts the user with a list of options to choose from with the arrow keys and enter key.
func (p Prompt) MultiSelect(message, help string, options []string, validator ValidatorFunc, promptCfgs ...PromptConfig) ([]string, error) {
	if len(options) <= 0 {
		// returns nil slice if error
		return nil, ErrEmptyOptions
	}
	multiselect := &survey.MultiSelect{
		Message: message,
		Options: options,
		Default: options[0],
	}
	if help != "" {
		multiselect.Help = color.Help(help)
	}

	prompt := &prompt{
		prompter: multiselect,
	}
	for _, cfg := range promptCfgs {
		cfg(prompt)
	}

	var result []string
	var err error
	if validator == nil {
		err = p(prompt, &result, stdio(), icons())
	} else {
		err = p(prompt, &result, stdio(), validators(validator), icons())
	}
	return result, err
}

type prettyOptions struct {
	choices      []string
	choice2Value map[string]string
}

func prettifyOptions(opts []Option) (prettyOptions, error) {
	buf := new(strings.Builder)
	tw := tabwriter.NewWriter(buf, minCellWidth, tabWidth, cellPaddingWidth, paddingChar, noAdditionalFormatting)
	var lines []string
	for _, opt := range opts {
		lines = append(lines, opt.String())
	}
	if _, err := tw.Write([]byte(strings.Join(lines, "\n"))); err != nil {
		return prettyOptions{}, fmt.Errorf("render options: %v", err)
	}
	if err := tw.Flush(); err != nil {
		return prettyOptions{}, fmt.Errorf("flush tabwriter options: %v", err)
	}
	choices := strings.Split(buf.String(), "\n")
	choice2Value := make(map[string]string)
	for idx, choice := range choices {
		choice2Value[choice] = opts[idx].Value
	}
	return prettyOptions{
		choices:      choices,
		choice2Value: choice2Value,
	}, nil
}

func parseValueFromOptionFmt(formatted string) string {
	if idx := strings.Index(formatted, "("); idx != -1 {
		s := regexpSGR.ReplaceAllString(formatted[:idx], "")
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(formatted)
}

func parseValuesFromOptions(formatted string) string {
	options := strings.Split(formatted, ", ")
	out := make([]string, len(options))
	for i, option := range options {
		out[i] = parseValueFromOptionFmt(option)
	}
	return strings.Join(out, ", ")
}

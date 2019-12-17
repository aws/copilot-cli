// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package prompt provides functionality to retrieve free-form text, selection,
// and confirmation input from the user via a terminal.
package prompt

import (
	"errors"
	"os"

	"github.com/AlecAivazis/survey/v2"
)

func init() {
	survey.ConfirmQuestionTemplate = `
{{ if .ShowHelp }}{{- color .Config.Icons.Help.Format }}{{ .Config.Icons.Help.Text }} {{ .Help }}{{color "reset"}}{{"\n"}}{{end}}
{{- color .Config.Icons.Question.Format }}{{ .Config.Icons.Question.Text }} {{color "reset"}}
{{- color "default"}}{{ .Message }} {{color "reset"}}
{{- if .Answer}}
  {{- color "cyan"}}{{.Answer}}{{color "reset"}}{{"\n"}}
{{- else }}
  {{- if and .Help (not .ShowHelp)}}{{color "white"}}[{{ .Config.HelpInput }} for help]{{color "reset"}} {{end}}
  {{- color "cyan"}}{{if .Default}}(Y/n) {{else}}(y/N) {{end}}{{color "reset"}}
{{- end}}`

	survey.SelectQuestionTemplate = `
{{ if .ShowHelp }}{{- color .Config.Icons.Help.Format }}{{ .Config.Icons.Help.Text }} {{ .Help }}{{color "reset"}}{{"\n"}}{{end}}
{{- color .Config.Icons.Question.Format }}{{ .Config.Icons.Question.Text }} {{color "reset"}}
{{- color "default"}}{{ .Message }}{{ .FilterMessage }}{{color "reset"}}
{{- if .ShowAnswer}}{{color "cyan"}} {{.Answer}}{{color "reset"}}{{"\n"}}
{{- else}}
  {{- "  "}}{{- color "white"}}[Use arrows to move, type to filter{{- if and .Help (not .ShowHelp)}}, {{ .Config.HelpInput }} for more help{{end}}]{{color "reset"}}
  {{- "\n"}}
  {{- range $ix, $choice := .PageEntries}}
    {{- if eq $ix $.SelectedIndex }}{{color "cyan+b" }}{{ $.Config.Icons.SelectFocus.Text }} {{else}}{{color "default"}}  {{end}}
    {{- $choice.Value}}
    {{- color "reset"}}{{"\n"}}
  {{- end}}
{{- end}}`

	survey.InputQuestionTemplate = `
{{ if .ShowHelp }}{{- color .Config.Icons.Help.Format }}{{ .Config.Icons.Help.Text }} {{ .Help }}{{color "reset"}}{{"\n"}}{{end}}
{{- color .Config.Icons.Question.Format }}{{ .Config.Icons.Question.Text }} {{color "reset"}}
{{- color "default"}}{{ .Message }} {{color "reset"}}
{{- if .ShowAnswer}}
  {{- color "cyan"}}{{.Answer}}{{color "reset"}}{{"\n"}}
{{- else }}
  {{- if and .Help (not .ShowHelp)}}{{color "white"}}[{{ print .Config.HelpInput }} for help]{{color "reset"}} {{end}}
  {{- if .Default}}{{color "cyan"}}({{.Default}}) {{color "reset"}}{{end}}
{{- end}}`

	survey.PasswordQuestionTemplate = `
{{ if .ShowHelp }}{{- color .Config.Icons.Help.Format }}{{ .Config.Icons.Help.Text }} {{ .Help }}{{color "reset"}}{{"\n"}}{{end}}
{{- color .Config.Icons.Question.Format }}{{ .Config.Icons.Question.Text }} {{color "reset"}}
{{- color "default"}}{{ .Message }} {{color "reset"}}
{{- if and .Help (not .ShowHelp)}}{{color "white"}}[{{ .Config.HelpInput }} for help]{{color "reset"}} {{end}}`
}

// ErrEmptyOptions indicates the input options list was empty.
var ErrEmptyOptions = errors.New("list of provided options is empty")

// Prompt abstracts the survey.Askone function.
type Prompt func(survey.Prompt, interface{}, ...survey.AskOpt) error

// ValidatorFunc defines the function signature for validating inputs.
type ValidatorFunc func(interface{}) error

// New returns an Prompt with default configuration.
func New() Prompt {
	return survey.AskOne
}

// Get prompts the user for free-form text input.
func (p Prompt) Get(message, help string, validator ValidatorFunc, opts ...GetOption) (string, error) {
	prompt := &survey.Input{
		Message: message,
		Help:    help,
	}
	for _, opt := range opts {
		opt(prompt)
	}

	var result string
	err := p(prompt, &result, stdio(), validators(validator), icons())
	return result, err
}

// GetOption is a functional option to modify the Get prompt.
type GetOption func(*survey.Input)

// WithDefaultInput sets a default message for the input.
func WithDefaultInput(s string) GetOption {
	return func(input *survey.Input) {
		input.Default = s
	}
}

// GetSecret prompts the user for sensitive input. Wraps survey.Password
func (p Prompt) GetSecret(message, help string) (string, error) {
	prompt := &survey.Password{
		Message: message,
		Help:    help,
	}

	var result string

	err := p(prompt, &result, stdio(), icons())

	return result, err
}

// SelectOne prompts the user with a list of options to choose from with the arrow keys.
func (p Prompt) SelectOne(message, help string, options []string) (string, error) {
	if len(options) <= 0 {
		return "", ErrEmptyOptions
	}

	prompt := &survey.Select{
		Message: message,
		Help:    help,
		Options: options,
		// TODO: we can expose this if we want to enable consumers to set an explicit default.
		Default: options[0],
	}

	var result string

	err := p(prompt, &result, stdio(), icons())

	return result, err
}

type ConfirmOption func(*survey.Confirm)

// Confirm prompts the user with a yes/no option.
func (p Prompt) Confirm(message, help string, opts ...ConfirmOption) (bool, error) {
	prompt := &survey.Confirm{
		Message: message,
		Help:    help,
	}
	for _, option := range opts {
		option(prompt)
	}

	var result bool

	err := p(prompt, &result, stdio(), icons())

	return result, err
}

func WithTrueDefault() ConfirmOption {
	return func(confirm *survey.Confirm) {
		confirm.Default = true
	}
}

func stdio() survey.AskOpt {
	return survey.WithStdio(os.Stdin, os.Stderr, os.Stderr)
}

func icons() survey.AskOpt {
	return survey.WithIcons(func(icons *survey.IconSet) {
		// The question mark "?" icon to denote a prompt will be colored in bold cyan.
		icons.Question.Format = "cyan+b"
		// Help text shown when user presses "?" will have the inverse color "i" and be surrounded by a background box ":default" of default text color.
		// For example, if your terminal background is white with black text, the help text will have a black background with white text.
		icons.Help.Format = "default+i:default"
	})
}

func validators(validatorFunc ValidatorFunc) survey.AskOpt {
	var v survey.Validator

	if validatorFunc != nil {
		v = survey.ComposeValidators(survey.Required, survey.Validator(validatorFunc))
	} else {
		v = survey.Required
	}

	return survey.WithValidator(v)
}

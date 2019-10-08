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
func (p Prompt) Get(message, help string, validator ValidatorFunc) (string, error) {
	prompt := &survey.Input{
		Message: message,
		Help:    help,
	}

	var result string

	err := p(prompt, &result, stdio(), validators(validator), icons())

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
		icons.Question.Format = "cyan"
		icons.Help.Format = "white"
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

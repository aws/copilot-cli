// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"fmt"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/require"
)

func TestPrompt_Get(t *testing.T) {
	mockError := fmt.Errorf("error")
	mockInput := "yes"
	mockDefaultInput := "yes"
	mockMessage := "mockMessage"
	mockHelpMessage := "mockHelpMessage"
	mockFinalMessage := "mockFinalMessage"

	testCases := map[string]struct {
		inValidator ValidatorFunc
		inPrompt    Prompt

		wantValue string
		wantError error
	}{
		"should return users input": {
			inValidator: func(ans interface{}) error {
				return nil
			},
			inPrompt: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				internalPrompt, ok := p.(*prompt)
				require.True(t, ok, "input prompt should be type *prompt")
				require.Equal(t, mockFinalMessage, internalPrompt.FinalMessage)
				input, ok := internalPrompt.prompter.(*survey.Input)
				require.True(t, ok, "internal prompt should be type *survey.Input")
				require.Equal(t, mockMessage, input.Message)
				require.Equal(t, mockHelpMessage, input.Help)
				require.Equal(t, mockDefaultInput, input.Default)

				result, ok := out.(*string)

				require.True(t, ok, "type to write user input to should be a string")

				*result = mockInput

				require.Equal(t, 3, len(opts))

				return nil
			},
			wantValue: mockInput,
			wantError: nil,
		},
		"should echo error": {
			inPrompt: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				return mockError
			},
			wantError: mockError,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotValue, gotError := tc.inPrompt.Get(mockMessage, mockHelpMessage, tc.inValidator,
				WithDefaultInput(mockDefaultInput), WithFinalMessage(mockFinalMessage))

			require.Equal(t, tc.wantValue, gotValue)
			require.Equal(t, tc.wantError, gotError)
		})
	}
}

func TestPrompt_GetSecret(t *testing.T) {
	mockError := fmt.Errorf("error")
	mockMessage := "What's your super secret password?"
	mockSecret := "password"

	testCases := map[string]struct {
		inPrompt Prompt

		wantValue string
		wantError error
	}{
		"should return true": {
			inPrompt: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				internalPrompt, ok := p.(*prompt)
				require.True(t, ok, "input prompt should be type *prompt")
				require.Empty(t, internalPrompt.FinalMessage)

				passwd, ok := internalPrompt.prompter.(*passwordPrompt)
				require.True(t, ok, "internal prompt should be type *passwordPrompt")
				require.Equal(t, mockMessage, passwd.Message)
				require.Empty(t, passwd.Help)

				result, ok := out.(*string)

				require.True(t, ok, "type to write user input to should be a string")

				*result = mockSecret

				require.Equal(t, 2, len(opts))

				return nil
			},
			wantValue: mockSecret,
			wantError: nil,
		},
		"should echo error": {
			inPrompt: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				return mockError
			},
			wantError: mockError,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotValue, gotError := tc.inPrompt.GetSecret(mockMessage, "")

			require.Equal(t, tc.wantValue, gotValue)
			require.Equal(t, tc.wantError, gotError)
		})
	}
}

func TestPrompt_SelectOne(t *testing.T) {
	mockError := fmt.Errorf("error")
	mockMessage := "Which droid is best droid?"

	testCases := map[string]struct {
		inPrompt Prompt
		inOpts   []string

		wantValue string
		wantError error
	}{
		"should return users input": {
			inPrompt: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				internalPrompt, ok := p.(*prompt)
				require.True(t, ok, "input prompt should be type *prompt")
				require.Empty(t, internalPrompt.FinalMessage)

				sel, ok := internalPrompt.prompter.(*survey.Select)
				require.True(t, ok, "internal prompt should be type *survey.Select")
				require.Equal(t, mockMessage, sel.Message)
				require.Empty(t, sel.Help)
				require.NotEmpty(t, sel.Options)

				result, ok := out.(*string)

				require.True(t, ok, "type to write user input to should be a string")

				*result = sel.Options[0]

				require.Equal(t, 2, len(opts))

				return nil
			},
			inOpts:    []string{"r2d2", "c3po", "bb8"},
			wantValue: "r2d2",
			wantError: nil,
		},
		"should echo error": {
			inPrompt: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				return mockError
			},
			inOpts:    []string{"apple", "orange", "banana"},
			wantError: mockError,
		},
		"should return error if input options list is empty": {
			inOpts:    []string{},
			wantError: ErrEmptyOptions,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotValue, gotError := tc.inPrompt.SelectOne(mockMessage, "", tc.inOpts)

			require.Equal(t, tc.wantValue, gotValue)
			require.Equal(t, tc.wantError, gotError)
		})
	}
}

func TestPrompt_MultiSelect(t *testing.T) {
	mockError := fmt.Errorf("error")
	mockMessage := "Which dogs are best?"
	mockFinalMessage := "Best dogs:"

	testCases := map[string]struct {
		inPrompt Prompt
		inOpts   []string

		wantValue []string
		wantError error
	}{
		"should return users input": {
			inPrompt: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				internalPrompt, ok := p.(*prompt)
				require.True(t, ok, "input prompt should be type *prompt")
				require.Equal(t, mockFinalMessage, internalPrompt.FinalMessage)

				sel, ok := internalPrompt.prompter.(*survey.MultiSelect)
				require.True(t, ok, "internal prompt should be type *survey.MultiSelect")
				require.Equal(t, mockMessage, sel.Message)
				require.Empty(t, sel.Help)
				require.NotEmpty(t, sel.Options)

				result, ok := out.(*[]string)

				require.True(t, ok, "type to write user input to should be a string")

				*result = sel.Options

				require.Equal(t, 2, len(opts))

				return nil
			},
			inOpts:    []string{"bowie", "clyde", "keno", "cava", "meow"},
			wantValue: []string{"bowie", "clyde", "keno", "cava", "meow"},
			wantError: nil,
		},
		"should echo error": {
			inPrompt: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				return mockError
			},
			inOpts:    []string{"apple", "orange", "banana"},
			wantError: mockError,
		},
		"should return error if input options list is empty": {
			inOpts:    []string{},
			wantError: ErrEmptyOptions,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotValue, gotError := tc.inPrompt.MultiSelect(mockMessage, "", tc.inOpts, WithFinalMessage(mockFinalMessage))

			require.Equal(t, tc.wantValue, gotValue)
			require.Equal(t, tc.wantError, gotError)
		})
	}
}
func TestPrompt_Confirm(t *testing.T) {
	mockError := fmt.Errorf("error")
	mockMessage := "Is devx awesome?"
	mockHelpMessage := "Yes."
	mockFinalMessage := "Awesome"

	testCases := map[string]struct {
		inPrompt Prompt

		wantValue bool
		wantError error
	}{
		"should return true": {
			inPrompt: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				internalPrompt, ok := p.(*prompt)
				require.True(t, ok, "input prompt should be type *prompt")
				require.Equal(t, mockFinalMessage, internalPrompt.FinalMessage)

				confirm, ok := internalPrompt.prompter.(*survey.Confirm)
				require.True(t, ok, "internal prompt should be type *survey.Confirm")
				require.Equal(t, mockMessage, confirm.Message)
				require.Equal(t, mockHelpMessage, confirm.Help)
				require.True(t, confirm.Default)

				result, ok := out.(*bool)

				require.True(t, ok, "type to write user input to should be a bool")

				*result = true

				require.Equal(t, 2, len(opts))

				return nil
			},
			wantValue: true,
			wantError: nil,
		},
		"should echo error": {
			inPrompt: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				return mockError
			},
			wantError: mockError,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotValue, gotError := tc.inPrompt.Confirm(mockMessage, mockHelpMessage,
				WithTrueDefault(), WithFinalMessage(mockFinalMessage))

			require.Equal(t, tc.wantValue, gotValue)
			require.Equal(t, tc.wantError, gotError)
		})
	}
}

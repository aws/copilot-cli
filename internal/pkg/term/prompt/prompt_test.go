// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"fmt"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	mockError := fmt.Errorf("error")
	mockInput := "yes"
	mockMessage := "mockMessage"
	mockHelpMessage := "mockHelpMessage"

	testCases := map[string]struct {
		mockPrompter      Prompt
		mockValidatorFunc ValidatorFunc

		wantValue string
		wantError error
	}{
		"should return users input": {
			mockPrompter: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				internalPrompt, ok := p.(*survey.Input)

				require.True(t, ok, "input prompt should be type *survey.Input")
				require.Equal(t, mockMessage, internalPrompt.Message)
				require.Equal(t, mockHelpMessage, internalPrompt.Help)

				result, ok := out.(*string)

				require.True(t, ok, "type to write user input to should be a string")

				*result = mockInput

				require.Equal(t, len(opts), 2)

				return nil
			},
			mockValidatorFunc: func(ans interface{}) error {
				return nil
			},
			wantValue: mockInput,
			wantError: nil,
		},
		"should echo error": {
			mockPrompter: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				return mockError
			},
			wantError: mockError,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotValue, gotError := tc.mockPrompter.Get(mockMessage, mockHelpMessage, nil)

			require.Equal(t, tc.wantValue, gotValue)
			require.Equal(t, tc.wantError, gotError)
		})
	}
}

func TestSelectOne(t *testing.T) {
	mockError := fmt.Errorf("error")
	mockMessage := "Which droid is best droid?"
	mockHelpMessage := "All the droids."

	testCases := map[string]struct {
		mockPrompter Prompt
		mockOptions  []string

		wantValue string
		wantError error
	}{
		"should return users input": {
			mockPrompter: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				internalPrompt, ok := p.(*survey.Select)

				require.True(t, ok, "input prompt should be type *survey.Select")
				require.Equal(t, mockMessage, internalPrompt.Message)
				require.Equal(t, mockHelpMessage, internalPrompt.Help)
				require.NotEmpty(t, internalPrompt.Options)

				result, ok := out.(*string)

				require.True(t, ok, "type to write user input to should be a string")

				*result = internalPrompt.Options[0]

				require.Equal(t, len(opts), 1)

				return nil
			},
			mockOptions: []string{"r2d2", "c3po", "bb8"},
			wantValue:   "r2d2",
			wantError:   nil,
		},
		"should echo error": {
			mockPrompter: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				return mockError
			},
			mockOptions: []string{"apple", "orange", "banana"},
			wantError:   mockError,
		},
		"should return error if input options list is empty": {
			mockOptions: []string{},
			wantError:   ErrEmptyOptions,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotValue, gotError := tc.mockPrompter.SelectOne(mockMessage, mockHelpMessage, tc.mockOptions)

			require.Equal(t, tc.wantValue, gotValue)
			require.Equal(t, tc.wantError, gotError)
		})
	}
}

func TestConfirm(t *testing.T) {
	mockError := fmt.Errorf("error")
	mockMessage := "Is devx awesome?"
	mockHelpMessage := "Yes."

	testCases := map[string]struct {
		mockPrompter Prompt

		wantValue bool
		wantError error
	}{
		"should return true": {
			mockPrompter: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				internalPrompt, ok := p.(*survey.Confirm)

				require.True(t, ok, "input prompt should be type *survey.Confirm")
				require.Equal(t, mockMessage, internalPrompt.Message)
				require.Equal(t, mockHelpMessage, internalPrompt.Help)

				result, ok := out.(*bool)

				require.True(t, ok, "type to write user input to should be a bool")

				*result = true

				require.Equal(t, len(opts), 1)

				return nil
			},
			wantValue: true,
			wantError: nil,
		},
		"should echo error": {
			mockPrompter: func(p survey.Prompt, out interface{}, opts ...survey.AskOpt) error {
				return mockError
			},
			wantError: mockError,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			gotValue, gotError := tc.mockPrompter.Confirm(mockMessage, mockHelpMessage)

			require.Equal(t, tc.wantValue, gotValue)
			require.Equal(t, tc.wantError, gotError)
		})
	}
}

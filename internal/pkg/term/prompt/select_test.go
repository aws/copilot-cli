// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"fmt"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/require"
)

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

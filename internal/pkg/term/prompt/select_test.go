// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"fmt"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/require"
)

func TestOption_String(t *testing.T) {
	testCases := map[string]struct {
		input  Option
		wanted string
	}{
		"should render the value with a tab if there is no hint": {
			input: Option{
				Value: "Help me decide!",
			},
			wanted: "Help me decide!\t",
		},
		"should render a hint in parenthesis separated by a tab": {
			input: Option{
				Value: "Load Balanced Web Service",
				Hint:  "ELB -> ECS on Fargate",
			},
			wanted: "Load Balanced Web Service\t(ELB -> ECS on Fargate)",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.wanted, tc.input.String())
		})
	}
}

func TestPrompt_SelectOption(t *testing.T) {
	t.Run("should return ErrEmptyOptions if there are no options", func(t *testing.T) {
		_, err := New().SelectOption("to be or not to be?", "this is the question", nil)
		require.EqualError(t, err, ErrEmptyOptions.Error())
	})
	t.Run("should return value without hint", func(t *testing.T) {
		// GIVEN
		var p Prompt = func(p survey.Prompt, out interface{}, _ ...survey.AskOpt) error {
			sel := p.(*prompt).prompter.(*survey.Select)
			require.ElementsMatch(t, []string{
				"Load Balanced Web Service  (ELB -> ECS on Fargate)",
				"Backend Service            (ECS on Fargate)",
				"Scheduled Job              (CW Event -> StateMachine -> Fargate)",
			}, sel.Options)
			result := out.(*string)
			*result = "Load Balanced Web Service  (ELB -> ECS on Fargate)"
			return nil
		}
		opts := []Option{
			{
				Value: "Load Balanced Web Service",
				Hint:  "ELB -> ECS on Fargate",
			},
			{
				Value: "Backend Service",
				Hint:  "ECS on Fargate",
			},
			{
				Value: "Scheduled Job",
				Hint:  "CW Event -> StateMachine -> Fargate",
			},
		}

		// WHEN
		actual, err := p.SelectOption("Which workload type?", "choose!", opts)

		// THEN
		require.NoError(t, err)
		require.Equal(t, "Load Balanced Web Service", actual)
	})
	t.Run("should return value without extra spaces when there are no hints", func(t *testing.T) {
		// GIVEN
		var p Prompt = func(p survey.Prompt, out interface{}, _ ...survey.AskOpt) error {
			sel := p.(*prompt).prompter.(*survey.Select)
			require.ElementsMatch(t, []string{
				"Load Balanced Web Service  (ELB -> ECS on Fargate)",
				"Help me decide!            ",
				"Backend Service            (ECS on Fargate)",
			}, sel.Options)
			result := out.(*string)
			*result = "Help me decide!            "
			return nil
		}
		opts := []Option{
			{
				Value: "Load Balanced Web Service",
				Hint:  "ELB -> ECS on Fargate",
			},
			{
				Value: "Help me decide!",
			},
			{
				Value: "Backend Service",
				Hint:  "ECS on Fargate",
			},
		}

		// WHEN
		actual, err := p.SelectOption("Which workload type?", "choose!", opts)

		// THEN
		require.NoError(t, err)
		require.Equal(t, "Help me decide!", actual)
	})
	t.Run("should return value instead of friendly text", func(t *testing.T) {
		// GIVEN
		var p Prompt = func(p survey.Prompt, out interface{}, _ ...survey.AskOpt) error {
			sel := p.(*prompt).prompter.(*survey.Select)
			require.ElementsMatch(t, []string{
				"Friendly Load Balanced Web Service  (ELB -> ECS on Fargate)",
				"Friendly Backend Service            (ECS on Fargate)",
				"Friendly Scheduled Job              (CW Event -> StateMachine -> Fargate)",
			}, sel.Options)
			result := out.(*string)
			*result = "Friendly Load Balanced Web Service  (ELB -> ECS on Fargate)"
			return nil
		}
		opts := []Option{
			{
				Value:        "Load Balanced Web Service",
				Hint:         "ELB -> ECS on Fargate",
				FriendlyText: "Friendly Load Balanced Web Service",
			},
			{
				Value:        "Backend Service",
				Hint:         "ECS on Fargate",
				FriendlyText: "Friendly Backend Service",
			},
			{
				Value:        "Scheduled Job",
				Hint:         "CW Event -> StateMachine -> Fargate",
				FriendlyText: "Friendly Scheduled Job",
			},
		}

		// WHEN
		actual, err := p.SelectOption("Which workload type?", "choose!", opts)

		// THEN
		require.NoError(t, err)
		require.Equal(t, "Load Balanced Web Service", actual)
	})
	t.Run("should return value instead of friendly text without extra spaces when there are no hints", func(t *testing.T) {
		// GIVEN
		var p Prompt = func(p survey.Prompt, out interface{}, _ ...survey.AskOpt) error {
			sel := p.(*prompt).prompter.(*survey.Select)
			require.ElementsMatch(t, []string{
				"Load Balanced Web Service  (ELB -> ECS on Fargate)",
				"Friend! Help me decide!    ",
				"Backend Service            (ECS on Fargate)",
			}, sel.Options)
			result := out.(*string)
			*result = "Friend! Help me decide!    "
			return nil
		}
		opts := []Option{
			{
				Value: "Load Balanced Web Service",
				Hint:  "ELB -> ECS on Fargate",
			},
			{
				Value:        "Help me decide!",
				FriendlyText: "Friend! Help me decide!",
			},
			{
				Value: "Backend Service",
				Hint:  "ECS on Fargate",
			},
		}

		// WHEN
		actual, err := p.SelectOption("Which workload type?", "choose!", opts)

		// THEN
		require.NoError(t, err)
		require.Equal(t, "Help me decide!", actual)
	})
}

func TestPrompt_MultiSelectOptions(t *testing.T) {
	// GIVEN
	var p Prompt = func(p survey.Prompt, out interface{}, _ ...survey.AskOpt) error {
		sel := p.(*prompt).prompter.(*survey.MultiSelect)
		require.ElementsMatch(t, []string{
			"Service                    (AWS::ECS::Service)",
			"DiscoveryService           (AWS::ServiceDiscovery::Service)",
			"PublicNetworkLoadBalancer  (AWS::ElasticLoadBalancingV2::LoadBalancer)",
		}, sel.Options)
		result := out.(*[]string)
		*result = []string{
			"Service                    (AWS::ECS::Service)",
			"PublicNetworkLoadBalancer  (AWS::ElasticLoadBalancingV2::LoadBalancer)",
		}
		return nil
	}
	opts := []Option{
		{
			Value: "Service",
			Hint:  "AWS::ECS::Service",
		},
		{
			Value: "DiscoveryService",
			Hint:  "AWS::ServiceDiscovery::Service",
		},
		{
			Value: "PublicNetworkLoadBalancer",
			Hint:  "AWS::ElasticLoadBalancingV2::LoadBalancer",
		},
	}

	// WHEN
	actual, err := p.MultiSelectOptions("Which resource?", "choose!", opts)

	// THEN
	require.NoError(t, err)
	require.Equal(t, []string{"Service", "PublicNetworkLoadBalancer"}, actual)
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
			gotValue, gotError := tc.inPrompt.MultiSelect(mockMessage, "", tc.inOpts, nil, WithFinalMessage(mockFinalMessage))

			require.Equal(t, tc.wantValue, gotValue)
			require.Equal(t, tc.wantError, gotError)
		})
	}
}

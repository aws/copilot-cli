// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package selector

import (
	"errors"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/template"

	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/term/selector/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestCFNSelector_Resources(t *testing.T) {
	t.Run("should return a wrapped error if prompting fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// GIVEN
		mockPrompt := mocks.NewMockPrompter(ctrl)
		mockPrompt.EXPECT().
			MultiSelectOptions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("some error"))
		sel := NewCFNSelector(mockPrompt)

		// WHEN
		_, err := sel.Resources("", "", "", "")

		// THEN
		require.EqualError(t, err, "select CloudFormation resources: some error")
	})
	t.Run("should filter out custom resource when prompting", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// GIVEN
		mockPrompt := mocks.NewMockPrompter(ctrl)
		mockPrompt.EXPECT().
			MultiSelectOptions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_, _ string, opts []prompt.Option, _ ...prompt.PromptConfig) ([]string, error) {
				require.ElementsMatch(t, []prompt.Option{
					{
						Value: "LogGroup",
						Hint:  "AWS::Logs::LogGroup",
					},
					{
						Value: "AutoScalingTarget",
						Hint:  "AWS::ApplicationAutoScaling::ScalableTarget",
					},
				}, opts)
				return nil, nil
			})
		body := `
Resources:
  LogGroup:
    Type: AWS::Logs::LogGroup
  DynamicDesiredCountFunction:
    Type: AWS::Lambda::Function
  DynamicDesiredCountAction:
    Type: Custom::DynamicDesiredCountFunction
  AutoScalingTarget:
    Type: AWS::ApplicationAutoScaling::ScalableTarget
`
		sel := NewCFNSelector(mockPrompt)

		// WHEN
		_, err := sel.Resources("", "", "", body)

		// THEN
		require.NoError(t, err)
	})
	t.Run("should transform selected options", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// GIVEN
		mockPrompt := mocks.NewMockPrompter(ctrl)
		mockPrompt.EXPECT().
			MultiSelectOptions(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]string{"LogGroup", "Service"}, nil)
		body := `
Resources:
  LogGroup:
    Type: AWS::Logs::LogGroup
  AutoScalingTarget:
    Type: AWS::ApplicationAutoScaling::ScalableTarget
  Service:
    Type: AWS::ECS::Service
`
		sel := NewCFNSelector(mockPrompt)

		// WHEN
		actual, err := sel.Resources("", "", "", body)

		// THEN
		require.NoError(t, err)
		require.ElementsMatch(t, []template.CFNResource{
			{
				Type:      "AWS::Logs::LogGroup",
				LogicalID: "LogGroup",
			},
			{
				Type:      "AWS::ECS::Service",
				LogicalID: "Service",
			},
		}, actual)
	})
}

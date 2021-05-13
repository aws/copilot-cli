// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

// Package stepfunctions provides a client to make API requests to Amazon Step Functions.
package stepfunctions

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sfn"

	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/aws/stepfunctions/mocks"
	"github.com/golang/mock/gomock"
)

func TestStepFunctions_StateMachineDefinition(t *testing.T) {
	testCases := map[string]struct {
		inStateMachineARN string

		mockStepFunctionsClient func(m *mocks.Mockapi)

		wantedError      error
		wantedDefinition string
	}{
		"fail to describe state machine": {
			inStateMachineARN: "ninth inning",
			mockStepFunctionsClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeStateMachine(&sfn.DescribeStateMachineInput{
					StateMachineArn: aws.String("ninth inning"),
				}).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("describe state machine: some error"),
		},
		"success": {
			inStateMachineARN: "ninth inning",
			mockStepFunctionsClient: func(m *mocks.Mockapi) {
				m.EXPECT().DescribeStateMachine(&sfn.DescribeStateMachineInput{
					StateMachineArn: aws.String("ninth inning"),
				}).Return(&sfn.DescribeStateMachineOutput{
					Definition: aws.String("{\n  \"Version\": \"42\",\n  \"Comment\": \"very important comment\"\n}"),
				}, nil)
			},
			wantedDefinition: "{\n  \"Version\": \"42\",\n  \"Comment\": \"very important comment\"\n}",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStepFunctionsClient := mocks.NewMockapi(ctrl)
			tc.mockStepFunctionsClient(mockStepFunctionsClient)
			sfn := StepFunctions{
				client: mockStepFunctionsClient,
			}

			out, err := sfn.StateMachineDefinition(tc.inStateMachineARN)
			if tc.wantedError != nil {
				require.EqualError(t, tc.wantedError, err.Error())
			} else {
				require.Equal(t, tc.wantedDefinition, out)
			}
		})
	}
}

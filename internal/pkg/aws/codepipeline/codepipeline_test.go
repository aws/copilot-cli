// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package codepipeline

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/codepipeline/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/golang/mock/gomock"

	"github.com/aws/aws-sdk-go/service/codepipeline"
	"github.com/stretchr/testify/require"
)

func TestCodePipeline_GetPipeline(t *testing.T) {
	mockPipelineName := "pieline-dinder-badgoose-repo"
	mockError := errors.New("mockError")
	mockOutput := &codepipeline.GetPipelineOutput{
		Pipeline: &codepipeline.PipelineDeclaration{
			Name: aws.String(mockPipelineName),
		},
	}

	tests := map[string]struct {
		inPipelineName string
		callMock       func(m *mocks.MockcodepipelineClient)

		expectedError error
	}{
		"happy path": {
			inPipelineName: mockPipelineName,
			callMock: func(m *mocks.MockcodepipelineClient) {
				m.EXPECT().GetPipeline(&codepipeline.GetPipelineInput{
					Name: aws.String(mockPipelineName),
				}).Return(mockOutput, nil)

			},
			expectedError: nil,
		},
		"should wrap error": {
			inPipelineName: mockPipelineName,
			callMock: func(m *mocks.MockcodepipelineClient) {
				m.EXPECT().GetPipeline(&codepipeline.GetPipelineInput{
					Name: aws.String(mockPipelineName),
				}).Return(nil, mockError)

			},
			expectedError: fmt.Errorf("get pipeline %s: %w", mockPipelineName, mockError),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := mocks.NewMockcodepipelineClient(ctrl)

			cp := CodePipeline{
				client: mockClient,
			}

			tc.callMock(mockClient)

			// WHEN

			_, err := cp.GetPipeline(tc.inPipelineName)

			// THEN
			require.Equal(t, tc.expectedError, err)
		})
	}
}

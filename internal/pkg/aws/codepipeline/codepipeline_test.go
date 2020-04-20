// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package codepipeline

import (
	"errors"
	"fmt"
	"testing"

	cpmocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/codepipeline/mocks"
	rgmocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/resourcegroups/mocks"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/golang/mock/gomock"

	"github.com/aws/aws-sdk-go/service/codepipeline"
	"github.com/stretchr/testify/require"
)

type codepipelineMocks struct {
	cp *cpmocks.Mockapi
	rg *rgmocks.MockResourceGroupsClient
}

func TestCodePipeline_GetPipeline(t *testing.T) {
	mockPipelineName := "pipeline-dinder-badgoose-repo"
	mockError := errors.New("mockError")
	mockOutput := &codepipeline.GetPipelineOutput{
		Pipeline: &codepipeline.PipelineDeclaration{
			Name: aws.String(mockPipelineName),
		},
	}

	tests := map[string]struct {
		inPipelineName string
		callMocks      func(m codepipelineMocks)

		expectedOut   *Pipeline
		expectedError error
	}{
		"happy path": {
			inPipelineName: mockPipelineName,
			callMocks: func(m codepipelineMocks) {
				m.cp.EXPECT().GetPipeline(&codepipeline.GetPipelineInput{
					Name: aws.String(mockPipelineName),
				}).Return(mockOutput, nil)

			},
			expectedOut:   &Pipeline{Name: mockPipelineName},
			expectedError: nil,
		},
		"should wrap error": {
			inPipelineName: mockPipelineName,
			callMocks: func(m codepipelineMocks) {
				m.cp.EXPECT().GetPipeline(&codepipeline.GetPipelineInput{
					Name: aws.String(mockPipelineName),
				}).Return(nil, mockError)

			},
			expectedOut:   nil,
			expectedError: fmt.Errorf("get pipeline %s: %w", mockPipelineName, mockError),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := cpmocks.NewMockapi(ctrl)
			mockrgClient := rgmocks.NewMockResourceGroupsClient(ctrl)
			mocks := codepipelineMocks{
				cp: mockClient,
				rg: mockrgClient,
			}
			tc.callMocks(mocks)

			cp := CodePipeline{
				client:   mockClient,
				rgClient: mockrgClient,
			}

			// WHEN
			actualOut, err := cp.GetPipeline(tc.inPipelineName)

			// THEN
			require.Equal(t, tc.expectedError, err)
			require.Equal(t, tc.expectedOut, actualOut)
		})
	}
}

func TestCodePipeline_ListPipelinesForProject(t *testing.T) {
	mockProjectName := "dinder"
	mockPipelineName := "pipeline-dinder-badgoose-repo"
	mockError := errors.New("mockError")
	mockOutput := []string{
		"arn:aws:codepipeline:us-west-2:1234567890:" + mockPipelineName,
	}
	testTags := map[string]string{
		"ecs-project": mockProjectName,
	}
	badArn := "badArn"

	tests := map[string]struct {
		inProjectName string
		callMocks     func(m codepipelineMocks)
		expectedOut   []string

		expectedError error
	}{
		"happy path": {
			inProjectName: mockProjectName,
			callMocks: func(m codepipelineMocks) {
				m.rg.EXPECT().GetResourcesByTags(pipelineResourceType, testTags).Return(mockOutput, nil)
			},
			expectedOut:   []string{mockPipelineName},
			expectedError: nil,
		},
		"should return error from resourcegroups client": {
			inProjectName: mockProjectName,
			callMocks: func(m codepipelineMocks) {
				m.rg.EXPECT().GetResourcesByTags(pipelineResourceType, testTags).Return(nil, mockError)
			},
			expectedOut:   nil,
			expectedError: mockError,
		},
		"should return error for bad arns": {
			inProjectName: mockProjectName,
			callMocks: func(m codepipelineMocks) {
				m.rg.EXPECT().GetResourcesByTags(pipelineResourceType, testTags).Return([]string{badArn}, nil)
			},
			expectedOut:   nil,
			expectedError: fmt.Errorf("cannot parse pipeline ARN: %s", badArn),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockClient := cpmocks.NewMockapi(ctrl)
			mockrgClient := rgmocks.NewMockResourceGroupsClient(ctrl)
			mocks := codepipelineMocks{
				cp: mockClient,
				rg: mockrgClient,
			}
			tc.callMocks(mocks)

			cp := CodePipeline{
				client:   mockClient,
				rgClient: mockrgClient,
			}

			// WHEN
			actualOut, actualErr := cp.ListPipelinesForProject(tc.inProjectName)

			// THEN
			if actualErr != nil {
				require.EqualError(t, tc.expectedError, actualErr.Error())
			} else {
				require.Equal(t, tc.expectedOut, actualOut)
			}
		})
	}
}

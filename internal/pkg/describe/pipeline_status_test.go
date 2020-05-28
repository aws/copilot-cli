// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package describe

import (
	"errors"
	"fmt"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/codepipeline"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/describe/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type pipelineStatusDescriberMocks struct {
	pipelineStateGetter *mocks.MockpipelineStateGetter
}

var mockPipelineName = "pipeline-dinder-badgoose-repo"
var mockParsedTime = func() *time.Time {
	t, _ := time.Parse(time.RFC3339, "2020-02-02T15:04:05+00:00")
	return &t
}
var mockPipelineState = &codepipeline.PipelineState{
	PipelineName: mockPipelineName,
	StageStates: []*codepipeline.StageState{
		{
			StageName:  "Source",
			Status:     "Succeeded",
			Transition: "ENABLED",
		},
		{
			StageName:  "Build",
			Status:     "In Progress",
			Transition: "ENABLED",
		},
		{
			StageName:  "DeployTo-test",
			Status:     "Failed",
			Transition: "ENABLED",
		},
		{
			StageName:  "DeployTo-prod",
			Transition: "DISABLED",
		},
	},
	UpdatedAt: mockParsedTime(),
}

func TestPipelineStatusDescriber_Describe(t *testing.T) {
	mockError := errors.New("some error")

	testCases := map[string]struct {
		setupMocks func(m pipelineStatusDescriberMocks)

		expectedError  error
		expectedOutput *PipelineStatus
	}{
		"wraps GetPipelineState error": {
			setupMocks: func(m pipelineStatusDescriberMocks) {
				m.pipelineStateGetter.EXPECT().GetPipelineState(pipelineName).Return(nil, mockError)
			},
			expectedError:  fmt.Errorf("get pipeline status: %w", mockError),
			expectedOutput: nil,
		},
		"success": {
			setupMocks: func(m pipelineStatusDescriberMocks) {
				m.pipelineStateGetter.EXPECT().GetPipelineState(pipelineName).Return(mockPipelineState, nil)
			},
			expectedError:  nil,
			expectedOutput: &PipelineStatus{*mockPipelineState},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPipelineStateGetter := mocks.NewMockpipelineStateGetter(ctrl)

			mocks := pipelineStatusDescriberMocks{
				pipelineStateGetter: mockPipelineStateGetter,
			}
			tc.setupMocks(mocks)

			describer := &PipelineStatusDescriber{
				pipelineName: pipelineName,
				pipelineSvc:  mockPipelineStateGetter,
			}

			// WHEN
			pipelineStatus, err := describer.Describe()

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.expectedOutput, pipelineStatus, "expected output content match")
			}
		})
	}
}

func TestPipelineStatusDescriber_String(t *testing.T) {
	testCases := map[string]struct {
		testPipelineStatus  *PipelineStatus
		expectedHumanString string
		expectedJSONString  string
	}{
		"correct output": {
			testPipelineStatus: &PipelineStatus{*mockPipelineState},
			expectedHumanString: `Pipeline Status

  Stage             Status              Transition          
  -----             ------              ----------
  Source            Succeeded           ENABLED
  Build             In Progress         ENABLED
  DeployTo-test     Failed              ENABLED
  DeployTo-prod       -                 DISABLED

Last Deployment

  Updated At        3 months ago
`,
			expectedJSONString: "{\"pipelineName\":\"pipeline-dinder-badgoose-repo\",\"stageStates\":[{\"stageName\":\"Source\",\"status\":\"Succeeded\",\"transition\":\"ENABLED\"},{\"stageName\":\"Build\",\"status\":\"In Progress\",\"transition\":\"ENABLED\"},{\"stageName\":\"DeployTo-test\",\"status\":\"Failed\",\"transition\":\"ENABLED\"},{\"stageName\":\"DeployTo-prod\",\"status\":\"\",\"transition\":\"DISABLED\"}],\"updatedAt\":\"2020-02-02T15:04:05Z\"}\n",
		},
	}
	for _, tc := range testCases {
		human := tc.testPipelineStatus.HumanString()
		json, _ := tc.testPipelineStatus.JSONString()

		require.Equal(t, tc.expectedHumanString, human, "expected human output to match")
		require.Equal(t, tc.expectedJSONString, json, "expected JSON output to match")
	}
}

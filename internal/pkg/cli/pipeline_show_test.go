// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var (
	mockError        = errors.New("mock error")
	mockProjectName  = "dinder"
	mockPipelineName = "pipeline-dinder-badgoose-repo"
)

type showPipelineMocks struct {
	store       *mocks.MockstoreReader
	ws          *mocks.MockwsPipelineReader
	prompt      *mocks.Mockprompter
	pipelineSvc *mocks.MockpipelineGetter
}

func TestPipelineShow_Validate(t *testing.T) {
	testCases := map[string]struct {
		inProjectName  string
		inPipelineName string
		setupMocks     func(mocks showPipelineMocks)

		expectedErr error
	}{
		"with valid project name and pipeline name": {
			inProjectName:  mockProjectName,
			inPipelineName: mockPipelineName,
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetProject(mockProjectName).Return(&archer.Project{
						Name: "dinder",
					}, nil),
					mocks.pipelineSvc.EXPECT().GetPipeline(mockPipelineName).Return(nil, nil),
				)
			},
			expectedErr: nil,
		},
		"with invalid project name": {
			inProjectName:  mockProjectName,
			inPipelineName: "",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetProject(mockProjectName).Return(nil, mockError),
				)
			},
			expectedErr: mockError,
		},
		"with invalid pipeline name": {
			inProjectName:  mockProjectName,
			inPipelineName: "bad-pipeline",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetProject(mockProjectName).Return(&archer.Project{
						Name: "dinder",
					}, nil),
					mocks.pipelineSvc.EXPECT().GetPipeline("bad-pipeline").Return(nil, mockError),
				)
			},
			expectedErr: mockError,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstoreReader(ctrl)
			mockPipelineGetter := mocks.NewMockpipelineGetter(ctrl)

			mocks := showPipelineMocks{
				store:       mockStoreReader,
				pipelineSvc: mockPipelineGetter,
			}

			tc.setupMocks(mocks)

			opts := &showPipelineOpts{
				showPipelineVars: showPipelineVars{
					GlobalOpts: &GlobalOpts{
						projectName: tc.inProjectName,
					},
					pipelineName: tc.inPipelineName,
				},
				store:       mockStoreReader,
				pipelineSvc: mockPipelineGetter,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestPipelineShow_Ask(t *testing.T) {
	pipelineData := `
name: pipeline-dinder-badgoose-repo
version: 1

source:
  provider: GitHub
  properties:
    repository: badgoose/repo
    access_token_secret: "github-token-badgoose-repo"
    branch: master

stages:
    -
      name: test
    -
      name: prod
`

	testCases := map[string]struct {
		inProjectName  string
		inPipelineName string
		setupMocks     func(mocks showPipelineMocks)

		expectedProject  string
		expectedPipeline string
		expectedErr      error
	}{
		"happy path with project and pipeline flags": {
			inProjectName:  mockProjectName,
			inPipelineName: mockPipelineName,

			setupMocks: func(mocks showPipelineMocks) {},

			expectedProject:  mockProjectName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},

		"reads pipeline name from manifest": {
			inProjectName: mockProjectName,
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().ReadPipelineManifest().Return([]byte(pipelineData), nil),
				)
			},
			expectedProject:  mockProjectName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstoreReader(ctrl)
			mockWorkspace := mocks.NewMockwsPipelineReader(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)

			mocks := showPipelineMocks{
				store:  mockStoreReader,
				ws:     mockWorkspace,
				prompt: mockPrompt,
			}

			tc.setupMocks(mocks)

			opts := &showPipelineOpts{
				showPipelineVars: showPipelineVars{
					GlobalOpts: &GlobalOpts{
						prompt:      mockPrompt,
						projectName: tc.inProjectName,
					},
					pipelineName: tc.inPipelineName,
				},
				store: mockStoreReader,
				ws:    mockWorkspace,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.expectedProject, opts.ProjectName(), "expected project name to match")
				require.Equal(t, tc.expectedPipeline, opts.pipelineName, "expected pipeline name to match")
			}
		})
	}
}

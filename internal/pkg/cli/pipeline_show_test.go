// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/color"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var (
	mockError        = errors.New("mock error")
	mockProjectName  = "dinder"
	mockPipelineName = "pipeline-dinder-badgoose-repo"
)

type showPipelineMocks struct {
	store       *mocks.MockstoreClient
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
					mocks.store.EXPECT().GetApplication(mockProjectName).Return(&config.Application{
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
					mocks.store.EXPECT().GetApplication(mockProjectName).Return(nil, mockError),
				)
			},
			expectedErr: mockError,
		},
		"with invalid pipeline name": {
			inProjectName:  mockProjectName,
			inPipelineName: "bad-pipeline",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetApplication(mockProjectName).Return(&config.Application{
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

			mockStoreReader := mocks.NewMockstoreClient(ctrl)
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
				storeClient: mockStoreReader,
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
	mockPipelines := []string{mockPipelineName, "pipeline-the-other-one"}
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
	testTags := map[string]string{
		"copilot-application": mockProjectName,
	}

	testCases := map[string]struct {
		inProjectName  string
		inPipelineName string
		setupMocks     func(mocks showPipelineMocks)

		expectedProject  string
		expectedPipeline string
		expectedErr      error
	}{
		// happy paths
		"with project and pipeline flags": {
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
		"retrieves pipeline name from remote if no manifest found": {
			inProjectName: mockProjectName,
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().ReadPipelineManifest().Return(nil, workspace.ErrNoPipelineInWorkspace),
					mocks.pipelineSvc.EXPECT().ListPipelineNamesByTags(testTags).Return(mockPipelines, nil),
					mocks.prompt.EXPECT().SelectOne(fmt.Sprintf(fmtPipelineShowPipelineNamePrompt, color.HighlightUserInput(mockProjectName)), pipelineShowPipelineNameHelpPrompt, mockPipelines).Return(mockPipelineName, nil),
				)
			},
			expectedProject:  mockProjectName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"skip selecting if only one project found": {
			inProjectName:  "",
			inPipelineName: mockPipelineName,
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().ListApplications().Return([]*config.Application{{Name: "dinder"}}, nil),
				)
			},
			expectedProject:  mockProjectName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"skips selecting if only one pipeline found": {
			inProjectName:  mockProjectName,
			inPipelineName: "",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().ReadPipelineManifest().Return(nil, workspace.ErrNoPipelineInWorkspace),
					mocks.pipelineSvc.EXPECT().ListPipelineNamesByTags(testTags).Return([]string{mockPipelineName}, nil),
				)
			},
			expectedProject:  mockProjectName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"does not error when no pipelines found at all": {
			inProjectName:  mockProjectName,
			inPipelineName: "",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().ReadPipelineManifest().Return(nil, workspace.ErrNoPipelineInWorkspace),
					mocks.pipelineSvc.EXPECT().ListPipelineNamesByTags(testTags).Return([]string{}, nil),
				)
			},

			expectedProject:  mockProjectName,
			expectedPipeline: "",
			expectedErr:      nil,
		},

		// askProject errors
		"wraps error when fails to retrieve projects": {
			inProjectName: "",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().ListApplications().Return(nil, mockError),
				)
			},
			expectedErr: fmt.Errorf("list projects: %w", mockError),
		},
		"wraps error when no projects found": {
			inProjectName: "",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().ListApplications().Return([]*config.Application{}, nil),
				)
			},
			expectedErr: fmt.Errorf("no project found: run %s please", color.HighlightCode("project init")),
		},
		"wraps error when no projects selected": {
			inProjectName: "",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().ListApplications().Return([]*config.Application{
						{Name: "dinder"},
						{Name: "badgoose"},
					}, nil),
					mocks.prompt.EXPECT().SelectOne(pipelineShowProjectNamePrompt, pipelineShowProjectNameHelpPrompt, []string{"dinder", "badgoose"}).Return("", mockError).Times(1),
				)
			},
			expectedErr: fmt.Errorf("select projects: %w", mockError),
		},

		// askPipeline errors
		"wraps error when fails to retrieve pipelines": {
			inProjectName:  mockProjectName,
			inPipelineName: "",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().ReadPipelineManifest().Return(nil, workspace.ErrNoPipelineInWorkspace),
					mocks.pipelineSvc.EXPECT().ListPipelineNamesByTags(testTags).Return(nil, mockError),
				)
			},
			expectedErr: fmt.Errorf("list pipelines: %w", mockError),
		},
		"wraps error when no pipelines selected": {
			inProjectName: mockProjectName,
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().ReadPipelineManifest().Return(nil, workspace.ErrNoPipelineInWorkspace),
					mocks.pipelineSvc.EXPECT().ListPipelineNamesByTags(testTags).Return(mockPipelines, nil),
					mocks.prompt.EXPECT().SelectOne(fmt.Sprintf(fmtPipelineShowPipelineNamePrompt, color.HighlightUserInput(mockProjectName)), pipelineShowPipelineNameHelpPrompt, mockPipelines).Return("", mockError),
				)
			},
			expectedProject:  mockProjectName,
			expectedPipeline: mockPipelineName,
			expectedErr:      fmt.Errorf("select pipeline for project %s: %w", mockProjectName, mockError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstoreClient(ctrl)
			mockWorkspace := mocks.NewMockwsPipelineReader(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)
			mockPipelineSvc := mocks.NewMockpipelineGetter(ctrl)

			mocks := showPipelineMocks{
				store:       mockStoreReader,
				ws:          mockWorkspace,
				prompt:      mockPrompt,
				pipelineSvc: mockPipelineSvc,
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
				storeClient: mockStoreReader,
				ws:          mockWorkspace,
				pipelineSvc: mockPipelineSvc,
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

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/codepipeline"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type pipelineListMocks struct {
	prompt         *mocks.Mockprompter
	pipelineGetter *mocks.MockpipelineGetter
	pipelineLister *mocks.MockdeployedPipelineLister
	sel            *mocks.MockconfigSelector
}

func TestPipelineList_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputApp string

		mockSelector func(m *mocks.MockconfigSelector)
		mockStore    func(m *mocks.Mockstore)

		wantedApp string
		wantedErr error
	}{
		"success with no flags set": {
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(pipelineListAppNamePrompt, pipelineListAppNameHelper).Return("my-app", nil)
			},
			mockStore: func(m *mocks.Mockstore) {},
			wantedApp: "my-app",
			wantedErr: nil,
		},
		"success with app flag set": {
			inputApp:     "my-app",
			mockSelector: func(m *mocks.MockconfigSelector) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(nil, nil)
			},

			wantedApp: "my-app",
			wantedErr: nil,
		},
		"error if fail to select app": {
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(pipelineListAppNamePrompt, pipelineListAppNameHelper).Return("", errors.New("some error"))
			},
			mockStore: func(m *mocks.Mockstore) {},

			wantedApp: "my-app",
			wantedErr: fmt.Errorf("select application: some error"),
		},
		"error if passed-in app doesn't exist": {
			inputApp:     "my-app",
			mockSelector: func(m *mocks.MockconfigSelector) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(nil, errors.New("some error"))
			},

			wantedApp: "",
			wantedErr: errors.New("validate application: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSelector := mocks.NewMockconfigSelector(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			tc.mockSelector(mockSelector)
			tc.mockStore(mockStore)

			listPipelines := &listPipelineOpts{
				listPipelineVars: listPipelineVars{
					appName: tc.inputApp,
				},
				sel:   mockSelector,
				store: mockStore,
			}

			err := listPipelines.Ask()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedApp, listPipelines.appName, "expected app names to match")
			}
		})
	}
}

func TestPipelineList_Execute(t *testing.T) {
	const (
		mockAppName                    = "coolapp"
		mockPipelineResourceName       = "pipeline-coolapp-my-pipeline-repo-ABCDERANDOMRANDOM"
		mockPipelineName               = "my-pipeline-repo"
		mockLegacyPipelineResourceName = "bad-goose"
		mockLegacyPipelineName         = "bad-goose"
	)
	mockPipeline := deploy.Pipeline{
		AppName:      mockAppName,
		ResourceName: mockPipelineResourceName,
		Name:         mockPipelineName,
		IsLegacy:     false,
	}
	mockLegacyPipeline := deploy.Pipeline{
		AppName:      mockAppName,
		ResourceName: mockLegacyPipelineResourceName,
		Name:         mockLegacyPipelineName,
		IsLegacy:     true,
	}
	mockError := errors.New("mock error")
	testCases := map[string]struct {
		shouldOutputJSON bool
		setupMocks       func(m pipelineListMocks)
		expectedContent  string
		expectedErr      error
	}{
		"with JSON output": {
			shouldOutputJSON: true,
			setupMocks: func(m pipelineListMocks) {
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{mockPipeline, mockLegacyPipeline}, nil)
				m.pipelineGetter.EXPECT().
					GetPipeline(mockPipelineResourceName).
					Return(&codepipeline.Pipeline{Name: mockPipelineResourceName}, nil)
				m.pipelineGetter.EXPECT().
					GetPipeline(mockLegacyPipelineResourceName).
					Return(&codepipeline.Pipeline{Name: mockLegacyPipelineResourceName}, nil)
			},
			expectedContent: "{\"pipelines\":[{\"pipelineName\":\"pipeline-coolapp-my-pipeline-repo-ABCDERANDOMRANDOM\",\"region\":\"\",\"accountId\":\"\",\"stages\":null,\"createdAt\":\"0001-01-01T00:00:00Z\",\"updatedAt\":\"0001-01-01T00:00:00Z\"},{\"pipelineName\":\"bad-goose\",\"region\":\"\",\"accountId\":\"\",\"stages\":null,\"createdAt\":\"0001-01-01T00:00:00Z\",\"updatedAt\":\"0001-01-01T00:00:00Z\"}]}\n",
		},
		"with human output": {
			shouldOutputJSON: false,
			setupMocks: func(m pipelineListMocks) {
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{mockPipeline, mockLegacyPipeline}, nil)
			},
			expectedContent: `my-pipeline-repo
bad-goose
`,
		},
		"with failed call to list pipelines": {
			shouldOutputJSON: true,
			setupMocks: func(m pipelineListMocks) {
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return(nil, mockError)
			},
			expectedErr: fmt.Errorf("list deployed pipelines in application coolapp: mock error"),
		},
		"with failed call to get pipeline info": {
			shouldOutputJSON: true,
			setupMocks: func(m pipelineListMocks) {
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{mockPipeline}, nil)
				m.pipelineGetter.EXPECT().
					GetPipeline(mockPipelineResourceName).
					Return(nil, mockError)
			},
			expectedErr: fmt.Errorf("get pipeline info for my-pipeline-repo: mock error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mocks.NewMockprompter(ctrl)
			mockPipelineGetter := mocks.NewMockpipelineGetter(ctrl)
			mockPipelineLister := mocks.NewMockdeployedPipelineLister(ctrl)
			mockSel := mocks.NewMockconfigSelector(ctrl)

			mocks := pipelineListMocks{
				prompt:         mockPrompt,
				pipelineGetter: mockPipelineGetter,
				pipelineLister: mockPipelineLister,
				sel:            mockSel,
			}
			tc.setupMocks(mocks)

			b := &bytes.Buffer{}
			opts := &listPipelineOpts{
				listPipelineVars: listPipelineVars{
					appName:          mockAppName,
					shouldOutputJSON: tc.shouldOutputJSON,
				},
				codepipeline:   mockPipelineGetter,
				pipelineLister: mockPipelineLister,
				sel:            mockSel,
				prompt:         mockPrompt,
				w:              b,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.Equal(t, tc.expectedContent, b.String())
			}
		})
	}
}

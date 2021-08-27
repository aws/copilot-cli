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
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type pipelineListMocks struct {
	prompt      *mocks.Mockprompter
	pipelineSvc *mocks.MockpipelineGetter
	sel         *mocks.MockconfigSelector
}

func TestPipelineList_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputApp string

		mockSelector func(m *mocks.MockconfigSelector)

		wantedApp string
		wantedErr error
	}{
		"with no flags set": {
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(pipelineListAppNamePrompt, pipelineListAppNameHelper).Return("my-app", nil)
			},
			wantedApp: "my-app",
		},
		"with pipeline flags set": {
			inputApp:     "my-app",
			wantedApp:    "my-app",
			mockSelector: func(m *mocks.MockconfigSelector) {},
		},
		"error if fail to select app": {
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(pipelineListAppNamePrompt, pipelineListAppNameHelper).Return("", errors.New("some error"))
			},
			wantedApp: "my-app",
			wantedErr: fmt.Errorf("select application: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSelector := mocks.NewMockconfigSelector(ctrl)
			tc.mockSelector(mockSelector)

			listPipelines := &listPipelineOpts{
				listPipelineVars: listPipelineVars{
					appName: tc.inputApp,
				},
				sel: mockSelector,
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
	mockError := errors.New("mock error")
	testCases := map[string]struct {
		shouldOutputJSON bool
		appName          string
		setupMocks       func(m pipelineListMocks)
		expectedContent  string
		expectedErr      error
	}{
		"with JSON output": {
			shouldOutputJSON: true,
			appName:          "coolapp",
			setupMocks: func(m pipelineListMocks) {
				m.pipelineSvc.EXPECT().
					GetPipelinesByTags(gomock.Eq(map[string]string{"copilot-application": "coolapp"})).
					Return([]*codepipeline.Pipeline{
						{Name: "test"},
						{Name: "test2"},
					}, nil)
			},
			expectedContent: "{\"pipelines\":[{\"name\":\"test\",\"region\":\"\",\"accountId\":\"\",\"stages\":null,\"createdAt\":\"0001-01-01T00:00:00Z\",\"updatedAt\":\"0001-01-01T00:00:00Z\"},{\"name\":\"test2\",\"region\":\"\",\"accountId\":\"\",\"stages\":null,\"createdAt\":\"0001-01-01T00:00:00Z\",\"updatedAt\":\"0001-01-01T00:00:00Z\"}]}\n",
		},
		"with human output": {
			shouldOutputJSON: false,
			appName:          "coolapp",
			setupMocks: func(m pipelineListMocks) {
				m.pipelineSvc.EXPECT().
					ListPipelineNamesByTags(gomock.Eq(map[string]string{"copilot-application": "coolapp"})).
					Return([]string{"test", "test2"}, nil)
			},
			expectedContent: "test\ntest2\n",
		},
		"with failed call to list pipelines": {
			shouldOutputJSON: false,
			appName:          "coolapp",
			setupMocks: func(m pipelineListMocks) {
				m.pipelineSvc.EXPECT().
					ListPipelineNamesByTags(gomock.Eq(map[string]string{"copilot-application": "coolapp"})).
					Return(nil, mockError)
			},
			expectedErr: fmt.Errorf("list pipelines: mock error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mocks.NewMockprompter(ctrl)
			mockPLSvc := mocks.NewMockpipelineGetter(ctrl)
			mockSel := mocks.NewMockconfigSelector(ctrl)

			mocks := pipelineListMocks{
				prompt:      mockPrompt,
				pipelineSvc: mockPLSvc,
				sel:         mockSel,
			}
			tc.setupMocks(mocks)

			b := &bytes.Buffer{}
			opts := &listPipelineOpts{
				listPipelineVars: listPipelineVars{
					appName:          tc.appName,
					shouldOutputJSON: tc.shouldOutputJSON,
				},
				pipelineSvc: mockPLSvc,
				sel:         mockSel,
				prompt:      mockPrompt,
				w:           b,
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

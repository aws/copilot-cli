// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

var (
	mockError        = errors.New("mock error")
	mockAppName      = "dinder"
	mockPipelineName = "pipeline-dinder-badgoose-repo"
)

type showPipelineMocks struct {
	store       *mocks.Mockstore
	ws          *mocks.MockwsPipelineReader
	prompt      *mocks.Mockprompter
	pipelineSvc *mocks.MockpipelineGetter
	sel         *mocks.MockappSelector
}

func TestPipelineShow_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName      string
		inPipelineName string
		setupMocks     func(mocks showPipelineMocks)

		expectedErr error
	}{
		"with valid application name and pipeline name": {
			inAppName:      mockAppName,
			inPipelineName: mockPipelineName,
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetApplication(mockAppName).Return(&config.Application{
						Name: "dinder",
					}, nil),
					mocks.pipelineSvc.EXPECT().GetPipeline(mockPipelineName).Return(nil, nil),
				)
			},
			expectedErr: nil,
		},
		"with invalid app name": {
			inAppName:      mockAppName,
			inPipelineName: "",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetApplication(mockAppName).Return(nil, mockError),
				)
			},
			expectedErr: mockError,
		},
		"with invalid pipeline name": {
			inAppName:      mockAppName,
			inPipelineName: "bad-pipeline",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetApplication(mockAppName).Return(&config.Application{
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

			mockStoreReader := mocks.NewMockstore(ctrl)
			mockPipelineGetter := mocks.NewMockpipelineGetter(ctrl)

			mocks := showPipelineMocks{
				store:       mockStoreReader,
				pipelineSvc: mockPipelineGetter,
			}

			tc.setupMocks(mocks)

			opts := &showPipelineOpts{
				showPipelineVars: showPipelineVars{
					GlobalOpts: &GlobalOpts{
						appName: tc.inAppName,
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
		"copilot-application": mockAppName,
	}

	testCases := map[string]struct {
		inAppName      string
		inPipelineName string
		setupMocks     func(mocks showPipelineMocks)

		expectedApp      string
		expectedPipeline string
		expectedErr      error
	}{
		// happy paths
		"with application and pipeline flags": {
			inAppName:      mockAppName,
			inPipelineName: mockPipelineName,

			setupMocks: func(mocks showPipelineMocks) {},

			expectedApp:      mockAppName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},

		"reads pipeline name from manifest": {
			inAppName: mockAppName,
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().ReadPipelineManifest().Return([]byte(pipelineData), nil),
				)
			},
			expectedApp:      mockAppName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"retrieves pipeline name from remote if no manifest found": {
			inAppName: mockAppName,
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().ReadPipelineManifest().Return(nil, workspace.ErrNoPipelineInWorkspace),
					mocks.pipelineSvc.EXPECT().ListPipelineNamesByTags(testTags).Return(mockPipelines, nil),
					mocks.prompt.EXPECT().SelectOne(fmt.Sprintf(fmtPipelineShowPipelineNamePrompt, color.HighlightUserInput(mockAppName)), pipelineShowPipelineNameHelpPrompt, mockPipelines).Return(mockPipelineName, nil),
				)
			},
			expectedApp:      mockAppName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"skips selecting if only one pipeline found": {
			inAppName:      mockAppName,
			inPipelineName: "",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().ReadPipelineManifest().Return(nil, workspace.ErrNoPipelineInWorkspace),
					mocks.pipelineSvc.EXPECT().ListPipelineNamesByTags(testTags).Return([]string{mockPipelineName}, nil),
				)
			},
			expectedApp:      mockAppName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"does not error when no pipelines found at all": {
			inAppName:      mockAppName,
			inPipelineName: "",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().ReadPipelineManifest().Return(nil, workspace.ErrNoPipelineInWorkspace),
					mocks.pipelineSvc.EXPECT().ListPipelineNamesByTags(testTags).Return([]string{}, nil),
				)
			},

			expectedApp:      mockAppName,
			expectedPipeline: "",
			expectedErr:      nil,
		},
		"wraps error when no applications selected": {
			inAppName: "",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.sel.EXPECT().Application(pipelineShowAppNamePrompt, pipelineShowAppNameHelpPrompt).Return("", mockError),
				)
			},
			expectedErr: fmt.Errorf("select application: %w", mockError),
		},

		// askPipeline errors
		"wraps error when fails to retrieve pipelines": {
			inAppName:      mockAppName,
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
			inAppName: mockAppName,
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().ReadPipelineManifest().Return(nil, workspace.ErrNoPipelineInWorkspace),
					mocks.pipelineSvc.EXPECT().ListPipelineNamesByTags(testTags).Return(mockPipelines, nil),
					mocks.prompt.EXPECT().SelectOne(fmt.Sprintf(fmtPipelineShowPipelineNamePrompt, color.HighlightUserInput(mockAppName)), pipelineShowPipelineNameHelpPrompt, mockPipelines).Return("", mockError),
				)
			},
			expectedApp:      mockAppName,
			expectedPipeline: mockPipelineName,
			expectedErr:      fmt.Errorf("select pipeline for application %s: %w", mockAppName, mockError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			mockWorkspace := mocks.NewMockwsPipelineReader(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)
			mockPipelineSvc := mocks.NewMockpipelineGetter(ctrl)
			mockSel := mocks.NewMockappSelector(ctrl)

			mocks := showPipelineMocks{
				store:       mockStoreReader,
				ws:          mockWorkspace,
				prompt:      mockPrompt,
				pipelineSvc: mockPipelineSvc,
				sel:         mockSel,
			}

			tc.setupMocks(mocks)

			opts := &showPipelineOpts{
				showPipelineVars: showPipelineVars{
					GlobalOpts: &GlobalOpts{
						prompt:  mockPrompt,
						appName: tc.inAppName,
					},
					pipelineName: tc.inPipelineName,
				},
				store:       mockStoreReader,
				ws:          mockWorkspace,
				pipelineSvc: mockPipelineSvc,
				sel:         mockSel,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.expectedApp, opts.AppName(), "expected application names to match")
				require.Equal(t, tc.expectedPipeline, opts.pipelineName, "expected pipeline name to match")
			}
		})
	}
}
func TestPipelineShow_Execute(t *testing.T) {
	mockPipeline := mockDescribeData{
		data: "mockData",
		err:  mockError,
	}
	testCases := map[string]struct {
		inPipelineName   string
		setupMocks       func(m *mocks.Mockdescriber)
		shouldOutputJSON bool

		expectedContent string
		expectedErr     error
	}{
		"noop if pipeline name is empty": {
			inPipelineName: "",
			setupMocks: func(m *mocks.Mockdescriber) {
				m.EXPECT().Describe().Times(0)
			},
		},
		"happy  path": {
			inPipelineName: mockPipelineName,
			setupMocks: func(m *mocks.Mockdescriber) {
				m.EXPECT().Describe().Return(&mockPipeline, nil)
			},
			shouldOutputJSON: false,
			expectedContent:  "mockData",
			expectedErr:      nil,
		},
		"return error if fail to generate JSON output": {
			inPipelineName: mockPipelineName,
			setupMocks: func(m *mocks.Mockdescriber) {
				m.EXPECT().Describe().Return(&mockPipeline, nil)
			},
			shouldOutputJSON: true,

			expectedErr: mockError,
		},
		"return error if fail to describe pipeline": {
			inPipelineName: mockPipelineName,
			setupMocks: func(m *mocks.Mockdescriber) {
				m.EXPECT().Describe().Return(nil, mockError)
			},
			shouldOutputJSON: false,

			expectedErr: fmt.Errorf("describe pipeline %s: %w", mockPipelineName, mockError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockDescriber := mocks.NewMockdescriber(ctrl)

			tc.setupMocks(mockDescriber)

			opts := &showPipelineOpts{
				showPipelineVars: showPipelineVars{
					shouldOutputJSON: tc.shouldOutputJSON,
					pipelineName:     tc.inPipelineName,
				},
				describer:     mockDescriber,
				initDescriber: func(bool) error { return nil },
				w:             b,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.expectedContent, b.String(), "expected output content match")
			}
		})
	}
}

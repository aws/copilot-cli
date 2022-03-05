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
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type showPipelineMocks struct {
	store        *mocks.Mockstore
	ws           *mocks.MockwsPipelineReader
	prompt       *mocks.Mockprompter
	codepipeline *mocks.MockpipelineGetter
	sel          *mocks.MockappSelector
}

func TestPipelineShow_Validate(t *testing.T) {
	const (
		mockAppName = "dinder"
	)
	mockError := errors.New("mock error")
	testCases := map[string]struct {
		inAppName      string
		inPipelineName string
		setupMocks     func(mocks showPipelineMocks)

		expectedApp string
		expectedErr error
	}{
		"with valid application name via flag": {
			inAppName: mockAppName,
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetApplication(mockAppName).Return(&config.Application{
						Name: "dinder",
					}, nil),
				)
			},
			expectedApp: mockAppName,
			expectedErr: nil,
		},
		"with invalid app name": {
			inAppName: mockAppName,
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetApplication(mockAppName).Return(nil, mockError),
				)
			},
			expectedApp: "",
			expectedErr: fmt.Errorf("validate application name: %w", mockError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)

			mocks := showPipelineMocks{
				store: mockStoreReader,
			}

			tc.setupMocks(mocks)

			opts := &showPipelineOpts{
				showPipelineVars: showPipelineVars{
					appName: tc.inAppName,
				},
				store: mockStoreReader,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedApp, opts.appName)
			}
		})
	}
}

func TestPipelineShow_Ask(t *testing.T) {
	const (
		mockAppName      = "dinder"
		mockPipelineName = "pipeline-dinder-badgoose-repo"
	)
	mockError := errors.New("mock error")
	mockPipelines := []string{mockPipelineName, "pipeline-the-other-one"}
	testTags := map[string]string{
		"copilot-application": mockAppName,
	}

	testCases := map[string]struct {
		inAppName      string
		inPipelineName string
		setupMocks     func(mocks showPipelineMocks)

		expectedPipeline string
		expectedErr      error
	}{
		"with pipeline flag": {
			inPipelineName: mockPipelineName,
			inAppName:      mockAppName,

			setupMocks: func(mocks showPipelineMocks) {
				mocks.codepipeline.EXPECT().GetPipeline(mockPipelineName).Return(nil, nil)
			},
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"retrieves pipeline name from remote": {
			inAppName: mockAppName,

			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.codepipeline.EXPECT().ListPipelineNamesByTags(testTags).Return(mockPipelines, nil),
					mocks.prompt.EXPECT().SelectOne(fmt.Sprintf(fmtPipelineShowPipelineNamePrompt, color.HighlightUserInput(mockAppName)), pipelineShowPipelineNameHelpPrompt, mockPipelines, gomock.Any()).Return(mockPipelineName, nil),
				)
			},
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"prompt if no app name AND no pipeline name": {
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.sel.EXPECT().Application(gomock.Any(), gomock.Any()).Return(mockAppName, nil))
				mocks.codepipeline.EXPECT().ListPipelineNamesByTags(testTags).Return(mockPipelines, nil)
				mocks.prompt.EXPECT().SelectOne(fmt.Sprintf(fmtPipelineShowPipelineNamePrompt, color.HighlightUserInput(mockAppName)), pipelineShowPipelineNameHelpPrompt, mockPipelines, gomock.Any()).Return(mockPipelineName, nil)
			},
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"error if invalid name passed in": {
			inPipelineName: "dander",
			inAppName:      mockAppName,

			setupMocks: func(mocks showPipelineMocks) {
				mocks.codepipeline.EXPECT().GetPipeline("dander").Return(nil, errors.New("some error"))
			},
			expectedErr: errors.New("validate pipeline name dander: some error"),
		},
		"error if problem selecting app": {
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.sel.EXPECT().Application(gomock.Any(), gomock.Any()).Return("", mockError))
			},
			expectedErr: fmt.Errorf("select application: %w", mockError),
		},
		"skips selecting if only one pipeline found": {
			inAppName:      mockAppName,
			inPipelineName: "",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.codepipeline.EXPECT().ListPipelineNamesByTags(testTags).Return([]string{mockPipelineName}, nil),
				)
			},
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"wraps error when fails to retrieve pipelines": {
			inAppName:      mockAppName,
			inPipelineName: "",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.codepipeline.EXPECT().ListPipelineNamesByTags(testTags).Return(nil, mockError),
				)
			},
			expectedErr: fmt.Errorf("list pipelines: %w", mockError),
		},
		"wraps error when no pipelines selected": {
			inAppName: mockAppName,
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.codepipeline.EXPECT().ListPipelineNamesByTags(testTags).Return(mockPipelines, nil),
					mocks.prompt.EXPECT().SelectOne(fmt.Sprintf(fmtPipelineShowPipelineNamePrompt, color.HighlightUserInput(mockAppName)), pipelineShowPipelineNameHelpPrompt, mockPipelines, gomock.Any()).Return("", mockError),
				)
			},
			expectedPipeline: mockPipelineName,
			expectedErr:      fmt.Errorf("select pipeline for application %s: %w", mockAppName, mockError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstore(ctrl)
			mockWorkspace := mocks.NewMockwsPipelineReader(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)
			mockPipelineSvc := mocks.NewMockpipelineGetter(ctrl)
			mockSel := mocks.NewMockappSelector(ctrl)

			mocks := showPipelineMocks{
				store:        mockStore,
				ws:           mockWorkspace,
				prompt:       mockPrompt,
				codepipeline: mockPipelineSvc,
				sel:          mockSel,
			}

			tc.setupMocks(mocks)

			opts := &showPipelineOpts{
				showPipelineVars: showPipelineVars{
					appName: tc.inAppName,
					name:    tc.inPipelineName,
				},
				store:        mockStore,
				ws:           mockWorkspace,
				codepipeline: mockPipelineSvc,
				prompt:       mockPrompt,
				sel:          mockSel,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedPipeline, opts.name, "expected pipeline name to match")
			}
		})
	}
}
func TestPipelineShow_Execute(t *testing.T) {
	const (
		mockPipelineName = "pipeline-dinder-badgoose-repo"
	)
	mockError := errors.New("mock error")
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
					name:             tc.inPipelineName,
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
				require.NoError(t, err)
				require.Equal(t, tc.expectedContent, b.String(), "expected output content match")
			}
		})
	}
}

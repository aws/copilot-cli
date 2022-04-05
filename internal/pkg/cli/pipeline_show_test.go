// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type showPipelineMocks struct {
	store                  *mocks.Mockstore
	ws                     *mocks.MockwsPipelineReader
	prompt                 *mocks.Mockprompter
	codepipeline           *mocks.MockpipelineGetter
	sel                    *mocks.MockcodePipelineSelector
	deployedPipelineLister *mocks.MockdeployedPipelineLister
}

func TestPipelineShow_Ask(t *testing.T) {
	const (
		mockAppName      = "dinder"
		mockPipelineName = "pipeline-dinder-badgoose-repo"
	)
	mockError := errors.New("mock error")

	testCases := map[string]struct {
		inAppName      string
		inPipelineName string
		setupMocks     func(mocks showPipelineMocks)

		expectedApp      string
		expectedPipeline string
		expectedErr      error
	}{
		"with valid application name via flag, no pipeline name flag": {
			inAppName: mockAppName,
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetApplication(mockAppName).Return(&config.Application{
						Name: "dinder",
					}, nil),
					mocks.sel.EXPECT().DeployedPipeline(gomock.Any(), gomock.Any(), mockAppName).Return(deploy.Pipeline{
						Name:    mockPipelineName,
						AppName: mockAppName,
					}, nil),
				)
			},
			expectedApp:      mockAppName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"error if problem selecting app": {
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.sel.EXPECT().Application(gomock.Any(), gomock.Any()).Return("", mockError))
			},
			expectedErr: fmt.Errorf("select application: %w", mockError),
		},
		"with pipeline flag": {
			inPipelineName: mockPipelineName,
			inAppName:      mockAppName,

			setupMocks: func(mocks showPipelineMocks) {
				mocks.store.EXPECT().GetApplication(mockAppName).Return(&config.Application{
					Name: "dinder",
				}, nil)
				mocks.deployedPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{
					{
						Name: mockPipelineName,
					},
				}, nil)
			},
			expectedApp:      mockAppName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"prompt if no app name AND no pipeline name": {
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.sel.EXPECT().Application(gomock.Any(), gomock.Any()).Return(mockAppName, nil))
				mocks.sel.EXPECT().DeployedPipeline(gomock.Any(), gomock.Any(), mockAppName).Return(deploy.Pipeline{
					Name: mockPipelineName,
				}, nil)
			},
			expectedApp:      mockAppName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"error if invalid name passed in": {
			inPipelineName: "dander",
			inAppName:      mockAppName,

			setupMocks: func(mocks showPipelineMocks) {
				mocks.store.EXPECT().GetApplication(mockAppName).Return(&config.Application{
					Name: "dinder",
				}, nil)
				mocks.deployedPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
			},
			expectedApp: mockAppName,
			expectedErr: errors.New("validate pipeline name dander: cannot find pipeline named dander"),
		},
		"error occurs when listing deployed pipelines for validation": {
			inPipelineName: "dander",
			inAppName:      mockAppName,

			setupMocks: func(mocks showPipelineMocks) {
				mocks.store.EXPECT().GetApplication(mockAppName).Return(&config.Application{
					Name: "dinder",
				}, nil)
				mocks.deployedPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, errors.New("some error"))
			},
			expectedApp: mockAppName,
			expectedErr: errors.New("validate pipeline name dander: list deployed pipelines: some error"),
		},
		"wraps error when fails to retrieve deployed pipelines to select": {
			inAppName:      mockAppName,
			inPipelineName: "",
			setupMocks: func(mocks showPipelineMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetApplication(mockAppName).Return(&config.Application{
						Name: "dinder",
					}, nil),
					mocks.sel.EXPECT().DeployedPipeline(gomock.Any(), gomock.Any(), mockAppName).Return(deploy.Pipeline{}, mockError),
				)
			},
			expectedErr: fmt.Errorf("select deployed pipelines: %w", mockError),
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
			mockSel := mocks.NewMockcodePipelineSelector(ctrl)
			mockDeployedPipelineLister := mocks.NewMockdeployedPipelineLister(ctrl)

			mocks := showPipelineMocks{
				store:                  mockStore,
				ws:                     mockWorkspace,
				prompt:                 mockPrompt,
				codepipeline:           mockPipelineSvc,
				sel:                    mockSel,
				deployedPipelineLister: mockDeployedPipelineLister,
			}

			tc.setupMocks(mocks)

			opts := &showPipelineOpts{
				showPipelineVars: showPipelineVars{
					appName: tc.inAppName,
					name:    tc.inPipelineName,
				},
				store:                  mockStore,
				ws:                     mockWorkspace,
				codepipeline:           mockPipelineSvc,
				prompt:                 mockPrompt,
				sel:                    mockSel,
				deployedPipelineLister: mockDeployedPipelineLister,
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

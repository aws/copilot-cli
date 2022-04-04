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

type pipelineStatusMocks struct {
	store                  *mocks.Mockstore
	ws                     *mocks.MockwsPipelineReader
	prompt                 *mocks.Mockprompter
	codepipeline           *mocks.MockpipelineGetter
	describer              *mocks.Mockdescriber
	sel                    *mocks.MockcodePipelineSelector
	deployedPipelineLister *mocks.MockdeployedPipelineLister
}

func TestPipelineStatus_Ask(t *testing.T) {
	const (
		mockAppName      = "dinder"
		mockPipelineName = "pipeline-dinder-badgoose-repo"
	)
	mockError := errors.New("mock error")
	mockTestCommands := []string{"make test", "echo 'honk'"}

	testCases := map[string]struct {
		testAppName      string
		testPipelineName string
		setupMocks       func(mocks pipelineStatusMocks)

		expectedApp          string
		expectedPipeline     string
		expectedTestCommands []string
		expectedErr          error
	}{
		"with invalid app name": {
			testAppName: mockAppName,
			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetApplication(mockAppName).Return(nil, mockError),
				)
			},
			expectedApp: "",
			expectedErr: fmt.Errorf("validate application name: %w", mockError),
		},
		"prompts for app name if not passed in with flag and name not passed in": {
			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.sel.EXPECT().Application(gomock.Any(), gomock.Any()).Return(mockAppName, nil),
					mocks.sel.EXPECT().DeployedPipeline(gomock.Any(), gomock.Any(), mockAppName).Return(deploy.Pipeline{
						Name: mockPipelineName,
					}, nil),
				)
			},

			expectedApp:      mockAppName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"errors if fail to select app name": {
			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.sel.EXPECT().Application(gomock.Any(), gomock.Any()).Return("", errors.New("some error")))
			},
			expectedApp: "",
			expectedErr: errors.New("select application: some error"),
		},
		"retrieves pipeline name from remote": {
			testAppName: mockAppName,
			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetApplication(mockAppName).Return(&config.Application{
						Name: "dinder",
					}, nil),
					mocks.sel.EXPECT().DeployedPipeline(gomock.Any(), gomock.Any(), mockAppName).Return(deploy.Pipeline{
						Name: mockPipelineName,
					}, nil),
				)
			},
			expectedApp:      mockAppName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"wraps error when fails to retrieve pipelines": {
			testAppName:      mockAppName,
			testPipelineName: "",
			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetApplication(mockAppName).Return(&config.Application{
						Name: "dinder",
					}, nil),
					mocks.sel.EXPECT().DeployedPipeline(gomock.Any(), gomock.Any(), mockAppName).Return(deploy.Pipeline{}, mockError),
				)
			},
			expectedApp: mockAppName,
			expectedErr: fmt.Errorf("select deployed pipelines: %w", mockError),
		},
		"success with flags": {
			testAppName:      mockAppName,
			testPipelineName: mockPipelineName,

			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetApplication(mockAppName).Return(&config.Application{
						Name: "dinder",
					}, nil),
					mocks.deployedPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{
						{
							Name: mockPipelineName,
						},
					}, nil),
				)
			},
			expectedApp:          mockAppName,
			expectedPipeline:     mockPipelineName,
			expectedTestCommands: mockTestCommands,
			expectedErr:          nil,
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstore(ctrl)
			mockWS := mocks.NewMockwsPipelineReader(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)
			mockPLSvc := mocks.NewMockpipelineGetter(ctrl)
			mockSel := mocks.NewMockcodePipelineSelector(ctrl)

			mocks := pipelineStatusMocks{
				store:                  mockStore,
				ws:                     mockWS,
				prompt:                 mockPrompt,
				codepipeline:           mockPLSvc,
				sel:                    mockSel,
				deployedPipelineLister: mocks.NewMockdeployedPipelineLister(ctrl),
			}

			tc.setupMocks(mocks)

			opts := &pipelineStatusOpts{
				pipelineStatusVars: pipelineStatusVars{
					appName: tc.testAppName,
					name:    tc.testPipelineName,
				},
				store:                  mockStore,
				ws:                     mockWS,
				codepipeline:           mockPLSvc,
				sel:                    mockSel,
				prompt:                 mockPrompt,
				deployedPipelineLister: mocks.deployedPipelineLister,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedApp, opts.appName, "expected app name to match")
				require.Equal(t, tc.expectedPipeline, opts.name, "expected pipeline name to match")

			}
		})
	}
}

func TestPipelineStatus_Execute(t *testing.T) {
	const (
		mockPipelineName = "pipeline-dinder-badgoose-repo"
	)
	mockError := errors.New("mock error")
	mockPipelineStatus := mockDescribeData{
		data: "mockData",
		err:  mockError,
	}
	testCases := map[string]struct {
		shouldOutputJSON bool
		pipelineName     string
		setupMocks       func(m pipelineStatusMocks)

		expectedContent string
		expectedError   error
	}{
		"errors if fail to return JSON output": {
			pipelineName:     mockPipelineName,
			shouldOutputJSON: true,
			setupMocks: func(m pipelineStatusMocks) {
				m.describer.EXPECT().Describe().Return(&mockPipelineStatus, nil)
			},
			expectedError: mockError,
		},
		"success with HumanString": {
			pipelineName: mockPipelineName,
			setupMocks: func(m pipelineStatusMocks) {
				m.describer.EXPECT().Describe().Return(&mockPipelineStatus, nil)
			},
			expectedContent: "mockData",
			expectedError:   nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockDescriber := mocks.NewMockdescriber(ctrl)

			mocks := pipelineStatusMocks{
				describer: mockDescriber,
			}

			tc.setupMocks(mocks)

			opts := &pipelineStatusOpts{
				pipelineStatusVars: pipelineStatusVars{
					shouldOutputJSON: tc.shouldOutputJSON,
					name:             tc.pipelineName,
				},
				describer:     mockDescriber,
				initDescriber: func(o *pipelineStatusOpts) error { return nil },
				w:             b,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedContent, b.String(), "expected output content to match")
			}
		})
	}
}

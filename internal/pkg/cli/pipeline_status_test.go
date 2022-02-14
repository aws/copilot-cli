// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type pipelineStatusMocks struct {
	store       *mocks.Mockstore
	ws          *mocks.MockwsPipelineReader
	prompt      *mocks.Mockprompter
	pipelineSvc *mocks.MockpipelineGetter
	describer   *mocks.Mockdescriber
	sel         *mocks.MockappSelector
}

func TestPipelineStatus_Validate(t *testing.T) {
	const (
		mockAppName      = "dinder"
		mockPipelineName = "pipeline-dinder-badgoose-repo"
	)
	mockError := errors.New("mock error")
	testCases := map[string]struct {
		testAppName      string
		testPipelineName string
		setupMocks       func(mocks pipelineStatusMocks)

		expectedErr error
	}{
		"errors if app name is invalid": {
			testAppName: "bad-app-le",
			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetApplication("bad-app-le").Return(nil, mockError),
				)
			},
			expectedErr: mockError,
		},
		"errors if pipeline name is invalid": {
			testAppName:      mockAppName,
			testPipelineName: "no-good-pipeline",
			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetApplication(mockAppName).Return(&config.Application{
						Name: "my-app",
					}, nil),
					mocks.pipelineSvc.EXPECT().GetPipeline("no-good-pipeline").Return(nil, mockError),
				)
			},
			expectedErr: mockError,
		},
		"success": {
			testAppName:      mockAppName,
			testPipelineName: mockPipelineName,
			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetApplication(mockAppName).Return(&config.Application{
						Name: "my-app",
					}, nil),
					mocks.pipelineSvc.EXPECT().GetPipeline(mockPipelineName).Return(nil, nil),
				)
			},
			expectedErr: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			mockPipelineStateGetter := mocks.NewMockpipelineGetter(ctrl)

			mocks := pipelineStatusMocks{
				store:       mockStoreReader,
				pipelineSvc: mockPipelineStateGetter,
			}

			tc.setupMocks(mocks)

			opts := &pipelineStatusOpts{
				pipelineStatusVars: pipelineStatusVars{
					appName:      tc.testAppName,
					pipelineName: tc.testPipelineName,
				},
				store:       mockStoreReader,
				pipelineSvc: mockPipelineStateGetter,
			}

			//WHEN
			err := opts.Validate()

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPipelineStatus_Ask(t *testing.T) {
	const (
		mockAppName                = "dinder"
		mockPipelineName           = "pipeline-dinder-badgoose-repo"
		pipelineManifestLegacyPath = "copilot/pipeline.yml"
	)
	mockError := errors.New("mock error")
	testTags := map[string]string{
		"copilot-application": mockAppName,
	}
	mockPipelines := []string{mockPipelineName, "pipeline-the-other-one"}
	mockTestCommands := []string{"make test", "echo 'honk'"}
	mockPipelineManifest := &manifest.PipelineManifest{
		Name:    mockPipelineName,
		Version: 1,
	}

	testCases := map[string]struct {
		testAppName      string
		testPipelineName string
		setupMocks       func(mocks pipelineStatusMocks)

		expectedApp          string
		expectedPipeline     string
		expectedTestCommands []string
		expectedErr          error
	}{
		"skips selecting if only one pipeline found": {
			testAppName:      mockAppName,
			testPipelineName: "",
			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().PipelineManifestLegacyPath().Return(pipelineManifestLegacyPath, nil),
					mocks.ws.EXPECT().ReadPipelineManifest(pipelineManifestLegacyPath).Return(nil, workspace.ErrNoPipelineInWorkspace),
					mocks.pipelineSvc.EXPECT().ListPipelineNamesByTags(testTags).Return([]string{mockPipelineName}, nil),
				)
			},
			expectedApp:      mockAppName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"reads pipeline name and test commands from manifest": {
			testAppName: mockAppName,
			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().PipelineManifestLegacyPath().Return(pipelineManifestLegacyPath, nil),
					mocks.ws.EXPECT().ReadPipelineManifest(pipelineManifestLegacyPath).Return(mockPipelineManifest, nil),
				)
			},
			expectedApp:          mockAppName,
			expectedPipeline:     mockPipelineName,
			expectedTestCommands: mockTestCommands,
			expectedErr:          nil,
		},
		"retrieves pipeline name from remote if no manifest found": {
			testAppName: mockAppName,
			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().PipelineManifestLegacyPath().Return(pipelineManifestLegacyPath, nil),
					mocks.ws.EXPECT().ReadPipelineManifest(pipelineManifestLegacyPath).Return(nil, workspace.ErrNoPipelineInWorkspace),
					mocks.pipelineSvc.EXPECT().ListPipelineNamesByTags(testTags).Return(mockPipelines, nil),
					mocks.prompt.EXPECT().SelectOne(fmt.Sprintf(fmtPipelineStatusPipelineNamePrompt, color.HighlightUserInput(mockAppName)), pipelineStatusPipelineNameHelpPrompt, mockPipelines, gomock.Any()).Return(mockPipelineName, nil),
				)
			},
			expectedApp:      mockAppName,
			expectedPipeline: mockPipelineName,
			expectedErr:      nil,
		},
		"throws error if no pipeline found": {
			testAppName:      mockAppName,
			testPipelineName: "",
			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().PipelineManifestLegacyPath().Return(pipelineManifestLegacyPath, nil),
					mocks.ws.EXPECT().ReadPipelineManifest(pipelineManifestLegacyPath).Return(nil, workspace.ErrNoPipelineInWorkspace),
					mocks.pipelineSvc.EXPECT().ListPipelineNamesByTags(testTags).Return([]string{}, nil),
				)
			},
			expectedApp:      mockAppName,
			expectedPipeline: "",
			expectedErr:      fmt.Errorf("no pipelines found for application %s", mockAppName),
		},
		"wraps error when no applications selected": {
			testAppName: "",
			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.sel.EXPECT().Application(pipelineStatusAppNamePrompt, pipelineStatusAppNameHelpPrompt).Return("", mockError),
				)
			},
			expectedErr: fmt.Errorf("select application: %w", mockError),
		},
		"wraps error when fails to retrieve pipelines": {
			testAppName:      mockAppName,
			testPipelineName: "",
			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().PipelineManifestLegacyPath().Return(pipelineManifestLegacyPath, nil),
					mocks.ws.EXPECT().ReadPipelineManifest(pipelineManifestLegacyPath).Return(nil, workspace.ErrNoPipelineInWorkspace),
					mocks.pipelineSvc.EXPECT().ListPipelineNamesByTags(testTags).Return(nil, mockError),
				)
			},
			expectedErr: fmt.Errorf("list pipelines: %w", mockError),
		},
		"wraps error when no pipelines selected": {
			testAppName: mockAppName,
			setupMocks: func(mocks pipelineStatusMocks) {
				gomock.InOrder(
					mocks.ws.EXPECT().PipelineManifestLegacyPath().Return(pipelineManifestLegacyPath, nil),
					mocks.ws.EXPECT().ReadPipelineManifest(pipelineManifestLegacyPath).Return(nil, workspace.ErrNoPipelineInWorkspace),
					mocks.pipelineSvc.EXPECT().ListPipelineNamesByTags(testTags).Return(mockPipelines, nil),
					mocks.prompt.EXPECT().SelectOne(fmt.Sprintf(fmtPipelineStatusPipelineNamePrompt, color.HighlightUserInput(mockAppName)), pipelineStatusPipelineNameHelpPrompt, mockPipelines, gomock.Any()).Return("", mockError),
				)
			},
			expectedApp:      mockAppName,
			expectedPipeline: mockPipelineName,
			expectedErr:      fmt.Errorf("select pipeline for application %s: %w", mockAppName, mockError),
		},
		"success with flags": {
			testAppName:      mockAppName,
			testPipelineName: mockPipelineName,

			setupMocks:           func(mocks pipelineStatusMocks) {},
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
			mockSel := mocks.NewMockappSelector(ctrl)

			mocks := pipelineStatusMocks{
				store:       mockStore,
				ws:          mockWS,
				prompt:      mockPrompt,
				pipelineSvc: mockPLSvc,
				sel:         mockSel,
			}

			tc.setupMocks(mocks)

			opts := &pipelineStatusOpts{
				pipelineStatusVars: pipelineStatusVars{
					appName:      tc.testAppName,
					pipelineName: tc.testPipelineName,
				},
				store:       mockStore,
				ws:          mockWS,
				pipelineSvc: mockPLSvc,
				sel:         mockSel,
				prompt:      mockPrompt,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedApp, opts.appName, "expected application names to match")
				require.Equal(t, tc.expectedPipeline, opts.pipelineName, "expected pipeline name to match")
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
					pipelineName:     tc.pipelineName,
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

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
	"github.com/aws/copilot-cli/internal/pkg/describe"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type pipelineListMocks struct {
	prompt         *mocks.Mockprompter
	sel            *mocks.MockconfigSelector
	store          *mocks.Mockstore
	workspace      *mocks.MockwsPipelineGetter
	pipelineLister *mocks.MockdeployedPipelineLister
	describer      *mocks.Mockdescriber
}

func TestPipelineList_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputApp                 string
		inWsAppName              string
		shouldShowLocalPipelines bool

		setupMocks func(mocks pipelineListMocks)

		wantedApp string
		wantedErr error
	}{
		"success with no flags set": {
			setupMocks: func(m pipelineListMocks) {
				m.sel.EXPECT().Application(pipelineListAppNamePrompt, pipelineListAppNameHelper).Return("my-app", nil)
			},
			wantedApp: "my-app",
			wantedErr: nil,
		},
		"success with app flag set": {
			inputApp: "my-app",
			setupMocks: func(m pipelineListMocks) {
				m.store.EXPECT().GetApplication("my-app").Return(nil, nil)
			},
			wantedApp: "my-app",
			wantedErr: nil,
		},
		"error if fail to select app": {
			setupMocks: func(m pipelineListMocks) {
				m.sel.EXPECT().Application(pipelineListAppNamePrompt, pipelineListAppNameHelper).Return("", errors.New("some error"))
			},
			wantedApp: "my-app",
			wantedErr: fmt.Errorf("select application: some error"),
		},
		"error if passed-in app doesn't exist": {
			inputApp: "my-app",
			setupMocks: func(m pipelineListMocks) {
				m.store.EXPECT().GetApplication("my-app").Return(nil, errors.New("some error"))
			},
			wantedApp: "",
			wantedErr: errors.New("validate application: some error"),
		},
		"using workspace successful": {
			inWsAppName: "my-app",
			setupMocks: func(m pipelineListMocks) {
				m.store.EXPECT().GetApplication("my-app").Return(nil, nil)
			},
			shouldShowLocalPipelines: true,
		},
		"--local not in workspace": {
			inWsAppName:              "",
			shouldShowLocalPipelines: true,
			wantedErr:                errNoAppInWorkspace,
		},
		"--local workspace and app name mismatch": {
			inWsAppName:              "my-app",
			inputApp:                 "not-my-app",
			shouldShowLocalPipelines: true,
			wantedErr:                errors.New("cannot specify app not-my-app because the workspace is already registered with app my-app"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := pipelineListMocks{
				prompt:         mocks.NewMockprompter(ctrl),
				sel:            mocks.NewMockconfigSelector(ctrl),
				store:          mocks.NewMockstore(ctrl),
				workspace:      mocks.NewMockwsPipelineGetter(ctrl),
				pipelineLister: mocks.NewMockdeployedPipelineLister(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(mocks)
			}

			opts := &listPipelineOpts{
				listPipelineVars: listPipelineVars{
					appName:                  tc.inputApp,
					shouldShowLocalPipelines: tc.shouldShowLocalPipelines,
				},
				prompt:         mocks.prompt,
				sel:            mocks.sel,
				store:          mocks.store,
				workspace:      mocks.workspace,
				pipelineLister: mocks.pipelineLister,
				wsAppName:      tc.inWsAppName,
			}

			err := opts.Ask()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedApp, opts.appName, "expected app names to match")
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
		shouldOutputJSON         bool
		shouldShowLocalPipelines bool
		setupMocks               func(m pipelineListMocks)
		expectedContent          string
		expectedErr              error
	}{
		"with JSON output": {
			shouldOutputJSON: true,
			setupMocks: func(m pipelineListMocks) {
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{mockPipeline, mockLegacyPipeline}, nil)
				m.describer.EXPECT().Describe().Return(&describe.Pipeline{
					Name: mockLegacyPipelineName,
					Pipeline: codepipeline.Pipeline{
						Name: mockLegacyPipelineResourceName,
					},
				}, nil)
				m.describer.EXPECT().Describe().Return(&describe.Pipeline{
					Name: mockPipelineName,
					Pipeline: codepipeline.Pipeline{
						Name: mockPipelineResourceName,
					},
				}, nil)
			},
			expectedContent: `{"pipelines":[{"name":"bad-goose","pipelineName":"bad-goose","region":"","accountId":"","stages":null,"createdAt":"0001-01-01T00:00:00Z","updatedAt":"0001-01-01T00:00:00Z"},{"name":"my-pipeline-repo","pipelineName":"pipeline-coolapp-my-pipeline-repo-ABCDERANDOMRANDOM","region":"","accountId":"","stages":null,"createdAt":"0001-01-01T00:00:00Z","updatedAt":"0001-01-01T00:00:00Z"}]}` + "\n",
		},
		"with human output": {
			setupMocks: func(m pipelineListMocks) {
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{mockPipeline, mockLegacyPipeline}, nil)
			},
			expectedContent: `bad-goose
my-pipeline-repo
`,
		},
		"with failed call to list pipelines": {
			shouldOutputJSON: true,
			setupMocks: func(m pipelineListMocks) {
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return(nil, mockError)
			},
			expectedErr: fmt.Errorf("list deployed pipelines: mock error"),
		},
		"with failed call to get pipeline info": {
			shouldOutputJSON: true,
			setupMocks: func(m pipelineListMocks) {
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{mockPipeline}, nil)
				m.describer.EXPECT().Describe().Return(nil, mockError)
			},
			expectedErr: fmt.Errorf(`describe pipeline %q: mock error`, mockPipelineResourceName),
		},
		"ls --local": {
			shouldShowLocalPipelines: true,
			setupMocks: func(m pipelineListMocks) {
				m.workspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{{Name: mockLegacyPipeline.Name}, {Name: mockPipeline.Name}}, nil)
			},
			expectedContent: `bad-goose
my-pipeline-repo
`,
		},
		"ls --local --json with one deployed, one local": {
			shouldShowLocalPipelines: true,
			shouldOutputJSON:         true,
			setupMocks: func(m pipelineListMocks) {
				m.workspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{
					{Name: mockLegacyPipeline.Name, Path: "/copilot/pipeline.yml"},
					{Name: mockPipeline.Name, Path: "/copilot/pipelines/my-pipeline-repo/manifest.yml"}}, nil)
				m.pipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{mockPipeline}, nil)
				m.describer.EXPECT().Describe().Return(&describe.Pipeline{
					Name: mockPipelineName,
					Pipeline: codepipeline.Pipeline{
						Name: mockPipelineResourceName,
					},
				}, nil)
			},
			expectedContent: `{"pipelines":[{"name":"bad-goose","manifestPath":"/copilot/pipeline.yml"},{"name":"my-pipeline-repo","manifestPath":"/copilot/pipelines/my-pipeline-repo/manifest.yml","pipelineName":"pipeline-coolapp-my-pipeline-repo-ABCDERANDOMRANDOM"}]}` + "\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := pipelineListMocks{
				prompt:         mocks.NewMockprompter(ctrl),
				sel:            mocks.NewMockconfigSelector(ctrl),
				store:          mocks.NewMockstore(ctrl),
				workspace:      mocks.NewMockwsPipelineGetter(ctrl),
				pipelineLister: mocks.NewMockdeployedPipelineLister(ctrl),
				describer:      mocks.NewMockdescriber(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(mocks)
			}

			b := &bytes.Buffer{}
			opts := &listPipelineOpts{
				listPipelineVars: listPipelineVars{
					appName:                  mockAppName,
					shouldOutputJSON:         tc.shouldOutputJSON,
					shouldShowLocalPipelines: tc.shouldShowLocalPipelines,
				},
				prompt:         mocks.prompt,
				sel:            mocks.sel,
				store:          mocks.store,
				w:              b,
				workspace:      mocks.workspace,
				pipelineLister: mocks.pipelineLister,
				newDescriber: func(pipeline deploy.Pipeline) (describer, error) {
					return mocks.describer, nil
				},
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

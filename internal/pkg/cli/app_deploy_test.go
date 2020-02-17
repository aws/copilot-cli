// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
)

func TestAppDeployOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inProjectName string
		inAppName     string
		inEnvName     string

		mockWs    func(m *climocks.MockwsAppReader)
		mockStore func(m *climocks.MockprojectService)

		wantedError error
	}{
		"no existing projects": {
			mockWs:    func(m *climocks.MockwsAppReader) {},
			mockStore: func(m *climocks.MockprojectService) {},

			wantedError: errNoProjectInWorkspace,
		},
		"with workspace error": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			mockWs: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Return(nil, errors.New("some error"))
			},
			mockStore: func(m *climocks.MockprojectService) {},

			wantedError: errors.New("list applications in the workspace: some error"),
		},
		"with application not in workspace": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			mockWs: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Return([]string{}, nil)
			},
			mockStore: func(m *climocks.MockprojectService) {},

			wantedError: errors.New("application frontend not found in the workspace"),
		},
		"with unknown environment": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			mockWs:        func(m *climocks.MockwsAppReader) {},
			mockStore: func(m *climocks.MockprojectService) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(nil, errors.New("unknown env"))
			},

			wantedError: errors.New("get environment test from metadata store: unknown env"),
		},
		"successful validation": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			inEnvName:     "test",
			mockWs: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Return([]string{"frontend"}, nil)
			},
			mockStore: func(m *climocks.MockprojectService) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(&archer.Environment{Name: "test"}, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockWs := climocks.NewMockwsAppReader(ctrl)
			mockStore := climocks.NewMockprojectService(ctrl)
			tc.mockWs(mockWs)
			tc.mockStore(mockStore)
			opts := appDeployOpts{
				appDeployVars: appDeployVars{
					GlobalOpts: &GlobalOpts{
						projectName: tc.inProjectName,
					},
					AppName: tc.inAppName,
					EnvName: tc.inEnvName,
				},
				workspaceService: mockWs,
				projectService:   mockStore,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestAppDeployOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inProjectName string
		inAppName     string
		inEnvName     string
		inImageTag    string

		mockWs     func(m *climocks.MockwsAppReader)
		mockStore  func(m *climocks.MockprojectService)
		mockPrompt func(m *climocks.Mockprompter)

		wantedAppName  string
		wantedEnvName  string
		wantedImageTag string
		wantedError    error
	}{
		"no applications in the workspace": {
			mockWs: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Return([]string{}, nil)
			},
			mockStore:  func(m *climocks.MockprojectService) {},
			mockPrompt: func(m *climocks.Mockprompter) {},

			wantedError: errors.New("no applications found in the workspace"),
		},
		"default to single application": {
			inEnvName:  "test",
			inImageTag: "latest",
			mockWs: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Return([]string{"frontend"}, nil)
			},
			mockStore:  func(m *climocks.MockprojectService) {},
			mockPrompt: func(m *climocks.Mockprompter) {},

			wantedAppName:  "frontend",
			wantedEnvName:  "test",
			wantedImageTag: "latest",
		},
		"prompts for application name if there are more than one option": {
			inEnvName:  "test",
			inImageTag: "latest",
			mockWs: func(m *climocks.MockwsAppReader) {
				m.EXPECT().AppNames().Return([]string{"frontend", "webhook"}, nil)
			},
			mockStore: func(m *climocks.MockprojectService) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne("Select an application", "", []string{"frontend", "webhook"}).
					Return("frontend", nil)
			},

			wantedAppName:  "frontend",
			wantedEnvName:  "test",
			wantedImageTag: "latest",
		},
		"fails to list environments": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			inImageTag:    "latest",
			mockWs:        func(m *climocks.MockwsAppReader) {},
			mockStore: func(m *climocks.MockprojectService) {
				m.EXPECT().ListEnvironments("phonetool").Return(nil, errors.New("some error"))
			},
			mockPrompt: func(m *climocks.Mockprompter) {
			},

			wantedError: errors.New("get environments for project phonetool from metadata store: some error"),
		},
		"no existing environments": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			inImageTag:    "latest",
			mockWs:        func(m *climocks.MockwsAppReader) {},
			mockStore: func(m *climocks.MockprojectService) {
				m.EXPECT().ListEnvironments("phonetool").Return([]*archer.Environment{}, nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
			},

			wantedError: errors.New("no environments found in project phonetool"),
		},
		"defaults to single environment": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			inImageTag:    "latest",
			mockWs:        func(m *climocks.MockwsAppReader) {},
			mockStore: func(m *climocks.MockprojectService) {
				m.EXPECT().ListEnvironments("phonetool").Return([]*archer.Environment{
					{
						Name: "test",
					},
				}, nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
			},

			wantedAppName:  "frontend",
			wantedEnvName:  "test",
			wantedImageTag: "latest",
		},
		"prompts for environment name if there are more than one option": {
			inProjectName: "phonetool",
			inAppName:     "frontend",
			inImageTag:    "latest",
			mockWs:        func(m *climocks.MockwsAppReader) {},
			mockStore: func(m *climocks.MockprojectService) {
				m.EXPECT().ListEnvironments("phonetool").Return([]*archer.Environment{
					{
						Name: "test",
					},
					{
						Name: "prod-iad",
					},
				}, nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne("Select an environment", "", []string{"test", "prod-iad"}).
					Return("prod-iad", nil)
			},

			wantedAppName:  "frontend",
			wantedEnvName:  "prod-iad",
			wantedImageTag: "latest",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockWs := climocks.NewMockwsAppReader(ctrl)
			mockStore := climocks.NewMockprojectService(ctrl)
			mockPrompt := climocks.NewMockprompter(ctrl)
			tc.mockWs(mockWs)
			tc.mockStore(mockStore)
			tc.mockPrompt(mockPrompt)

			opts := appDeployOpts{
				appDeployVars: appDeployVars{
					GlobalOpts: &GlobalOpts{
						projectName: tc.inProjectName,
						prompt:      mockPrompt,
					},
					AppName:  tc.inAppName,
					EnvName:  tc.inEnvName,
					ImageTag: tc.inImageTag,
				},
				workspaceService: mockWs,
				projectService:   mockStore,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError == nil {
				require.Nil(t, err)
				require.Equal(t, tc.wantedAppName, opts.AppName)
				require.Equal(t, tc.wantedEnvName, opts.EnvName)
				require.Equal(t, tc.wantedImageTag, opts.ImageTag)
			} else {
				require.EqualError(t, err, tc.wantedError.Error())
			}
		})
	}
}

func TestAppDeployOpts_getAppDockerfilePath(t *testing.T) {
	var mockWorkspace *climocks.MockwsAppReader

	mockError := errors.New("mockError")
	mockManifest := []byte(`name: appA
type: 'Load Balanced Web App'
image:
  build: appA/Dockerfile
`)

	tests := map[string]struct {
		inputApp   string
		setupMocks func(controller *gomock.Controller)

		wantPath string
		wantErr  error
	}{
		"should return error if workspaceService ReadFile returns error": {
			inputApp: "appA",
			setupMocks: func(controller *gomock.Controller) {
				mockWorkspace = climocks.NewMockwsAppReader(controller)

				gomock.InOrder(
					mockWorkspace.EXPECT().ReadAppManifest("appA").Times(1).Return(nil, mockError),
				)
			},
			wantPath: "",
			wantErr:  fmt.Errorf("read manifest file %s: %w", "appA", mockError),
		},
		"should trim the manifest DockerfilePath if it contains /Dockerfile": {
			inputApp: "appA",
			setupMocks: func(controller *gomock.Controller) {
				mockWorkspace = climocks.NewMockwsAppReader(controller)

				gomock.InOrder(
					mockWorkspace.EXPECT().ReadAppManifest("appA").Times(1).Return(mockManifest, nil),
				)
			},
			wantPath: "appA",
			wantErr:  nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			test.setupMocks(ctrl)
			opts := appDeployOpts{
				appDeployVars: appDeployVars{
					AppName: test.inputApp,
				},
				workspaceService: mockWorkspace,
			}

			gotPath, gotErr := opts.getAppDockerfilePath()

			require.Equal(t, test.wantPath, gotPath)
			require.Equal(t, test.wantErr, gotErr)
		})
	}
}

func TestAppDeployOpts_askImageTag(t *testing.T) {
	var mockRunner *climocks.Mockrunner
	var mockPrompter *climocks.Mockprompter

	mockError := errors.New("mockError")

	tests := map[string]struct {
		inputImageTag string

		setupMocks func(controller *gomock.Controller)

		wantErr      error
		wantImageTag string
	}{
		"should return nil if input image tag is not empty": {
			inputImageTag: "anythingreally",
			setupMocks:    func(controller *gomock.Controller) {},
			wantErr:       nil,
			wantImageTag:  "anythingreally",
		},
		"should wrap error from prompting": {
			inputImageTag: "",
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = climocks.NewMockrunner(controller)
				mockPrompter = climocks.NewMockprompter(controller)

				gomock.InOrder(
					mockRunner.EXPECT().Run("git", []string{"describe", "--always"}, gomock.Any()).Times(1).Return(mockError),
					mockPrompter.EXPECT().Get(inputImageTagPrompt, "", nil).Times(1).Return("", mockError),
				)
			},
			wantErr:      fmt.Errorf("prompt for image tag: %w", mockError),
			wantImageTag: "",
		},
		"should set opts imageTag to user input value": {
			inputImageTag: "",
			setupMocks: func(controller *gomock.Controller) {
				mockRunner = climocks.NewMockrunner(controller)
				mockPrompter = climocks.NewMockprompter(controller)

				gomock.InOrder(
					mockRunner.EXPECT().Run("git", []string{"describe", "--always"}, gomock.Any()).Times(1).Return(mockError),
					mockPrompter.EXPECT().Get(inputImageTagPrompt, "", nil).Times(1).Return("youwotm8", nil),
				)
			},
			wantErr:      nil,
			wantImageTag: "youwotm8",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			test.setupMocks(ctrl)
			opts := &appDeployOpts{
				appDeployVars: appDeployVars{
					GlobalOpts: &GlobalOpts{
						prompt: mockPrompter,
					},
					ImageTag: test.inputImageTag,
				},
				runner: mockRunner,
			}

			got := opts.askImageTag()

			require.Equal(t, test.wantErr, got)
			require.Equal(t, test.wantImageTag, opts.ImageTag)
		})
	}
}

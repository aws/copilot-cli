// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeleteAppOpts_Validate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProjectService := climocks.NewMockprojectService(ctrl)
	mockError := errors.New("some error")

	tests := map[string]struct {
		inProjectName string
		inAppName     string
		setupMocks    func()

		want error
	}{
		"should return errNoProjectInWorkspace": {
			setupMocks: func() {},
			inAppName:  "my-app",
			want:       errNoProjectInWorkspace,
		},
		"with no flag set": {
			inProjectName: "phonetool",
			setupMocks:    func() {},
			want:          nil,
		},
		"with all flag set": {
			inProjectName: "phonetool",
			inAppName:     "my-app",
			setupMocks: func() {
				mockProjectService.EXPECT().GetApplication("phonetool", "my-app").Times(1).Return(&archer.Application{
					Name: "my-app",
				}, nil)
			},
			want: nil,
		},
		"should return error if fail to get app name": {
			inProjectName: "phonetool",
			inAppName:     "my-app",
			setupMocks: func() {
				mockProjectService.EXPECT().GetApplication("phonetool", "my-app").Times(1).Return(nil, mockError)
			},
			want: errors.New("some error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test.setupMocks()
			opts := deleteAppOpts{
				deleteAppVars: deleteAppVars{
					GlobalOpts: &GlobalOpts{
						projectName: test.inProjectName,
					},
					AppName: test.inAppName,
				},
				projectService: mockProjectService,
			}

			got := opts.Validate()

			require.Equal(t, test.want, got)
		})
	}
}

func TestDeleteAppOpts_Ask(t *testing.T) {
	const (
		mockProjectName = "phonetool"
		testAppName     = "my-app"
		testProjectName = "my-project"
	)
	mockError := errors.New("mockError")

	tests := map[string]struct {
		skipConfirmation bool
		inAppName        string

		mockProjectService func(m *climocks.MockprojectService)
		mockPrompt         func(m *climocks.Mockprompter)

		wantedApp   string
		wantedError error
	}{
		"should ask for app name": {
			inAppName:        "",
			skipConfirmation: true,
			mockProjectService: func(m *climocks.MockprojectService) {
				m.EXPECT().ListApplications(mockProjectName).Return([]*archer.Application{
					&archer.Application{
						Name: "my-app",
					},
					&archer.Application{
						Name: "test-app",
					},
				}, nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(appDeleteNamePrompt, "", []string{"my-app", "test-app"}).Times(1).Return("my-app", nil)
			},

			wantedApp: testAppName,
		},
		"returns error if no application found": {
			inAppName:        "",
			skipConfirmation: true,
			mockProjectService: func(m *climocks.MockprojectService) {
				m.EXPECT().ListApplications(mockProjectName).Return([]*archer.Application{}, nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {},

			wantedError: fmt.Errorf("couldn't find any application in the project phonetool"),
		},
		"returns error if fail to select application": {
			inAppName:        "",
			skipConfirmation: true,
			mockProjectService: func(m *climocks.MockprojectService) {
				m.EXPECT().ListApplications(mockProjectName).Return([]*archer.Application{
					&archer.Application{
						Name: "my-app",
					},
					&archer.Application{
						Name: "test-app",
					},
				}, nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(appDeleteNamePrompt, "", []string{"my-app", "test-app"}).Times(1).Return("", mockError)
			},

			wantedError: fmt.Errorf("select application to delete: %w", mockError),
		},
		"should skip confirmation": {
			inAppName:          testAppName,
			skipConfirmation:   true,
			mockProjectService: func(m *climocks.MockprojectService) {},
			mockPrompt:         func(m *climocks.Mockprompter) {},

			wantedApp: testAppName,
		},
		"should wrap error returned from prompter confirmation": {
			inAppName:          testAppName,
			skipConfirmation:   false,
			mockProjectService: func(m *climocks.MockprojectService) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(appDeleteConfirmPrompt, testAppName, mockProjectName),
					appDeleteConfirmHelp,
				).Times(1).Return(true, mockError)
			},

			wantedError: fmt.Errorf("app delete confirmation prompt: %w", mockError),
		},
		"should return error if user does not confirm app deletion": {
			inAppName:          testAppName,
			skipConfirmation:   false,
			mockProjectService: func(m *climocks.MockprojectService) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(appDeleteConfirmPrompt, testAppName, mockProjectName),
					appDeleteConfirmHelp,
				).Times(1).Return(false, nil)
			},

			wantedError: errAppDeleteCancelled,
		},
		"should return error nil if user confirms app delete": {
			inAppName:          testAppName,
			skipConfirmation:   false,
			mockProjectService: func(m *climocks.MockprojectService) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(appDeleteConfirmPrompt, testAppName, mockProjectName),
					appDeleteConfirmHelp,
				).Times(1).Return(true, nil)
			},

			wantedApp: testAppName,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompter := climocks.NewMockprompter(ctrl)
			mockProjectService := climocks.NewMockprojectService(ctrl)
			test.mockPrompt(mockPrompter)
			test.mockProjectService(mockProjectService)

			opts := deleteAppOpts{
				deleteAppVars: deleteAppVars{
					SkipConfirmation: test.skipConfirmation,
					GlobalOpts: &GlobalOpts{
						projectName: mockProjectName,
						prompt:      mockPrompter,
					},
					AppName: test.inAppName,
				},
				projectService: mockProjectService,
			}

			got := opts.Ask()

			if got != nil {
				require.Equal(t, test.wantedError, got)
			} else {
				require.Equal(t, test.wantedApp, opts.AppName)
			}
		})
	}
}

func TestDeleteAppOpts_sourceProjectEnvironments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProjectService := climocks.NewMockprojectService(ctrl)
	mockProjectName := "mockProjectName"
	mockError := errors.New("mockError")
	mockEnvList := []*archer.Environment{
		&archer.Environment{Project: mockProjectName},
	}

	tests := map[string]struct {
		setupMocks func()

		want            error
		wantOptsEnvList []*archer.Environment
	}{
		"should wrap error returned from call to ListEnvironments()": {
			setupMocks: func() {
				mockProjectService.EXPECT().ListEnvironments(gomock.Eq(mockProjectName)).Times(1).Return(nil, mockError)
			},
			want:            fmt.Errorf("get environments: %w", mockError),
			wantOptsEnvList: nil,
		},
		"should set the opts environment list": {
			setupMocks: func() {
				mockProjectService.EXPECT().ListEnvironments(gomock.Eq(mockProjectName)).Times(1).Return(mockEnvList, nil)
			},
			want:            nil,
			wantOptsEnvList: mockEnvList,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test.setupMocks()
			opts := deleteAppOpts{
				deleteAppVars: deleteAppVars{
					GlobalOpts: &GlobalOpts{
						projectName: mockProjectName,
					},
				},
				projectService: mockProjectService,
			}

			got := opts.sourceProjectEnvironments()

			require.Equal(t, test.want, got)
			require.Equal(t, test.wantOptsEnvList, opts.projectEnvironments)
		})
	}
}

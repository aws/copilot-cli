// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/mocks"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestConfirmDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := climocks.NewMockprompter(ctrl)

	mockAppName := "mockAppName"
	mockProjectName := "mockProjectName"
	mockError := errors.New("mockError")

	tests := map[string]struct {
		skipConfirmation bool

		setupMocks func()

		want error
	}{
		"should skip confirmation": {
			skipConfirmation: true,
			setupMocks:       func() {},
			want:             nil,
		},
		"should wrap error returned from prompter confirmation": {
			skipConfirmation: false,
			setupMocks: func() {
				mockPrompter.EXPECT().Confirm(
					fmt.Sprintf(appDeleteConfirmPrompt, mockAppName, mockProjectName),
					appDeleteConfirmHelp,
				).Times(1).Return(true, mockError)
			},
			want: fmt.Errorf("app delete confirmation prompt: %w", mockError),
		},
		"should return error if user does not confirm app deletion": {
			skipConfirmation: false,
			setupMocks: func() {
				mockPrompter.EXPECT().Confirm(
					fmt.Sprintf(appDeleteConfirmPrompt, mockAppName, mockProjectName),
					appDeleteConfirmHelp,
				).Times(1).Return(false, nil)
			},
			want: errAppDeleteCancelled,
		},
		"should return error nil if user confirms app delete": {
			skipConfirmation: false,
			setupMocks: func() {
				mockPrompter.EXPECT().Confirm(
					fmt.Sprintf(appDeleteConfirmPrompt, mockAppName, mockProjectName),
					appDeleteConfirmHelp,
				).Times(1).Return(true, nil)
			},
			want: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test.setupMocks()
			opts := deleteAppOpts{
				GlobalOpts: &GlobalOpts{
					projectName: mockProjectName,
				},
				app:              mockAppName,
				skipConfirmation: test.skipConfirmation,
				prompter:         mockPrompter,
			}

			got := opts.confirmDelete()

			require.Equal(t, test.want, got)
		})
	}
}

func TestSourceInputs(t *testing.T) {
	t.Run("should return errNoProjectInWorkspace", func(t *testing.T) {
		opts := deleteAppOpts{
			GlobalOpts: &GlobalOpts{
				projectName: "",
			},
		}

		got := opts.sourceInputs()

		require.Equal(t, errNoProjectInWorkspace, got)
	})
}

func TestSourceWorkspaceApplications(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWorkspaceService := mocks.NewMockWorkspace(ctrl)
	mockError := errors.New("mockError")

	tests := map[string]struct {
		setupMocks func()

		want error
	}{
		"should wrap error returned from workspaceService Apps() call": {
			setupMocks: func() {
				mockWorkspaceService.EXPECT().Apps().Times(1).Return(nil, mockError)
			},
			want: fmt.Errorf("get app names: %w", mockError),
		},
		"should return error if call to Apps() returns empty list": {
			setupMocks: func() {
				mockWorkspaceService.EXPECT().Apps().Times(1).Return([]archer.Manifest{}, nil)
			},
			want: errors.New("no applications found"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test.setupMocks()
			opts := deleteAppOpts{
				workspaceService: mockWorkspaceService,
			}

			got := opts.sourceWorkspaceApplications()

			require.Equal(t, test.want, got)
		})
	}
}

func TestAppDeleteSourceProjectEnvironments(t *testing.T) {
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
		"should return error if call to ListEnvironments() returns an empty list": {
			setupMocks: func() {
				mockProjectService.EXPECT().ListEnvironments(gomock.Eq(mockProjectName)).Times(1).Return([]*archer.Environment{}, nil)
			},
			want:            errors.New("no environments found"),
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
				GlobalOpts: &GlobalOpts{
					projectName: mockProjectName,
				},
				projectService: mockProjectService,
			}

			got := opts.sourceProjectEnvironments()

			require.Equal(t, test.want, got)
			require.Equal(t, test.wantOptsEnvList, opts.projectEnvironments)
		})
	}
}

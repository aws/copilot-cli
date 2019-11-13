// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
)

func TestSourceProjectApplications(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWorkspaceService := mocks.NewMockWorkspace(ctrl)

	mockProjectName := "mockProjectName"
	mockError := errors.New("error")
	mockAppNames := []string{
		mockProjectName,
	}

	testCases := map[string]struct {
		setupMocks func()

		wantErr      error
		wantAppNames []string
	}{
		"should wrap error returned from ListApplications": {
			setupMocks: func() {
				mockWorkspaceService.EXPECT().AppNames().Times(1).Return([]string{}, mockError)
			},
			wantErr: fmt.Errorf("get app names: %w", mockError),
		},
		"should return error given no apps returned": {
			setupMocks: func() {
				mockWorkspaceService.EXPECT().AppNames().Times(1).Return([]string{}, nil)
			},
			wantErr: errors.New("no applications found"),
		},
		"should set opts projectApplications field": {
			setupMocks: func() {
				mockWorkspaceService.EXPECT().AppNames().Times(1).Return(mockAppNames, nil)
			},
			wantAppNames: mockAppNames,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.setupMocks()

			opts := appDeployOpts{
				GlobalOpts: &GlobalOpts{
					projectName: mockProjectName,
				},
				workspaceService: mockWorkspaceService,
			}

			gotErr := opts.sourceProjectApplications()

			require.Equal(t, tc.wantErr, gotErr)
			require.Equal(t, tc.wantAppNames, opts.localProjectAppNames)
		})
	}
}

func TestSourceProjectEnvironments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProjectService := climocks.NewMockprojectService(ctrl)

	mockProjectName := "mockProjectName"
	mockError := errors.New("error")
	mockEnvs := []*archer.Environment{
		&archer.Environment{
			Project: mockProjectName,
		},
	}

	testCases := map[string]struct {
		setupMocks func()

		wantErr  error
		wantEnvs []*archer.Environment
	}{
		"should wrap error returned from ListEnvironments": {
			setupMocks: func() {
				mockProjectService.EXPECT().ListEnvironments(gomock.Eq(mockProjectName)).Times(1).Return(nil, mockError)
			},
			wantErr: fmt.Errorf("get environments: %w", mockError),
		},
		"should return error given no environments returned": {
			setupMocks: func() {
				mockProjectService.EXPECT().ListEnvironments(gomock.Eq(mockProjectName)).Times(1).Return([]*archer.Environment{}, nil)
			},
			wantErr: errors.New("no environments found"),
		},
		"should set the opts projectEnvironments field": {
			setupMocks: func() {
				mockProjectService.EXPECT().ListEnvironments(gomock.Eq(mockProjectName)).Times(1).Return(mockEnvs, nil)
			},
			wantEnvs: mockEnvs,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.setupMocks()

			opts := appDeployOpts{
				GlobalOpts: &GlobalOpts{
					projectName: mockProjectName,
				},
				projectService: mockProjectService,
			}

			gotErr := opts.sourceProjectEnvironments()

			require.Equal(t, tc.wantErr, gotErr)
			require.Equal(t, tc.wantEnvs, opts.projectEnvironments)
		})
	}
}

func TestSourceAppName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := climocks.NewMockprompter(ctrl)

	mockAppName := "myapp"

	testCases := map[string]struct {
		setupMocks func()

		inputAppFlag         string
		localProjectAppNames []string

		wantAppName string
		wantErr     error
	}{
		"should validate the input app flag name": {
			setupMocks:           func() {},
			inputAppFlag:         mockAppName,
			localProjectAppNames: []string{mockAppName},
			wantAppName:          mockAppName,
		},
		"should default the app name if there's only one option": {
			setupMocks:           func() {},
			localProjectAppNames: []string{mockAppName},
			wantAppName:          mockAppName,
		},
		"should prompt the user to select an app if there's multiple options": {
			setupMocks: func() {
				mockPrompt.EXPECT().
					SelectOne(gomock.Eq("Select an application"), gomock.Eq(""), gomock.Eq([]string{mockAppName, "anotherone"})).
					Times(1).
					Return(mockAppName, nil)
			},
			localProjectAppNames: []string{mockAppName, "anotherone"},
			wantAppName:          mockAppName,
		},
		"should return error if flag input value is not valid": {
			setupMocks:           func() {},
			inputAppFlag:         mockAppName,
			localProjectAppNames: []string{"anotherone"},
			wantErr:              fmt.Errorf("invalid app name: %s", mockAppName),
			wantAppName:          mockAppName,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.setupMocks()

			opts := appDeployOpts{
				app:                  tc.inputAppFlag,
				localProjectAppNames: tc.localProjectAppNames,
				GlobalOpts: &GlobalOpts{
					prompt: mockPrompt,
				},
			}

			gotErr := opts.sourceAppName()

			require.Equal(t, tc.wantErr, gotErr)
			require.Equal(t, tc.wantAppName, opts.app)
		})
	}
}

func TestSourceEnvName(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := climocks.NewMockprompter(ctrl)

	mockEnvName := "test"

	testCases := map[string]struct {
		setupMocks func()

		inputEnvFlag        string
		projectEnvironments []*archer.Environment

		wantEnvName string
		wantErr     error
	}{
		"should validate the input env flag name": {
			setupMocks:   func() {},
			inputEnvFlag: mockEnvName,
			projectEnvironments: []*archer.Environment{
				&archer.Environment{
					Name: mockEnvName,
				},
			},
			wantEnvName: mockEnvName,
		},
		"should default the env name if there's only one option": {
			setupMocks: func() {},
			projectEnvironments: []*archer.Environment{
				&archer.Environment{
					Name: mockEnvName,
				},
			},
			wantEnvName: mockEnvName,
		},
		"should prompt the user to select an env if there's multiple options": {
			setupMocks: func() {
				mockPrompt.EXPECT().
					SelectOne(gomock.Eq("Select an environment"), gomock.Eq(""), gomock.Eq([]string{mockEnvName, "anotherone"})).
					Times(1).
					Return(mockEnvName, nil)
			},
			projectEnvironments: []*archer.Environment{
				&archer.Environment{
					Name: mockEnvName,
				},
				&archer.Environment{
					Name: "anotherone",
				},
			},
			wantEnvName: mockEnvName,
		},
		"should return error if flag input value is not valid": {
			setupMocks:   func() {},
			inputEnvFlag: mockEnvName,
			projectEnvironments: []*archer.Environment{
				&archer.Environment{
					Name: "anotherone",
				},
			},
			wantErr:     fmt.Errorf("invalid env name: %s", mockEnvName),
			wantEnvName: mockEnvName,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.setupMocks()

			opts := appDeployOpts{
				env:                 tc.inputEnvFlag,
				projectEnvironments: tc.projectEnvironments,
				GlobalOpts: &GlobalOpts{
					prompt: mockPrompt,
				},
			}

			gotErr := opts.sourceEnvName()

			require.Equal(t, tc.wantErr, gotErr)
			require.Equal(t, tc.wantEnvName, opts.env)
		})
	}
}

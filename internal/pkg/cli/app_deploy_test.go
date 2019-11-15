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
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/build/ecr"
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

func TestSourceTargetEnv(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompt := climocks.NewMockprompter(ctrl)

	mockEnvName := "mockEnvName"
	mockEnv := &archer.Environment{
		Name: mockEnvName,
	}

	testCases := map[string]struct {
		setupMocks func()

		inputEnvFlag        string
		projectEnvironments []*archer.Environment

		wantErr error
	}{
		"should validate the input env flag name": {
			setupMocks:          func() {},
			inputEnvFlag:        mockEnvName,
			projectEnvironments: []*archer.Environment{mockEnv},
		},
		"should default the env name if there's only one option": {
			setupMocks:          func() {},
			projectEnvironments: []*archer.Environment{mockEnv},
		},
		"should prompt the user to select an env if there's multiple options": {
			setupMocks: func() {
				mockPrompt.EXPECT().
					SelectOne(gomock.Eq("Select an environment"), gomock.Eq(""), gomock.Eq([]string{mockEnv.Name, "anotherone"})).
					Times(1).
					Return(mockEnvName, nil)
			},
			projectEnvironments: []*archer.Environment{
				mockEnv,
				&archer.Environment{
					Name: "anotherone",
				},
			},
		},
		"should return error if flag input value is not valid": {
			setupMocks:   func() {},
			inputEnvFlag: mockEnvName,
			projectEnvironments: []*archer.Environment{
				&archer.Environment{
					Name: "anotherone",
				},
			},
			wantErr: fmt.Errorf("invalid env name: %s", mockEnvName),
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

			gotErr := opts.sourceTargetEnv()

			require.Equal(t, tc.wantErr, gotErr)
		})
	}
}

type mockECRService struct {
	t *testing.T

	mockGetRepository func(t *testing.T, name string) (string, error)
	mockGetECRAuth    func() (ecr.Auth, error)
}

func (m mockECRService) GetRepository(name string) (string, error) {
	return m.mockGetRepository(m.t, name)
}

func (m mockECRService) GetECRAuth() (ecr.Auth, error) {
	return m.GetECRAuth()
}

// TODO: expand on test suite once workspace/manifest and docker commands are more mockable
func TestDeployApp(t *testing.T) {
	mockProjectName := "mockProjectName"
	mockApp := "mockApp"
	mockError := errors.New("mockError")

	tests := map[string]struct {
		projectName string
		app         string

		mockGetRepository func(t *testing.T, name string) (string, error)

		want error
	}{
		"wrap error returned from ECR GetRepository": {
			projectName: mockProjectName,
			app:         mockApp,

			mockGetRepository: func(t *testing.T, name string) (string, error) {
				require.Equal(t, fmt.Sprintf("%s/%s", mockProjectName, mockApp), name)

				return "", mockError
			},

			want: fmt.Errorf("get ECR repository URI: %w", mockError),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			opts := appDeployOpts{
				GlobalOpts: &GlobalOpts{
					projectName: test.projectName,
				},
				app: test.app,
				ecrService: mockECRService{
					t:                 t,
					mockGetRepository: test.mockGetRepository,
				},
			}

			got := opts.deployApp()

			require.Equal(t, test.want, got)
		})
	}
}

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestInitProjectOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inProjectName string
		expect        func(opts *InitProjectOpts)

		wantedProjectName string
		wantedErr         string
	}{
		"do nothing if name is provided": {
			inProjectName: "metrics",
			expect:        func(opts *InitProjectOpts) {},

			wantedProjectName: "metrics",
		},
		"set the project name workspace.Summary": {
			expect: func(opts *InitProjectOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(&archer.WorkspaceSummary{ProjectName: "metrics"}, nil)
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Times(0)
			},
			wantedProjectName: "metrics",
		},
		"return error from new project name": {
			expect: func(opts *InitProjectOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Return([]*archer.Project{}, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("my error"))
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "prompt get project name: my error",
		},
		"enter new project name if no existing projects": {
			expect: func(opts *InitProjectOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Return([]*archer.Project{}, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedProjectName: "metrics",
		},
		"return error from project selection": {
			expect: func(opts *InitProjectOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Return([]*archer.Project{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("my error"))
			},
			wantedErr: "prompt select project name: my error",
		},
		"use existing projects": {
			expect: func(opts *InitProjectOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Return([]*archer.Project{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
			},
			wantedProjectName: "metrics",
		},
		"enter new project name if user opts out of selection": {
			expect: func(opts *InitProjectOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Return([]*archer.Project{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
			},
			wantedProjectName: "metrics",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			opts := &InitProjectOpts{
				ProjectName:  tc.inProjectName,
				projectStore: mocks.NewMockProjectStore(ctrl),
				ws:           mocks.NewMockWorkspace(ctrl),
				prompt:       climocks.NewMockprompter(ctrl),
			}
			tc.expect(opts)

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.Nil(t, err)
			}
			require.Equal(t, tc.wantedProjectName, opts.ProjectName)
		})
	}
}

func TestInitProjectOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inProjectName string
		wantedError   error
	}{
		"valid project name": {
			inProjectName: "metrics",
		},
		"invalid project name": {
			inProjectName: "123chicken",
			wantedError:   errValueBadFormat,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &InitProjectOpts{
				ProjectName: tc.inProjectName,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			require.True(t, errors.Is(err, tc.wantedError))
		})
	}
}

func TestInitProjectOpts_Execute(t *testing.T) {
	mockError := fmt.Errorf("error")

	testCases := map[string]struct {
		expectedProject archer.Project
		expectedError   error
		mocking         func(t *testing.T, mockProjectStore *mocks.MockProjectStore, mockWorkspace *mocks.MockWorkspace)
	}{
		"with a succesfull call to add env": {
			expectedProject: archer.Project{
				Name: "project",
			},
			mocking: func(t *testing.T, mockProjectStore *mocks.MockProjectStore, mockWorkspace *mocks.MockWorkspace) {
				mockProjectStore.
					EXPECT().
					CreateProject(gomock.Any()).
					Do(func(project *archer.Project) {
						require.Equal(t, project.Name, "project")
					})
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("project"))
			},
		},

		"should ignore ErrProjectAlreadyExists from CreateProject": {
			expectedProject: archer.Project{
				Name: "project",
			},
			mocking: func(t *testing.T, mockProjectStore *mocks.MockProjectStore, mockWorkspace *mocks.MockWorkspace) {
				mockProjectStore.
					EXPECT().
					CreateProject(gomock.Any()).
					Do(func(project *archer.Project) {
						require.Equal(t, project.Name, "project")
					}).
					Return(&store.ErrProjectAlreadyExists{ProjectName: "project"})
				mockWorkspace.
					EXPECT().
					Create(gomock.Any())
			},
		},

		"should return error from CreateProject": {
			expectedError: mockError,
			mocking: func(t *testing.T, mockProjectStore *mocks.MockProjectStore, mockWorkspace *mocks.MockWorkspace) {
				mockProjectStore.
					EXPECT().
					CreateProject(gomock.Any()).
					Return(mockError)
				mockWorkspace.
					EXPECT().
					Create(gomock.Any()).
					Times(0)
			},
		},

		"should return error from workspace.Create": {
			expectedError: mockError,
			mocking: func(t *testing.T, mockProjectStore *mocks.MockProjectStore, mockWorkspace *mocks.MockWorkspace) {
				mockProjectStore.
					EXPECT().
					CreateProject(gomock.Any()).
					Return(nil)
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("project")).
					Return(mockError)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockProjectStore := mocks.NewMockProjectStore(ctrl)
			mockWorkspace := mocks.NewMockWorkspace(ctrl)

			opts := &InitProjectOpts{
				ProjectName:  "project",
				projectStore: mockProjectStore,
				ws:           mockWorkspace,
			}
			tc.mocking(t, mockProjectStore, mockWorkspace)

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.expectedError == nil {
				require.NoError(t, err)
			} else {
				require.True(t, errors.Is(err, tc.expectedError))
			}
		})
	}
}

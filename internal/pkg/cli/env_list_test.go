// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
	climocks "github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/cli/mocks"
	"github.com/aws/PRIVATE-amazon-ecs-archer/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestEnvList_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockError := fmt.Errorf("error")
	mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
	mockProjectStore := mocks.NewMockProjectStore(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		listOpts    ListEnvOpts
		mocking     func()
		expectedErr error
	}{
		"with envs": {
			listOpts: ListEnvOpts{
				ProjectName:   "coolproject",
				manager:       mockEnvStore,
				projectGetter: mockProjectStore,
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(&archer.Project{}, nil)
				mockEnvStore.
					EXPECT().
					ListEnvironments(gomock.Eq("coolproject")).
					Return([]*archer.Environment{
						&archer.Environment{Name: "test"},
						&archer.Environment{Name: "test2"},
					}, nil)

			},
		},
		"with invalid project name": {
			expectedErr: mockError,
			listOpts: ListEnvOpts{
				ProjectName:   "coolproject",
				manager:       mockEnvStore,
				projectGetter: mockProjectStore,
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(nil, mockError)

				mockEnvStore.
					EXPECT().
					ListEnvironments(gomock.Eq("coolproject")).
					Times(0)
			},
		},
		"with failed call to list": {
			expectedErr: mockError,
			listOpts: ListEnvOpts{
				ProjectName:   "coolproject",
				manager:       mockEnvStore,
				projectGetter: mockProjectStore,
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(&archer.Project{}, nil)

				mockEnvStore.
					EXPECT().
					ListEnvironments(gomock.Eq("coolproject")).
					Return(nil, mockError)
			},
		},
		"with production envs": {
			listOpts: ListEnvOpts{
				ProjectName:   "coolproject",
				manager:       mockEnvStore,
				projectGetter: mockProjectStore,
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(&archer.Project{}, nil)
				mockEnvStore.
					EXPECT().
					ListEnvironments(gomock.Eq("coolproject")).
					Return([]*archer.Environment{
						&archer.Environment{Name: "test"},
						&archer.Environment{Name: "test2", Prod: true},
					}, nil)

			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()
			tc.listOpts.w = ioutil.Discard

			err := tc.listOpts.Execute()

			if tc.expectedErr != nil {
				require.EqualError(t, tc.expectedErr, err.Error())
			}
		})
	}
}

func TestEnvList_Ask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPrompter := climocks.NewMockprompter(ctrl)

	testCases := map[string]struct {
		inputEnv     string
		inputProject string

		setupMocks func()

		wantedProject string
	}{
		"with no flags set": {
			setupMocks: func() {
				mockPrompter.EXPECT().
					Get(gomock.Eq("Which project's environments would you like to list?"),
						gomock.Eq("A project groups all of your environments together."),
						gomock.Any()).
					Return("project", nil).
					Times(1)
			},
			wantedProject: "project",
		},
		"with env flags set": {
			setupMocks:    func() {},
			inputProject:  "project",
			wantedProject: "project",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			listEnvs := &ListEnvOpts{
				ProjectName: tc.inputProject,
				prompter:    mockPrompter,
			}
			tc.setupMocks()

			err := listEnvs.Ask()

			require.NoError(t, err)
			require.Equal(t, tc.wantedProject, listEnvs.ProjectName, "expected project names to match")
		})
	}
}

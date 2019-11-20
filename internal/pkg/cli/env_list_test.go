// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
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
		listOpts        ListEnvOpts
		mocking         func()
		expectedErr     error
		expectedContent string
	}{
		"with json envs": {
			listOpts: ListEnvOpts{
				ShouldOutputJSON: true,
				manager:          mockEnvStore,
				projectGetter:    mockProjectStore,
				GlobalOpts: &GlobalOpts{
					projectName: "coolproject",
				},
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(&archer.Project{}, nil)
				mockEnvStore.
					EXPECT().
					ListEnvironments(gomock.Eq("coolproject")).
					Return([]*archer.Environment{
						{Name: "test"},
						{Name: "test2"},
					}, nil)
			},
			expectedContent: `{"environments":[{"project":"","name":"test","region":"","accountID":"","prod":false,"registryURL":"","managerRoleARN":""},{"project":"","name":"test2","region":"","accountID":"","prod":false,"registryURL":"","managerRoleARN":""}]}` + "\n",
		},
		"with envs": {
			listOpts: ListEnvOpts{
				manager:       mockEnvStore,
				projectGetter: mockProjectStore,
				GlobalOpts: &GlobalOpts{
					projectName: "coolproject",
				},
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(&archer.Project{}, nil)
				mockEnvStore.
					EXPECT().
					ListEnvironments(gomock.Eq("coolproject")).
					Return([]*archer.Environment{
						{Name: "test"},
						{Name: "test2"},
					}, nil)
			},
			expectedContent: "test\ntest2\n",
		},
		"with invalid project name": {
			expectedErr: mockError,
			listOpts: ListEnvOpts{
				manager:       mockEnvStore,
				projectGetter: mockProjectStore,
				GlobalOpts: &GlobalOpts{
					projectName: "coolproject",
				},
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
				manager:       mockEnvStore,
				projectGetter: mockProjectStore,
				GlobalOpts: &GlobalOpts{
					projectName: "coolproject",
				},
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
				manager:       mockEnvStore,
				projectGetter: mockProjectStore,
				GlobalOpts: &GlobalOpts{
					projectName: "coolproject",
				},
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(&archer.Project{}, nil)
				mockEnvStore.
					EXPECT().
					ListEnvironments(gomock.Eq("coolproject")).
					Return([]*archer.Environment{
						{Name: "test"},
						{Name: "test2", Prod: true},
					}, nil)
			},
			expectedContent: "test\ntest2 (prod)\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			b := &bytes.Buffer{}
			tc.mocking()
			tc.listOpts.w = b
			err := tc.listOpts.Execute()

			if tc.expectedErr != nil {
				require.EqualError(t, tc.expectedErr, err.Error())
			} else {
				require.Equal(t, tc.expectedContent, b.String())
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
				GlobalOpts: &GlobalOpts{
					prompt:      mockPrompter,
					projectName: tc.inputProject,
				},
			}
			tc.setupMocks()

			err := listEnvs.Ask()

			require.NoError(t, err)
			require.Equal(t, tc.wantedProject, listEnvs.ProjectName(), "expected project names to match")
		})
	}
}

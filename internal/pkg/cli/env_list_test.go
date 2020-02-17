// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer/mocks"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
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
		listOpts        listEnvOpts
		mocking         func()
		expectedErr     error
		expectedContent string
	}{
		"with json envs": {
			listOpts: listEnvOpts{
				listEnvVars: listEnvVars{
					ShouldOutputJSON: true,
					GlobalOpts: &GlobalOpts{
						projectName: "coolproject",
					},
				},
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
						{Name: "test"},
						{Name: "test2"},
					}, nil)
			},
			expectedContent: `{"environments":[{"project":"","name":"test","region":"","accountID":"","prod":false,"registryURL":"","executionRoleARN":"","managerRoleARN":""},{"project":"","name":"test2","region":"","accountID":"","prod":false,"registryURL":"","executionRoleARN":"","managerRoleARN":""}]}` + "\n",
		},
		"with envs": {
			listOpts: listEnvOpts{
				listEnvVars: listEnvVars{
					GlobalOpts: &GlobalOpts{
						projectName: "coolproject",
					},
				},
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
						{Name: "test"},
						{Name: "test2"},
					}, nil)
			},
			expectedContent: "test\ntest2\n",
		},
		"with invalid project name": {
			expectedErr: mockError,
			listOpts: listEnvOpts{
				listEnvVars: listEnvVars{
					GlobalOpts: &GlobalOpts{
						projectName: "coolproject",
					},
				},
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
			listOpts: listEnvOpts{
				listEnvVars: listEnvVars{
					GlobalOpts: &GlobalOpts{
						projectName: "coolproject",
					},
				},
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
			listOpts: listEnvOpts{
				listEnvVars: listEnvVars{
					GlobalOpts: &GlobalOpts{
						projectName: "coolproject",
					},
				},
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
	testCases := map[string]struct {
		inputProject string

		mockProjectLister func(m *mocks.MockProjectLister)
		mockPrompt        func(m *climocks.Mockprompter)

		wantedProject string
	}{
		"with no flags set": {
			mockProjectLister: func(m *mocks.MockProjectLister) {
				m.EXPECT().ListProjects().Return([]*archer.Project{
					&archer.Project{Name: "my-project"},
					&archer.Project{Name: "archer-project"},
				}, nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(environmentListProjectNamePrompt, environmentListProjectNameHelper, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1)
			},
			wantedProject: "my-project",
		},
		"with env flags set": {
			mockProjectLister: func(m *mocks.MockProjectLister) {},
			mockPrompt:        func(m *climocks.Mockprompter) {},
			inputProject:      "my-project",
			wantedProject:     "my-project",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockProjectLister := mocks.NewMockProjectLister(ctrl)
			mockPrompter := climocks.NewMockprompter(ctrl)
			tc.mockProjectLister(mockProjectLister)
			tc.mockPrompt(mockPrompter)

			listEnvs := &listEnvOpts{
				listEnvVars: listEnvVars{
					GlobalOpts: &GlobalOpts{
						prompt:      mockPrompter,
						projectName: tc.inputProject,
					},
				},
				projectLister: mockProjectLister,
			}

			err := listEnvs.Ask()

			require.NoError(t, err)
			require.Equal(t, tc.wantedProject, listEnvs.ProjectName(), "expected project names to match")
		})
	}
}

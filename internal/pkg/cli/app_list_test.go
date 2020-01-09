// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestAppList_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockError := fmt.Errorf("error")
	mockAppStore := mocks.NewMockApplicationStore(ctrl)
	mockProjectStore := mocks.NewMockProjectStore(ctrl)
	mockWorkspace := mocks.NewMockWorkspace(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		listOpts        listAppOpts
		mocking         func()
		expectedErr     error
		expectedContent string
	}{
		"with json outputs": {
			listOpts: listAppOpts{
				ShouldOutputJSON: true,
				appLister:        mockAppStore,
				projectGetter:    mockProjectStore,
				GlobalOpts: &GlobalOpts{
					projectName: "coolproject",
				},
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(&archer.Project{}, nil)
				mockAppStore.
					EXPECT().
					ListApplications(gomock.Eq("coolproject")).
					Return([]*archer.Application{
						{Name: "my-app"},
						{Name: "lb-app"},
					}, nil)
			},
			expectedContent: `{"applications":[{"project":"","name":"my-app","type":""},{"project":"","name":"lb-app","type":""}]}` + "\n",
		},
		"with human outputs": {
			listOpts: listAppOpts{
				appLister:     mockAppStore,
				projectGetter: mockProjectStore,
				GlobalOpts: &GlobalOpts{
					projectName: "coolproject",
				},
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(&archer.Project{}, nil)
				mockAppStore.
					EXPECT().
					ListApplications(gomock.Eq("coolproject")).
					Return([]*archer.Application{
						{Name: "my-app", Type: "Load Balanced Web App"},
						{Name: "lb-app", Type: "Load Balanced Web App"},
					}, nil)
			},
			expectedContent: "Name                Type\n------              ---------------------\nmy-app              Load Balanced Web App\nlb-app              Load Balanced Web App\n",
		},
		"with invalid project name": {
			expectedErr: mockError,
			listOpts: listAppOpts{
				appLister:     mockAppStore,
				projectGetter: mockProjectStore,
				GlobalOpts: &GlobalOpts{
					projectName: "coolproject",
				},
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(nil, mockError)

				mockAppStore.
					EXPECT().
					ListApplications(gomock.Eq("coolproject")).
					Times(0)
			},
		},
		"with failed call to list": {
			expectedErr: mockError,
			listOpts: listAppOpts{
				appLister:     mockAppStore,
				projectGetter: mockProjectStore,
				GlobalOpts: &GlobalOpts{
					projectName: "coolproject",
				},
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(&archer.Project{}, nil)

				mockAppStore.
					EXPECT().
					ListApplications(gomock.Eq("coolproject")).
					Return(nil, mockError)
			},
		},
		"with local flag enabled": {
			expectedErr: nil,
			listOpts: listAppOpts{
				appLister:           mockAppStore,
				projectGetter:       mockProjectStore,
				ws:                  mockWorkspace,
				ShouldShowLocalApps: true,
				GlobalOpts: &GlobalOpts{
					projectName: "coolproject",
				},
			},
			mocking: func() {
				mockProjectStore.EXPECT().
					GetProject(gomock.Eq("coolproject")).
					Return(&archer.Project{}, nil)
				mockAppStore.
					EXPECT().
					ListApplications(gomock.Eq("coolproject")).
					Return([]*archer.Application{
						{Name: "my-app", Type: "Load Balanced Web App"},
						{Name: "lb-app", Type: "Load Balanced Web App"},
					}, nil)
				mockWorkspace.EXPECT().Apps().
					Return([]archer.Manifest{
						&manifest.LBFargateManifest{
							AppManifest: manifest.AppManifest{
								Name: "my-app",
								Type: "Load Balanced Web App",
							},
						}}, nil).Times(1)
			},
			expectedContent: "Name                Type\n------              ---------------------\nmy-app              Load Balanced Web App\n",
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

func TestAppList_Ask(t *testing.T) {
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
				m.EXPECT().SelectOne(applicationListProjectNamePrompt, applicationListProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1)
			},
			wantedProject: "my-project",
		},
		"with project flags set": {
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

			listApps := &listAppOpts{
				projectLister: mockProjectLister,
				GlobalOpts: &GlobalOpts{
					prompt:      mockPrompter,
					projectName: tc.inputProject,
				},
			}

			err := listApps.Ask()

			require.NoError(t, err)
			require.Equal(t, tc.wantedProject, listApps.ProjectName(), "expected project names to match")
		})
	}
}

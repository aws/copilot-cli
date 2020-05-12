// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestAppList_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockError := fmt.Errorf("error")
	mockstore := mocks.NewMockstore(ctrl)
	mockWorkspace := mocks.NewMockwsSvcReader(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		listOpts        listAppOpts
		mocking         func()
		expectedErr     error
		expectedContent string
	}{
		"with json outputs": {
			listOpts: listAppOpts{
				listAppVars: listAppVars{
					ShouldOutputJSON: true,
					GlobalOpts: &GlobalOpts{
						appName: "coolproject",
					},
				},
				store: mockstore,
			},
			mocking: func() {
				mockstore.EXPECT().
					GetApplication(gomock.Eq("coolproject")).
					Return(&config.Application{}, nil)
				mockstore.
					EXPECT().
					ListServices(gomock.Eq("coolproject")).
					Return([]*config.Service{
						{Name: "my-app"},
						{Name: "lb-app"},
					}, nil)
			},
			expectedContent: "{\"applications\":[{\"App\":\"\",\"name\":\"my-app\",\"type\":\"\"},{\"App\":\"\",\"name\":\"lb-app\",\"type\":\"\"}]}\n",
		},
		"with human outputs": {
			listOpts: listAppOpts{
				listAppVars: listAppVars{
					GlobalOpts: &GlobalOpts{
						appName: "coolproject",
					},
				},
				store: mockstore,
			},
			mocking: func() {
				mockstore.EXPECT().
					GetApplication(gomock.Eq("coolproject")).
					Return(&config.Application{}, nil)
				mockstore.
					EXPECT().
					ListServices(gomock.Eq("coolproject")).
					Return([]*config.Service{
						{Name: "my-app", Type: "Load Balanced Web Service"},
						{Name: "lb-app", Type: "Load Balanced Web Service"},
					}, nil)
			},
			expectedContent: "Name                Type\n------              -------------------------\nmy-app              Load Balanced Web Service\nlb-app              Load Balanced Web Service\n",
		},
		"with invalid project name": {
			expectedErr: mockError,
			listOpts: listAppOpts{
				listAppVars: listAppVars{
					GlobalOpts: &GlobalOpts{
						appName: "coolproject",
					},
				},
				store: mockstore,
			},
			mocking: func() {
				mockstore.EXPECT().
					GetApplication(gomock.Eq("coolproject")).
					Return(nil, mockError)

				mockstore.
					EXPECT().
					ListServices(gomock.Eq("coolproject")).
					Times(0)
			},
		},
		"with failed call to list": {
			expectedErr: mockError,
			listOpts: listAppOpts{
				listAppVars: listAppVars{
					GlobalOpts: &GlobalOpts{
						appName: "coolproject",
					},
				},
				store: mockstore,
			},
			mocking: func() {
				mockstore.EXPECT().
					GetApplication(gomock.Eq("coolproject")).
					Return(&config.Application{}, nil)

				mockstore.
					EXPECT().
					ListServices(gomock.Eq("coolproject")).
					Return(nil, mockError)
			},
		},
		"with local flag enabled": {
			expectedErr: nil,
			listOpts: listAppOpts{
				listAppVars: listAppVars{
					ShouldShowLocalApps: true,
					GlobalOpts: &GlobalOpts{
						appName: "coolproject",
					},
				},
				store: mockstore,
				ws:    mockWorkspace,
			},
			mocking: func() {
				mockstore.EXPECT().
					GetApplication(gomock.Eq("coolproject")).
					Return(&config.Application{}, nil)
				mockstore.
					EXPECT().
					ListServices(gomock.Eq("coolproject")).
					Return([]*config.Service{
						{Name: "my-app", Type: "Load Balanced Web Service"},
						{Name: "lb-app", Type: "Load Balanced Web Service"},
					}, nil)
				mockWorkspace.EXPECT().ServiceNames().
					Return([]string{"my-app"}, nil).Times(1)
			},
			expectedContent: "Name                Type\n------              -------------------------\nmy-app              Load Balanced Web Service\n",
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

		mockstore  func(m *mocks.Mockstore)
		mockPrompt func(m *mocks.Mockprompter)

		wantedProject string
	}{
		"with no flags set": {
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListApplications().Return([]*config.Application{
					&config.Application{Name: "my-project"},
					&config.Application{Name: "archer-project"},
				}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(applicationListProjectNamePrompt, applicationListProjectNameHelpPrompt, []string{"my-project", "archer-project"}).Return("my-project", nil).Times(1)
			},
			wantedProject: "my-project",
		},
		"with project flags set": {
			mockstore:     func(m *mocks.Mockstore) {},
			mockPrompt:    func(m *mocks.Mockprompter) {},
			inputProject:  "my-project",
			wantedProject: "my-project",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockstore := mocks.NewMockstore(ctrl)
			mockPrompter := mocks.NewMockprompter(ctrl)
			tc.mockstore(mockstore)
			tc.mockPrompt(mockPrompter)

			listApps := &listAppOpts{
				listAppVars: listAppVars{
					GlobalOpts: &GlobalOpts{
						prompt:  mockPrompter,
						appName: tc.inputProject,
					},
				},
				store: mockstore,
			}

			err := listApps.Ask()

			require.NoError(t, err)
			require.Equal(t, tc.wantedProject, listApps.AppName(), "expected project names to match")
		})
	}
}

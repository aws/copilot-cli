// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"testing"

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
	"github.com/aws/PRIVATE-amazon-ecs-archer/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestProjectInit_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockProjectStore := mocks.NewMockProjectStore(ctrl)
	mockWorkspace := mocks.NewMockWorkspace(ctrl)
	mockError := fmt.Errorf("error")
	var capturedArgument *archer.Project
	defer ctrl.Finish()

	testCases := map[string]struct {
		initProjectOpts InitProjectOpts
		expectedProject archer.Project
		expectedError   error
		mocking         func()
	}{
		"with a succesful call to add env": {
			initProjectOpts: InitProjectOpts{
				ProjectName: "project",
				manager:     mockProjectStore,
				ws:          mockWorkspace,
			},
			expectedProject: archer.Project{
				Name:    "project",
				Version: "1.0",
			},
			mocking: func() {
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("project"))
				mockProjectStore.
					EXPECT().
					CreateProject(gomock.Any()).
					Do(func(project *archer.Project) {
						capturedArgument = project
					})
			},
		},
		"should return error from CreateProject": {
			initProjectOpts: InitProjectOpts{
				ProjectName: "project",
				manager:     mockProjectStore,
				ws:          mockWorkspace,
			},
			expectedError: mockError,
			mocking: func() {
				mockWorkspace.
					EXPECT().
					Create(gomock.Any()).
					Times(0)

				mockProjectStore.
					EXPECT().
					CreateProject(gomock.Any()).
					Return(mockError)

			},
		},

		"should return error from workspace.Create": {
			initProjectOpts: InitProjectOpts{
				ProjectName: "project",
				manager:     mockProjectStore,
				ws:          mockWorkspace,
			},
			expectedError: mockError,
			mocking: func() {
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("project")).
					Return(mockError)
				mockProjectStore.
					EXPECT().
					CreateProject(gomock.Any()).
					Return(nil)

			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup mocks
			tc.mocking()

			err := tc.initProjectOpts.Execute()
			if tc.expectedError == nil {
				require.NoError(t, err)
				require.Equal(t, tc.expectedProject, *capturedArgument)
			} else {
				require.Equal(t, tc.expectedError, err)
			}
		})
	}
}

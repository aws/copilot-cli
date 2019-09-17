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

func TestInit_Prepare(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockProjectStore := mocks.NewMockProjectStore(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		inputOpts              InitAppOpts
		mocking                func()
		wantedExistingProjects []string
	}{
		"with existing projects": {
			inputOpts: InitAppOpts{
				Name:    "frontend",
				Project: "coolproject",
			},
			wantedExistingProjects: []string{"project1", "project2"},
			mocking: func() {
				mockProjectStore.
					EXPECT().
					ListProjects().
					Return([]*archer.Project{
						&archer.Project{Name: "project1"},
						&archer.Project{Name: "project2"},
					}, nil)

			},
		},
		"with error loading projects": {
			inputOpts: InitAppOpts{
				Name:    "frontend",
				Project: "coolproject",
			},
			wantedExistingProjects: []string{},
			mocking: func() {
				mockProjectStore.
					EXPECT().
					ListProjects().
					Return(nil, fmt.Errorf("error loading projects"))

			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()
			tc.inputOpts.projStore = mockProjectStore
			tc.inputOpts.Prepare()
			require.ElementsMatch(t, tc.wantedExistingProjects, tc.inputOpts.existingProjects)
		})
	}
}

func TestInit_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputOpts       InitAppOpts
		wantedErrPrefix string
	}{
		"with valid project and app": {
			inputOpts: InitAppOpts{
				Name:    "frontend",
				Project: "coolproject",
			},
		},
		"with invalid project name": {
			inputOpts: InitAppOpts{
				Name:    "coolapp",
				Project: "!!!!",
			},
			wantedErrPrefix: "project name invalid",
		},
		"with invalid app name": {
			inputOpts: InitAppOpts{
				Name:    "!!!",
				Project: "coolproject",
			},
			wantedErrPrefix: "application name invalid",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.inputOpts.Validate()
			if tc.wantedErrPrefix != "" {
				require.Regexp(t, "^"+tc.wantedErrPrefix+".*", err.Error())
			} else {
				require.NoError(t, err, "There shouldn't have been an error")
			}
		})
	}
}

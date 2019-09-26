// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
	"github.com/aws/PRIVATE-amazon-ecs-archer/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestProjectList_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockProjectStore := mocks.NewMockProjectStore(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		listOpts ListProjectOpts
		mocking  func()
		want     error
	}{
		"with projects": {
			listOpts: ListProjectOpts{
				store: mockProjectStore,
				w:     ioutil.Discard,
			},
			mocking: func() {
				mockProjectStore.
					EXPECT().
					ListProjects().
					Return([]*archer.Project{
						&archer.Project{Name: "project1"},
						&archer.Project{Name: "project2"},
					}, nil).
					Times(1)
			},
		},
		"with an error": {
			listOpts: ListProjectOpts{
				store: mockProjectStore,
				w:     ioutil.Discard,
			},
			mocking: func() {
				mockProjectStore.
					EXPECT().
					ListProjects().
					Return(nil, fmt.Errorf("error fetching projects")).
					Times(1)
			},
			want: fmt.Errorf("error fetching projects"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()

			got := tc.listOpts.Execute()

			require.Equal(t, tc.want, got)
		})
	}
}

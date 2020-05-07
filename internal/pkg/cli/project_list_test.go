// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestProjectList_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockStoreClient := mocks.NewMockstoreClient(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		listOpts listProjectOpts
		mocking  func()
		want     error
	}{
		"with projects": {
			listOpts: listProjectOpts{
				store: mockStoreClient,
				w:     ioutil.Discard,
			},
			mocking: func() {
				mockStoreClient.
					EXPECT().
					ListApplications().
					Return([]*config.Application{
						{Name: "project1"},
						{Name: "project2"},
					}, nil).
					Times(1)
			},
		},
		"with an error": {
			listOpts: listProjectOpts{
				store: mockStoreClient,
				w:     ioutil.Discard,
			},
			mocking: func() {
				mockStoreClient.
					EXPECT().
					ListApplications().
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

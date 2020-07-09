// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestListAppOpts_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockstore := mocks.NewMockstore(ctrl)
	defer ctrl.Finish()
	testError := errors.New("error fetching apps")

	testCases := map[string]struct {
		listOpts listAppOpts
		mocking  func()
		want     error
	}{
		"with applications": {
			listOpts: listAppOpts{
				store: mockstore,
				w:     ioutil.Discard,
			},
			mocking: func() {
				mockstore.
					EXPECT().
					ListApplications().
					Return([]*config.Application{
						{Name: "app1"},
						{Name: "app2"},
					}, nil).
					Times(1)
			},
		},
		"with an error": {
			listOpts: listAppOpts{
				store: mockstore,
				w:     ioutil.Discard,
			},
			mocking: func() {
				mockstore.
					EXPECT().
					ListApplications().
					Return(nil, testError).
					Times(1)
			},
			want: fmt.Errorf("list applications: %w", testError),
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

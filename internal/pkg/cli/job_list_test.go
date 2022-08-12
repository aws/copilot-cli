// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestListJobOpts_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockError := fmt.Errorf("error")
	mockLister := mocks.NewMockworkloadListWriter(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		opts        listJobOpts
		mocking     func()
		expectedErr error
	}{
		"with successful call to list.Jobs": {
			opts: listJobOpts{
				listWkldVars: listWkldVars{
					shouldOutputJSON: true,
					appName:          "coolapp",
				},
				list: mockLister,
			},
			mocking: func() {
				mockLister.EXPECT().
					Write("coolapp").
					Return(nil)
			},
		},
		"with failed call to list.Jobs": {
			opts: listJobOpts{
				listWkldVars: listWkldVars{
					appName: "coolapp",
				},
				list: mockLister,
			},
			mocking: func() {
				mockLister.EXPECT().
					Write(gomock.Eq("coolapp")).
					Return(mockError)
			},
			expectedErr: fmt.Errorf("error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()
			err := tc.opts.Execute()

			if tc.expectedErr != nil {
				require.EqualError(t, tc.expectedErr, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestListJobOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inApp string

		mockSel func(m *mocks.MockappSelector)

		wantedApp string
	}{
		"with no flags set": {
			mockSel: func(m *mocks.MockappSelector) {
				m.EXPECT().Application(jobListAppNamePrompt, wkldAppNameHelpPrompt).Return("myapp", nil)
			},
			wantedApp: "myapp",
		},
		"with app flag set": {
			mockSel: func(m *mocks.MockappSelector) {
				m.EXPECT().Application(gomock.Any(), gomock.Any()).Times(0)
			},
			inApp:     "myapp",
			wantedApp: "myapp",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSel := mocks.NewMockappSelector(ctrl)
			tc.mockSel(mockSel)

			listApps := &listJobOpts{
				listWkldVars: listWkldVars{
					appName: tc.inApp,
				},
				sel: mockSel,
			}

			err := listApps.Ask()

			require.NoError(t, err)
			require.Equal(t, tc.wantedApp, listApps.appName, "expected application names to match")
		})
	}
}

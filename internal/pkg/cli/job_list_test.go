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
	mockLister := mocks.NewMockwsStoreJobLister(ctrl)
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
					Jobs("coolapp", false, true).
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
					Jobs(gomock.Eq("coolapp"), gomock.Any(), gomock.Any()).
					Return(mockError)
			},
			expectedErr: fmt.Errorf("error"),
		},
		"shouldListLocal argument rendered correctly": {
			opts: listJobOpts{
				listWkldVars: listWkldVars{
					shouldShowLocalWorkloads: true,
					appName:                  "coolapp",
				},
				list: mockLister,
			},
			mocking: func() {
				mockLister.EXPECT().
					Jobs("coolapp", true, false).
					Return(nil)
			},
		},
		"json and local arguments rendered correctly": {
			opts: listJobOpts{
				listWkldVars: listWkldVars{
					shouldShowLocalWorkloads: true,
					shouldOutputJSON:         true,
					appName:                  "coolapp",
				},
				list: mockLister,
			},
			mocking: func() {
				mockLister.EXPECT().
					Jobs("coolapp", true, true).
					Return(nil)
			},
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
				m.EXPECT().Application(jobListAppNamePrompt, wkldListAppNameHelp).Return("myapp", nil)
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

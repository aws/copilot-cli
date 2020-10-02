// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestListJobOpts_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockError := fmt.Errorf("error")
	mockstore := mocks.NewMockstore(ctrl)
	mockWorkspace := mocks.NewMockwsJobDirReader(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		opts            listJobOpts
		mocking         func()
		expectedErr     error
		expectedContent string
	}{
		"with json outputs": {
			opts: listJobOpts{
				listWkldVars: listWkldVars{
					shouldOutputJSON: true,
					appName:          "coolapp",
				},
				store: mockstore,
			},
			mocking: func() {
				mockstore.EXPECT().
					GetApplication(gomock.Eq("coolapp")).
					Return(&config.Application{}, nil)
				mockstore.
					EXPECT().
					ListJobs(gomock.Eq("coolapp")).
					Return([]*config.Workload{
						{Name: "mailer"},
						{Name: "reaper"},
					}, nil)
			},
			expectedContent: "{\"jobs\":[{\"app\":\"\",\"name\":\"mailer\",\"type\":\"\"},{\"app\":\"\",\"name\":\"reaper\",\"type\":\"\"}]}\n",
		},
		"with human outputs": {
			opts: listJobOpts{
				listWkldVars: listWkldVars{
					appName: "coolapp",
				},
				store: mockstore,
			},
			mocking: func() {
				mockstore.EXPECT().
					GetApplication(gomock.Eq("coolapp")).
					Return(&config.Application{}, nil)
				mockstore.
					EXPECT().
					ListJobs(gomock.Eq("coolapp")).
					Return([]*config.Workload{
						{Name: "mailer", Type: "Scheduled Job"},
						{Name: "reaper", Type: "Scheduled Job"},
					}, nil)
			},
			expectedContent: "Name                Type\n------              -------------\nmailer              Scheduled Job\nreaper              Scheduled Job\n",
		},
		"with invalid app name": {
			expectedErr: fmt.Errorf("get application: %w", mockError),
			opts: listJobOpts{
				listWkldVars: listWkldVars{
					appName: "coolapp",
				},
				store: mockstore,
			},
			mocking: func() {
				mockstore.EXPECT().
					GetApplication(gomock.Eq("coolapp")).
					Return(nil, mockError)

				mockstore.
					EXPECT().
					ListJobs(gomock.Eq("coolapp")).
					Times(0)
			},
		},
		"with failed call to list": {
			expectedErr: mockError,
			opts: listJobOpts{
				listWkldVars: listWkldVars{
					appName: "coolapp",
				},
				store: mockstore,
			},
			mocking: func() {
				mockstore.EXPECT().
					GetApplication(gomock.Eq("coolapp")).
					Return(&config.Application{}, nil)

				mockstore.
					EXPECT().
					ListJobs(gomock.Eq("coolapp")).
					Return(nil, mockError)
			},
		},
		"with local flag enabled": {
			expectedErr: nil,
			opts: listJobOpts{
				listWkldVars: listWkldVars{
					shouldShowLocalWorkloads: true,
					appName:                  "coolapp",
				},
				store: mockstore,
				ws:    mockWorkspace,
			},
			mocking: func() {
				mockstore.EXPECT().
					GetApplication(gomock.Eq("coolapp")).
					Return(&config.Application{}, nil)
				mockstore.
					EXPECT().
					ListJobs(gomock.Eq("coolapp")).
					Return([]*config.Workload{
						{Name: "mailer", Type: "Scheduled Job"},
						{Name: "reaper", Type: "Scheduled Job"},
					}, nil)
				mockWorkspace.EXPECT().JobNames().
					Return([]string{"mailer"}, nil).Times(1)
			},
			expectedContent: "Name                Type\n------              -------------\nmailer              Scheduled Job\n",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			b := &bytes.Buffer{}
			tc.mocking()
			tc.opts.w = b
			err := tc.opts.Execute()

			if tc.expectedErr != nil {
				require.EqualError(t, tc.expectedErr, err.Error())
			} else {
				require.Equal(t, tc.expectedContent, b.String())
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

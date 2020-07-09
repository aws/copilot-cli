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

func TestListSvcOpts_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockError := fmt.Errorf("error")
	mockstore := mocks.NewMockstore(ctrl)
	mockWorkspace := mocks.NewMockwsSvcReader(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		opts            listSvcOpts
		mocking         func()
		expectedErr     error
		expectedContent string
	}{
		"with json outputs": {
			opts: listSvcOpts{
				listSvcVars: listSvcVars{
					ShouldOutputJSON: true,
					GlobalOpts: &GlobalOpts{
						appName: "coolapp",
					},
				},
				store: mockstore,
			},
			mocking: func() {
				mockstore.EXPECT().
					GetApplication(gomock.Eq("coolapp")).
					Return(&config.Application{}, nil)
				mockstore.
					EXPECT().
					ListServices(gomock.Eq("coolapp")).
					Return([]*config.Service{
						{Name: "my-svc"},
						{Name: "lb-svc"},
					}, nil)
			},
			expectedContent: "{\"services\":[{\"app\":\"\",\"name\":\"my-svc\",\"type\":\"\"},{\"app\":\"\",\"name\":\"lb-svc\",\"type\":\"\"}]}\n",
		},
		"with human outputs": {
			opts: listSvcOpts{
				listSvcVars: listSvcVars{
					GlobalOpts: &GlobalOpts{
						appName: "coolapp",
					},
				},
				store: mockstore,
			},
			mocking: func() {
				mockstore.EXPECT().
					GetApplication(gomock.Eq("coolapp")).
					Return(&config.Application{}, nil)
				mockstore.
					EXPECT().
					ListServices(gomock.Eq("coolapp")).
					Return([]*config.Service{
						{Name: "my-svc", Type: "Load Balanced Web Service"},
						{Name: "lb-svc", Type: "Load Balanced Web Service"},
					}, nil)
			},
			expectedContent: "Name                Type\n------              -------------------------\nmy-svc              Load Balanced Web Service\nlb-svc              Load Balanced Web Service\n",
		},
		"with invalid app name": {
			expectedErr: fmt.Errorf("get application: %w", mockError),
			opts: listSvcOpts{
				listSvcVars: listSvcVars{
					GlobalOpts: &GlobalOpts{
						appName: "coolapp",
					},
				},
				store: mockstore,
			},
			mocking: func() {
				mockstore.EXPECT().
					GetApplication(gomock.Eq("coolapp")).
					Return(nil, mockError)

				mockstore.
					EXPECT().
					ListServices(gomock.Eq("coolapp")).
					Times(0)
			},
		},
		"with failed call to list": {
			expectedErr: mockError,
			opts: listSvcOpts{
				listSvcVars: listSvcVars{
					GlobalOpts: &GlobalOpts{
						appName: "coolapp",
					},
				},
				store: mockstore,
			},
			mocking: func() {
				mockstore.EXPECT().
					GetApplication(gomock.Eq("coolapp")).
					Return(&config.Application{}, nil)

				mockstore.
					EXPECT().
					ListServices(gomock.Eq("coolapp")).
					Return(nil, mockError)
			},
		},
		"with local flag enabled": {
			expectedErr: nil,
			opts: listSvcOpts{
				listSvcVars: listSvcVars{
					ShouldShowLocalServices: true,
					GlobalOpts: &GlobalOpts{
						appName: "coolapp",
					},
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
					ListServices(gomock.Eq("coolapp")).
					Return([]*config.Service{
						{Name: "my-svc", Type: "Load Balanced Web Service"},
						{Name: "lb-svc", Type: "Load Balanced Web Service"},
					}, nil)
				mockWorkspace.EXPECT().ServiceNames().
					Return([]string{"my-svc"}, nil).Times(1)
			},
			expectedContent: "Name                Type\n------              -------------------------\nmy-svc              Load Balanced Web Service\n",
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

func TestListSvcOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inApp string

		mockSel func(m *mocks.MockappSelector)

		wantedApp string
	}{
		"with no flags set": {
			mockSel: func(m *mocks.MockappSelector) {
				m.EXPECT().Application(svcListAppNamePrompt, svcListAppNameHelpPrompt).Return("myapp", nil)
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

			listApps := &listSvcOpts{
				listSvcVars: listSvcVars{
					GlobalOpts: &GlobalOpts{
						appName: tc.inApp,
					},
				},
				sel: mockSel,
			}

			err := listApps.Ask()

			require.NoError(t, err)
			require.Equal(t, tc.wantedApp, listApps.AppName(), "expected application names to match")
		})
	}
}

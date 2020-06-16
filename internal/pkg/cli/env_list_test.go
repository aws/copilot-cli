// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestEnvList_Ask(t *testing.T) {
	testCases := map[string]struct {
		inputApp string

		mockSelector func(m *mocks.MockconfigSelector)

		wantedApp string
		wantedErr error
	}{
		"with no flags set": {
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(envListAppNamePrompt, envListAppNameHelper).Return("my-app", nil)
			},
			wantedApp: "my-app",
		},
		"with env flags set": {
			inputApp:     "my-app",
			wantedApp:    "my-app",
			mockSelector: func(m *mocks.MockconfigSelector) {},
		},
		"error if fail to select app": {
			mockSelector: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(envListAppNamePrompt, envListAppNameHelper).Return("", errors.New("some error"))
			},
			wantedApp: "my-app",
			wantedErr: fmt.Errorf("select application: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSelector := mocks.NewMockconfigSelector(ctrl)
			tc.mockSelector(mockSelector)

			listEnvs := &listEnvOpts{
				listEnvVars: listEnvVars{
					GlobalOpts: &GlobalOpts{
						appName: tc.inputApp,
					},
				},
				sel: mockSelector,
			}

			err := listEnvs.Ask()

			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedApp, listEnvs.AppName(), "expected app names to match")
			}
		})
	}
}
func TestEnvList_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockError := fmt.Errorf("error")
	mockstore := mocks.NewMockstore(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		listOpts        listEnvOpts
		mocking         func()
		expectedErr     error
		expectedContent string
	}{
		"with json envs": {
			listOpts: listEnvOpts{
				listEnvVars: listEnvVars{
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
					ListEnvironments(gomock.Eq("coolapp")).
					Return([]*config.Environment{
						{Name: "test"},
						{Name: "test2"},
					}, nil)
			},
			expectedContent: "{\"environments\":[{\"app\":\"\",\"name\":\"test\",\"region\":\"\",\"accountID\":\"\",\"prod\":false,\"registryURL\":\"\",\"executionRoleARN\":\"\",\"managerRoleARN\":\"\"},{\"app\":\"\",\"name\":\"test2\",\"region\":\"\",\"accountID\":\"\",\"prod\":false,\"registryURL\":\"\",\"executionRoleARN\":\"\",\"managerRoleARN\":\"\"}]}\n",
		},
		"with envs": {
			listOpts: listEnvOpts{
				listEnvVars: listEnvVars{
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
					ListEnvironments(gomock.Eq("coolapp")).
					Return([]*config.Environment{
						{Name: "test"},
						{Name: "test2"},
					}, nil)
			},
			expectedContent: "test\ntest2\n",
		},
		"with invalid app name": {
			expectedErr: mockError,
			listOpts: listEnvOpts{
				listEnvVars: listEnvVars{
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
					ListEnvironments(gomock.Eq("coolapp")).
					Times(0)
			},
		},
		"with failed call to list": {
			expectedErr: mockError,
			listOpts: listEnvOpts{
				listEnvVars: listEnvVars{
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
					ListEnvironments(gomock.Eq("coolapp")).
					Return(nil, mockError)
			},
		},
		"with production envs": {
			listOpts: listEnvOpts{
				listEnvVars: listEnvVars{
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
					ListEnvironments(gomock.Eq("coolapp")).
					Return([]*config.Environment{
						{Name: "test"},
						{Name: "test2", Prod: true},
					}, nil)
			},
			expectedContent: "test\ntest2 (prod)\n",
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

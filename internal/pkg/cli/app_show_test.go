// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type showAppMocks struct {
	storeSvc *mocks.Mockstore
	prompt   *mocks.Mockprompter
	sel      *mocks.MockappSelector
}

func TestShowAppOpts_Validate(t *testing.T) {
	testError := errors.New("some error")
	testCases := map[string]struct {
		inAppName  string
		setupMocks func(mocks showAppMocks)

		wantedError error
	}{
		"valid app name": {
			inAppName: "my-app",

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name: "my-app",
				}, nil)
			},
			wantedError: nil,
		},
		"invalid app name": {
			inAppName: "my-app",

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(nil, testError)
			},

			wantedError: fmt.Errorf("get application %s: %w", "my-app", testError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			mockPrompter := mocks.NewMockprompter(ctrl)

			mocks := showAppMocks{
				storeSvc: mockStoreReader,
				prompt:   mockPrompter,
			}
			tc.setupMocks(mocks)

			opts := &showAppOpts{
				showAppVars: showAppVars{
					GlobalOpts: &GlobalOpts{
						prompt:  mockPrompter,
						appName: tc.inAppName,
					},
				},
				store: mockStoreReader,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestShowAppOpts_Ask(t *testing.T) {
	testError := errors.New("some error")
	testCases := map[string]struct {
		inApp string

		setupMocks func(mocks showAppMocks)

		wantedApp   string
		wantedError error
	}{
		"with all flags": {
			inApp: "my-app",

			setupMocks: func(m showAppMocks) {},

			wantedApp:   "my-app",
			wantedError: nil,
		},
		"prompt for all input": {
			inApp: "",

			setupMocks: func(m showAppMocks) {
				m.sel.EXPECT().Application(appShowNamePrompt, appShowNameHelpPrompt).Return("my-app", nil)
			},
			wantedApp:   "my-app",
			wantedError: nil,
		},
		"returns error if failed to select application": {
			inApp: "",

			setupMocks: func(m showAppMocks) {
				m.sel.EXPECT().Application(gomock.Any(), gomock.Any()).Return("", testError)
			},

			wantedError: fmt.Errorf("select application: %w", testError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := showAppMocks{
				sel: mocks.NewMockappSelector(ctrl),
			}
			tc.setupMocks(mocks)

			opts := &showAppOpts{
				showAppVars: showAppVars{
					GlobalOpts: &GlobalOpts{
						appName: tc.inApp,
					},
				},
				sel: mocks.sel,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedApp, opts.AppName(), "expected app names to match")

			}
		})
	}
}

func TestShowAppOpts_Execute(t *testing.T) {
	testAppName := "my-app"
	testError := errors.New("some error")
	testCases := map[string]struct {
		shouldOutputJSON bool

		setupMocks func(mocks showAppMocks)

		wantedContent string
		wantedError   error
	}{
		"correctly shows json output": {
			shouldOutputJSON: true,

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:   "my-app",
					Domain: "example.com",
				}, nil)
				m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Service{
					{
						Name: "my-svc",
						Type: "lb-web-svc",
					},
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
						Prod:      false,
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
						Prod:      true,
					},
				}, nil)
			},

			wantedContent: "{\"name\":\"my-app\",\"uri\":\"example.com\",\"environments\":[{\"app\":\"\",\"name\":\"test\",\"region\":\"us-west-2\",\"accountID\":\"123456789\",\"prod\":false,\"registryURL\":\"\",\"executionRoleARN\":\"\",\"managerRoleARN\":\"\"},{\"app\":\"\",\"name\":\"prod\",\"region\":\"us-west-1\",\"accountID\":\"123456789\",\"prod\":true,\"registryURL\":\"\",\"executionRoleARN\":\"\",\"managerRoleARN\":\"\"}],\"services\":[{\"app\":\"\",\"name\":\"my-svc\",\"type\":\"lb-web-svc\"}]}\n",
		},
		"correctly shows human output": {
			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:   "my-app",
					Domain: "example.com",
				}, nil)
				m.storeSvc.EXPECT().ListServices("my-app").Return([]*config.Service{
					{
						Name: "my-svc",
						Type: "lb-web-svc",
					},
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
			},

			wantedContent: `About

  Name              my-app
  URI               example.com

Environments

  Name              AccountID           Region
  test              123456789           us-west-2
  prod              123456789           us-west-1

Services

  Name              Type
  my-svc            lb-web-svc
`,
		},
		"returns error if fail to get application": {
			shouldOutputJSON: false,

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(nil, testError)
			},

			wantedError: fmt.Errorf("get application %s: %w", "my-app", testError),
		},
		"returns error if fail to list environment": {
			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:   "my-app",
					Domain: "example.com",
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return(nil, testError)
			},

			wantedError: fmt.Errorf("list environments in application %s: %w", "my-app", testError),
		},
		"returns error if fail to list services": {
			shouldOutputJSON: false,

			setupMocks: func(m showAppMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{
					Name:   "my-app",
					Domain: "example.com",
				}, nil)
				m.storeSvc.EXPECT().ListEnvironments("my-app").Return([]*config.Environment{
					{
						Name:      "test",
						Region:    "us-west-2",
						AccountID: "123456789",
					},
					{
						Name:      "prod",
						AccountID: "123456789",
						Region:    "us-west-1",
					},
				}, nil)
				m.storeSvc.EXPECT().ListServices("my-app").Return(nil, testError)
			},

			wantedError: fmt.Errorf("list services in application %s: %w", "my-app", testError),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			b := &bytes.Buffer{}
			mockStoreReader := mocks.NewMockstore(ctrl)

			mocks := showAppMocks{
				storeSvc: mockStoreReader,
			}
			tc.setupMocks(mocks)

			opts := &showAppOpts{
				showAppVars: showAppVars{
					shouldOutputJSON: tc.shouldOutputJSON,
					GlobalOpts: &GlobalOpts{
						appName: testAppName,
					},
				},
				store: mockStoreReader,
				w:     b,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedContent, b.String(), "expected output content match")
			}
		})
	}
}

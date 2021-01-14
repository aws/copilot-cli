// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"testing"

	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeleteTaskOpts_Validate(t *testing.T) {

	testCases := map[string]struct {
		inAppName        string
		inEnvName        string
		inName           string
		inDefaultCluster bool
		setupMocks       func(m *mocks.Mockstore)

		want error
	}{
		"with only app flag": {
			inAppName: "phonetool",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			want: nil,
		},
		"with no flags": {
			setupMocks: func(m *mocks.Mockstore) {},
			want:       nil,
		},
		"with app/env flags set": {
			inAppName: "phonetool",
			inEnvName: "test",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{Name: "test", App: "phonetool"}, nil)
			},
			want: nil,
		},
		"with default cluster flag set": {
			inDefaultCluster: true,
			inName:           "oneoff",
			setupMocks:       func(m *mocks.Mockstore) {},
			want:             nil,
		},
		"with default cluster and env flag": {
			inDefaultCluster: true,
			inEnvName:        "test",
			inAppName:        "phonetool",
			setupMocks:       func(m *mocks.Mockstore) {},
			want:             errors.New("cannot specify both `--app` and `--default`"),
		},
		"with error getting app": {
			inAppName: "phonetool",
			inEnvName: "test",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(nil, errors.New("some error"))
			},
			want: errors.New("get application: some error"),
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockstore := mocks.NewMockstore(ctrl)

			tc.setupMocks(mockstore)

			opts := deleteTaskOpts{
				deleteTaskVars: deleteTaskVars{
					skipConfirmation: false,
					app:              tc.inAppName,
					env:              tc.inEnvName,
					name:             tc.inName,
					defaultCluster:   tc.inDefaultCluster,
				},
				store: mockstore,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.want != nil {
				require.EqualError(t, err, tc.want.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}

}

func TestDeleteTaskOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inAppName          string
		inEnvName          string
		inName             string
		inDefaultCluster   bool
		inSkipConfirmation bool

		mockStore      func(m *mocks.Mockstore)
		mockSel        func(m *mocks.MockwsSelector)
		mockTaskSelect func(m *mocks.MockcfTaskSelector)
		mockSess       func(m *mocks.MocksessionProvider)
		mockPrompter   func(m *mocks.Mockprompter)

		wantErr string
	}{
		"all flags specified": {
			inAppName:          "phonetool",
			inEnvName:          "test",
			inName:             "abcd",
			inSkipConfirmation: true,

			mockStore:      func(m *mocks.Mockstore) {},
			mockSel:        func(m *mocks.MockwsSelector) {},
			mockTaskSelect: func(m *mocks.MockcfTaskSelector) {},
			mockSess:       func(m *mocks.MocksessionProvider) {},
			mockPrompter:   func(m *mocks.Mockprompter) {},
		},
		"name flag not specified": {
			inAppName: "phonetool",
			inEnvName: "test",

			mockStore: func(m *mocks.Mockstore) {
				// This call is in GetSession when an environment is specified and we need to get the Manager Role's session.
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{Name: "test", App: "phonetool"}, nil)
			},
			mockSel: func(m *mocks.MockwsSelector) {},
			mockTaskSelect: func(m *mocks.MockcfTaskSelector) {
				m.EXPECT().Task(taskDeleteNamePrompt, "", gomock.Any()).Return(&selector.DeployedTask{Name: "abc"}, nil)
			},
			mockSess: func(m *mocks.MocksessionProvider) {
				m.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&awssession.Session{}, nil)
			},
			mockPrompter: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm("Are you sure you want to delete abc from application phonetool and environment test?", gomock.Any()).Return(true, nil)
			},
		},
		"name flag not specified and confirm cancelled": {
			inAppName: "phonetool",
			inEnvName: "test",

			mockStore: func(m *mocks.Mockstore) {
				// This call is in GetSession when an environment is specified and we need to get the Manager Role's session.
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{Name: "test", App: "phonetool"}, nil)
			},
			mockSel: func(m *mocks.MockwsSelector) {},
			mockTaskSelect: func(m *mocks.MockcfTaskSelector) {
				m.EXPECT().Task(taskDeleteNamePrompt, "", gomock.Any()).Return(&selector.DeployedTask{Name: "abc"}, nil)
			},
			mockSess: func(m *mocks.MocksessionProvider) {
				m.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&awssession.Session{}, nil)
			},
			mockPrompter: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm("Are you sure you want to delete abc from application phonetool and environment test?", gomock.Any()).Return(false, nil)
			},
			wantErr: "task delete cancelled - no changes made",
		},
		"default flag specified": {
			inDefaultCluster: true,

			mockStore: func(m *mocks.Mockstore) {
			},
			mockSel: func(m *mocks.MockwsSelector) {},
			mockTaskSelect: func(m *mocks.MockcfTaskSelector) {
				m.EXPECT().Task(taskDeleteNamePrompt, "", gomock.Any()).Return(&selector.DeployedTask{Name: "abc"}, nil)
			},
			mockSess: func(m *mocks.MocksessionProvider) {
				m.EXPECT().Default().Return(&awssession.Session{}, nil)
			},
			mockPrompter: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm("Are you sure you want to delete abc from the default cluster?", gomock.Any()).Return(true, nil)
			},
		},
		"no flags specified": {
			mockStore: func(m *mocks.Mockstore) {
				// This call is in GetSession when an environment is specified and we need to get the Manager Role's session.
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{Name: "test", App: "phonetool"}, nil)
			},
			mockSel: func(m *mocks.MockwsSelector) {
				m.EXPECT().Application(taskDeleteAppPrompt, "", appEnvOptionNone).Return("phonetool", nil)
				m.EXPECT().Environment(taskDeleteEnvPrompt, "", "phonetool", appEnvOptionNone).Return("test", nil)
			},
			mockTaskSelect: func(m *mocks.MockcfTaskSelector) {
				m.EXPECT().Task(taskDeleteNamePrompt, "", gomock.Any()).Return(&selector.DeployedTask{Name: "abc"}, nil)
			},
			mockSess: func(m *mocks.MocksessionProvider) {
				m.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&awssession.Session{}, nil)
			},
			mockPrompter: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm("Are you sure you want to delete abc from application phonetool and environment test?", gomock.Any()).Return(true, nil)
			},
		},
		"no flags specified (default path)": {
			mockStore: func(m *mocks.Mockstore) {},
			mockSel: func(m *mocks.MockwsSelector) {
				m.EXPECT().Application(taskDeleteAppPrompt, "", appEnvOptionNone).Return(appEnvOptionNone, nil)
			},
			mockTaskSelect: func(m *mocks.MockcfTaskSelector) {
				m.EXPECT().Task(taskDeleteNamePrompt, "", gomock.Any()).Return(&selector.DeployedTask{Name: "abc"}, nil)
			},
			mockSess: func(m *mocks.MocksessionProvider) {
				m.EXPECT().Default().Return(&awssession.Session{}, nil)
			},
			mockPrompter: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm("Are you sure you want to delete abc from the default cluster?", gomock.Any()).Return(true, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstore(ctrl)
			mockSel := mocks.NewMockwsSelector(ctrl)
			mockSess := mocks.NewMocksessionProvider(ctrl)
			mockTaskSel := mocks.NewMockcfTaskSelector(ctrl)
			mockPrompt := mocks.NewMockprompter(ctrl)

			tc.mockStore(mockStore)
			tc.mockSel(mockSel)
			tc.mockSess(mockSess)
			tc.mockTaskSelect(mockTaskSel)
			tc.mockPrompter(mockPrompt)

			opts := deleteTaskOpts{
				deleteTaskVars: deleteTaskVars{
					skipConfirmation: tc.inSkipConfirmation,
					defaultCluster:   tc.inDefaultCluster,
					app:              tc.inAppName,
					env:              tc.inEnvName,
					name:             tc.inName,
				},

				store:  mockStore,
				sel:    mockSel,
				sess:   mockSess,
				prompt: mockPrompt,

				newTaskSel: func(sess *awssession.Session) cfTaskSelector { return mockTaskSel },
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

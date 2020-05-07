// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeleteAppOpts_Validate(t *testing.T) {
	mockError := errors.New("some error")

	tests := map[string]struct {
		inProjectName string
		inAppName     string
		inEnvName     string
		setupMocks    func(m *mocks.Mockstore)

		want error
	}{
		"should return errNoProjectInWorkspace": {
			setupMocks: func(m *mocks.Mockstore) {},
			inAppName:  "my-app",
			want:       errNoProjectInWorkspace,
		},
		"with no flag set": {
			inProjectName: "phonetool",
			setupMocks:    func(m *mocks.Mockstore) {},
			want:          nil,
		},
		"with all flag set": {
			inProjectName: "phonetool",
			inAppName:     "my-app",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetService("phonetool", "my-app").Times(1).Return(&config.Service{
					Name: "my-app",
				}, nil)
			},
			want: nil,
		},
		"with env flag set": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(&config.Environment{Name: "test"}, nil)
			},
			want: nil,
		},
		"with unknown environment": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(nil, errors.New("unknown env"))
			},
			want: errors.New("get environment test from metadata store: unknown env"),
		},
		"should return error if fail to get app name": {
			inProjectName: "phonetool",
			inAppName:     "my-app",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetService("phonetool", "my-app").Times(1).Return(nil, mockError)
			},
			want: errors.New("some error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockstore := mocks.NewMockstore(ctrl)

			test.setupMocks(mockstore)

			opts := deleteAppOpts{
				deleteAppVars: deleteAppVars{
					GlobalOpts: &GlobalOpts{
						projectName: test.inProjectName,
					},
					AppName: test.inAppName,
					EnvName: test.inEnvName,
				},
				store: mockstore,
			}

			err := opts.Validate()

			if test.want != nil {
				require.EqualError(t, err, test.want.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestDeleteAppOpts_Ask(t *testing.T) {
	const (
		mockProjectName = "phonetool"
		testAppName     = "my-app"
		testProjectName = "my-project"
	)
	mockError := errors.New("mockError")

	tests := map[string]struct {
		skipConfirmation bool
		inAppName        string

		mockstore  func(m *mocks.Mockstore)
		mockPrompt func(m *mocks.Mockprompter)

		wantedApp   string
		wantedError error
	}{
		"should ask for app name": {
			inAppName:        "",
			skipConfirmation: true,
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices(mockProjectName).Return([]*config.Service{
					{
						Name: "my-app",
					},
					{
						Name: "test-app",
					},
				}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(appDeleteNamePrompt, "", []string{"my-app", "test-app"}).Times(1).Return("my-app", nil)
			},

			wantedApp: testAppName,
		},
		"should skip asking for app name if only one app found": {
			inAppName:        "",
			skipConfirmation: true,
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices(mockProjectName).Return([]*config.Service{
					{
						Name: "my-app",
					},
				}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedApp: testAppName,
		},
		"returns error if no application found": {
			inAppName:        "",
			skipConfirmation: true,
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices(mockProjectName).Return([]*config.Service{}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("couldn't find any application in the project phonetool"),
		},
		"returns error if fail to select application": {
			inAppName:        "",
			skipConfirmation: true,
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices(mockProjectName).Return([]*config.Service{
					{
						Name: "my-app",
					},
					{
						Name: "test-app",
					},
				}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(appDeleteNamePrompt, "", []string{"my-app", "test-app"}).Times(1).Return("", mockError)
			},

			wantedError: fmt.Errorf("select application to delete: %w", mockError),
		},
		"should skip confirmation": {
			inAppName:        testAppName,
			skipConfirmation: true,
			mockstore:        func(m *mocks.Mockstore) {},
			mockPrompt:       func(m *mocks.Mockprompter) {},

			wantedApp: testAppName,
		},
		"should wrap error returned from prompter confirmation": {
			inAppName:        testAppName,
			skipConfirmation: false,
			mockstore:        func(m *mocks.Mockstore) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(appDeleteConfirmPrompt, testAppName, mockProjectName),
					appDeleteConfirmHelp,
				).Times(1).Return(true, mockError)
			},

			wantedError: fmt.Errorf("app delete confirmation prompt: %w", mockError),
		},
		"should return error if user does not confirm app deletion": {
			inAppName:        testAppName,
			skipConfirmation: false,
			mockstore:        func(m *mocks.Mockstore) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(appDeleteConfirmPrompt, testAppName, mockProjectName),
					appDeleteConfirmHelp,
				).Times(1).Return(false, nil)
			},

			wantedError: errAppDeleteCancelled,
		},
		"should return error nil if user confirms app delete": {
			inAppName:        testAppName,
			skipConfirmation: false,
			mockstore:        func(m *mocks.Mockstore) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(appDeleteConfirmPrompt, testAppName, mockProjectName),
					appDeleteConfirmHelp,
				).Times(1).Return(true, nil)
			},

			wantedApp: testAppName,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompter := mocks.NewMockprompter(ctrl)
			mockstore := mocks.NewMockstore(ctrl)
			test.mockPrompt(mockPrompter)
			test.mockstore(mockstore)

			opts := deleteAppOpts{
				deleteAppVars: deleteAppVars{
					SkipConfirmation: test.skipConfirmation,
					GlobalOpts: &GlobalOpts{
						projectName: mockProjectName,
						prompt:      mockPrompter,
					},
					AppName: test.inAppName,
				},
				store: mockstore,
			}

			got := opts.Ask()

			if got != nil {
				require.Equal(t, test.wantedError, got)
			} else {
				require.Equal(t, test.wantedApp, opts.AppName)
			}
		})
	}
}

func TestDeleteAppOpts_getProjectEnvironments(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockstore := mocks.NewMockstore(ctrl)
	mockProjectName := "mockProjectName"
	mockEnvName := "mockEnvName"
	mockError := errors.New("mockError")
	mockEnvElement := &config.Environment{App: mockProjectName, Name: mockEnvName}
	mockEnvList := []*config.Environment{
		mockEnvElement,
	}

	tests := map[string]struct {
		setupMocks      func()
		inEnvName       string
		want            error
		wantOptsEnvList []*config.Environment
	}{
		"should wrap error returned from call to ListEnvironments()": {
			setupMocks: func() {
				mockstore.EXPECT().ListEnvironments(gomock.Eq(mockProjectName)).Times(1).Return(nil, mockError)
			},
			want:            fmt.Errorf("get environments: %w", mockError),
			wantOptsEnvList: nil,
		},
		"should set the opts environment list": {
			setupMocks: func() {
				mockstore.EXPECT().ListEnvironments(gomock.Eq(mockProjectName)).Times(1).Return(mockEnvList, nil)
			},
			want:            nil,
			wantOptsEnvList: mockEnvList,
		},
		"should set one element to opts environment list": {
			setupMocks: func() {
				mockstore.EXPECT().GetEnvironment(gomock.Eq(mockProjectName), gomock.Eq(mockEnvName)).Return(mockEnvElement, nil)
			},
			inEnvName:       mockEnvName,
			want:            nil,
			wantOptsEnvList: mockEnvList,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			test.setupMocks()
			opts := deleteAppOpts{
				deleteAppVars: deleteAppVars{
					GlobalOpts: &GlobalOpts{
						projectName: mockProjectName,
					},
					EnvName: test.inEnvName,
				},
				store: mockstore,
			}

			got := opts.getProjectEnvironments()

			require.Equal(t, test.want, got)
			require.Equal(t, test.wantOptsEnvList, opts.projectEnvironments)
		})
	}
}

type deleteAppMocks struct {
	store          *mocks.Mockstore
	secretsmanager *mocks.MocksecretsManager
	sessProvider   *session.Provider
	deployer       *mocks.MockappDeployer
	ws             *mocks.MockwsAppDeleter
	spinner        *mocks.Mockprogress
	appRemover     *mocks.MockappRemover
	imageRemover   *mocks.MockimageRemover
}

func TestDeleteAppOpts_Execute(t *testing.T) {
	mockEnvName := "test"
	mockAppName := "backend"
	mockProjectName := "badgoose"
	mockEnv := &config.Environment{
		App:            mockProjectName,
		Name:           mockEnvName,
		ManagerRoleARN: "some-arn",
		Region:         "us-west-2",
	}
	mockEnvs := []*config.Environment{mockEnv}
	mockApp := &config.Application{
		Name: mockProjectName,
	}

	mockRepo := fmt.Sprintf("%s/%s", mockProjectName, mockAppName)
	testError := errors.New("some error")

	tests := map[string]struct {
		inProjectName string
		inAppName     string
		inEnvName     string

		setupMocks func(mocks deleteAppMocks)

		wantedError error
	}{
		"happy path with no environment passed in as flag": {
			inProjectName: mockProjectName,
			inAppName:     mockAppName,
			setupMocks: func(mocks deleteAppMocks) {
				gomock.InOrder(
					// getProjectEnvironments
					mocks.store.EXPECT().ListEnvironments(gomock.Eq(mockProjectName)).Times(1).Return(mockEnvs, nil),
					// deleteStacks
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtDeleteAppStart, mockAppName, mockEnvName)),
					mocks.deployer.EXPECT().DeleteService(gomock.Any()).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtDeleteAppComplete, mockAppName, mockEnvName)),
					// emptyECRRepos
					mocks.imageRemover.EXPECT().ClearRepository(mockRepo).Return(nil),

					// removeAppProjectResources
					mocks.store.EXPECT().GetApplication(mockProjectName).Return(mockApp, nil),
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtDeleteAppResourcesStart, mockAppName, mockProjectName)),
					mocks.appRemover.EXPECT().RemoveServiceFromApp(mockApp, mockAppName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtDeleteAppResourcesComplete, mockAppName, mockProjectName)),

					// deleteSSMParam
					mocks.store.EXPECT().DeleteService(mockProjectName, mockAppName).Return(nil),

					// deleteWorkspaceFile
					mocks.ws.EXPECT().DeleteService(mockAppName).Return(nil),
				)
			},
			wantedError: nil,
		},
		"happy path with environment passed in as flag": {
			inProjectName: mockProjectName,
			inAppName:     mockAppName,
			inEnvName:     mockEnvName,
			setupMocks: func(mocks deleteAppMocks) {
				gomock.InOrder(
					// getProjectEnvironments
					mocks.store.EXPECT().GetEnvironment(mockProjectName, mockEnvName).Times(1).Return(mockEnv, nil),
					// deleteStacks
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtDeleteAppStart, mockAppName, mockEnvName)),
					mocks.deployer.EXPECT().DeleteService(gomock.Any()).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtDeleteAppComplete, mockAppName, mockEnvName)),
					// emptyECRRepos
					mocks.imageRemover.EXPECT().ClearRepository(mockRepo).Return(nil),

					// removeAppProjectResources
					mocks.store.EXPECT().GetApplication(mockProjectName).Return(mockApp, nil),
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtDeleteAppResourcesStart, mockAppName, mockProjectName)),
					mocks.appRemover.EXPECT().RemoveServiceFromApp(mockApp, mockAppName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtDeleteAppResourcesComplete, mockAppName, mockProjectName)),

					// deleteSSMParam
					mocks.store.EXPECT().DeleteService(mockProjectName, mockAppName).Return(nil),

					// deleteWorkspaceFile
					mocks.ws.EXPECT().DeleteService(mockAppName).Return(nil),
				)
			},
			wantedError: nil,
		},
		"errors when deleting stack": {
			inProjectName: mockProjectName,
			inAppName:     mockAppName,
			inEnvName:     mockEnvName,
			setupMocks: func(mocks deleteAppMocks) {
				gomock.InOrder(
					// getProjectEnvironments
					mocks.store.EXPECT().GetEnvironment(mockProjectName, mockEnvName).Times(1).Return(mockEnv, nil),
					// deleteStacks
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtDeleteAppStart, mockAppName, mockEnvName)),
					mocks.deployer.EXPECT().DeleteService(gomock.Any()).Return(testError),
					mocks.spinner.EXPECT().Stop(log.Serrorf(fmtDeleteAppFailed, mockAppName, mockEnvName, testError)),
				)
			},
			wantedError: testError,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			mockstore := mocks.NewMockstore(ctrl)
			mockSecretsManager := mocks.NewMocksecretsManager(ctrl)
			mockWorkspace := mocks.NewMockwsAppDeleter(ctrl)
			mockSession := session.NewProvider()
			mockAppDeployer := mocks.NewMockappDeployer(ctrl)
			mockAppRemover := mocks.NewMockappRemover(ctrl)
			mockSpinner := mocks.NewMockprogress(ctrl)
			mockImageRemover := mocks.NewMockimageRemover(ctrl)
			mockGetAppDeployer := func(session *awssession.Session) appDeployer {
				return mockAppDeployer
			}

			mockGetImageRemover := func(session *awssession.Session) imageRemover {
				return mockImageRemover
			}

			mocks := deleteAppMocks{
				store:          mockstore,
				secretsmanager: mockSecretsManager,
				ws:             mockWorkspace,
				sessProvider:   mockSession,
				deployer:       mockAppDeployer,
				spinner:        mockSpinner,
				appRemover:     mockAppRemover,
				imageRemover:   mockImageRemover,
			}

			test.setupMocks(mocks)

			opts := deleteAppOpts{
				deleteAppVars: deleteAppVars{
					GlobalOpts: &GlobalOpts{
						projectName: test.inProjectName,
					},
					AppName: test.inAppName,
					EnvName: test.inEnvName,
				},
				store:            mockstore,
				workspaceService: mockWorkspace,
				sessProvider:     mockSession,
				spinner:          mockSpinner,
				appRemover:       mockAppRemover,
				getAppDeployer:   mockGetAppDeployer,
				getImageRemover:  mockGetImageRemover,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if test.wantedError != nil {
				require.EqualError(t, err, test.wantedError.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}

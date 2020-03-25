// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	awsmocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/session"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	awsSession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeleteAppOpts_Validate(t *testing.T) {
	mockError := errors.New("some error")

	tests := map[string]struct {
		inProjectName string
		inAppName     string
		inEnvName     string
		setupMocks    func(m *climocks.MockprojectService)

		want error
	}{
		"should return errNoProjectInWorkspace": {
			setupMocks: func(m *climocks.MockprojectService) {},
			inAppName:  "my-app",
			want:       errNoProjectInWorkspace,
		},
		"with no flag set": {
			inProjectName: "phonetool",
			setupMocks:    func(m *climocks.MockprojectService) {},
			want:          nil,
		},
		"with all flag set": {
			inProjectName: "phonetool",
			inAppName:     "my-app",
			setupMocks: func(m *climocks.MockprojectService) {
				m.EXPECT().GetApplication("phonetool", "my-app").Times(1).Return(&archer.Application{
					Name: "my-app",
				}, nil)
			},
			want: nil,
		},
		"with env flag set": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			setupMocks: func(m *climocks.MockprojectService) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(&archer.Environment{Name: "test"}, nil)
			},
			want: nil,
		},
		"with unknown environment": {
			inProjectName: "phonetool",
			inEnvName:     "test",
			setupMocks: func(m *climocks.MockprojectService) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(nil, errors.New("unknown env"))
			},
			want: errors.New("get environment test from metadata store: unknown env"),
		},
		"should return error if fail to get app name": {
			inProjectName: "phonetool",
			inAppName:     "my-app",
			setupMocks: func(m *climocks.MockprojectService) {
				m.EXPECT().GetApplication("phonetool", "my-app").Times(1).Return(nil, mockError)
			},
			want: errors.New("some error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockProjectService := climocks.NewMockprojectService(ctrl)

			test.setupMocks(mockProjectService)

			opts := deleteAppOpts{
				deleteAppVars: deleteAppVars{
					GlobalOpts: &GlobalOpts{
						projectName: test.inProjectName,
					},
					AppName: test.inAppName,
					EnvName: test.inEnvName,
				},
				projectService: mockProjectService,
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

		mockProjectService func(m *climocks.MockprojectService)
		mockPrompt         func(m *climocks.Mockprompter)

		wantedApp   string
		wantedError error
	}{
		"should ask for app name": {
			inAppName:        "",
			skipConfirmation: true,
			mockProjectService: func(m *climocks.MockprojectService) {
				m.EXPECT().ListApplications(mockProjectName).Return([]*archer.Application{
					&archer.Application{
						Name: "my-app",
					},
					&archer.Application{
						Name: "test-app",
					},
				}, nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(appDeleteNamePrompt, "", []string{"my-app", "test-app"}).Times(1).Return("my-app", nil)
			},

			wantedApp: testAppName,
		},
		"should skip asking for app name if only one app found": {
			inAppName:        "",
			skipConfirmation: true,
			mockProjectService: func(m *climocks.MockprojectService) {
				m.EXPECT().ListApplications(mockProjectName).Return([]*archer.Application{
					&archer.Application{
						Name: "my-app",
					},
				}, nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {},

			wantedApp: testAppName,
		},
		"returns error if no application found": {
			inAppName:        "",
			skipConfirmation: true,
			mockProjectService: func(m *climocks.MockprojectService) {
				m.EXPECT().ListApplications(mockProjectName).Return([]*archer.Application{}, nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {},

			wantedError: fmt.Errorf("couldn't find any application in the project phonetool"),
		},
		"returns error if fail to select application": {
			inAppName:        "",
			skipConfirmation: true,
			mockProjectService: func(m *climocks.MockprojectService) {
				m.EXPECT().ListApplications(mockProjectName).Return([]*archer.Application{
					&archer.Application{
						Name: "my-app",
					},
					&archer.Application{
						Name: "test-app",
					},
				}, nil)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(appDeleteNamePrompt, "", []string{"my-app", "test-app"}).Times(1).Return("", mockError)
			},

			wantedError: fmt.Errorf("select application to delete: %w", mockError),
		},
		"should skip confirmation": {
			inAppName:          testAppName,
			skipConfirmation:   true,
			mockProjectService: func(m *climocks.MockprojectService) {},
			mockPrompt:         func(m *climocks.Mockprompter) {},

			wantedApp: testAppName,
		},
		"should wrap error returned from prompter confirmation": {
			inAppName:          testAppName,
			skipConfirmation:   false,
			mockProjectService: func(m *climocks.MockprojectService) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(appDeleteConfirmPrompt, testAppName, mockProjectName),
					appDeleteConfirmHelp,
				).Times(1).Return(true, mockError)
			},

			wantedError: fmt.Errorf("app delete confirmation prompt: %w", mockError),
		},
		"should return error if user does not confirm app deletion": {
			inAppName:          testAppName,
			skipConfirmation:   false,
			mockProjectService: func(m *climocks.MockprojectService) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(appDeleteConfirmPrompt, testAppName, mockProjectName),
					appDeleteConfirmHelp,
				).Times(1).Return(false, nil)
			},

			wantedError: errAppDeleteCancelled,
		},
		"should return error nil if user confirms app delete": {
			inAppName:          testAppName,
			skipConfirmation:   false,
			mockProjectService: func(m *climocks.MockprojectService) {},
			mockPrompt: func(m *climocks.Mockprompter) {
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

			mockPrompter := climocks.NewMockprompter(ctrl)
			mockProjectService := climocks.NewMockprojectService(ctrl)
			test.mockPrompt(mockPrompter)
			test.mockProjectService(mockProjectService)

			opts := deleteAppOpts{
				deleteAppVars: deleteAppVars{
					SkipConfirmation: test.skipConfirmation,
					GlobalOpts: &GlobalOpts{
						projectName: mockProjectName,
						prompt:      mockPrompter,
					},
					AppName: test.inAppName,
				},
				projectService: mockProjectService,
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

	mockProjectService := climocks.NewMockprojectService(ctrl)
	mockProjectName := "mockProjectName"
	mockEnvName := "mockEnvName"
	mockError := errors.New("mockError")
	mockEnvElement := &archer.Environment{Project: mockProjectName, Name: mockEnvName}
	mockEnvList := []*archer.Environment{
		mockEnvElement,
	}

	tests := map[string]struct {
		setupMocks      func()
		inEnvName       string
		want            error
		wantOptsEnvList []*archer.Environment
	}{
		"should wrap error returned from call to ListEnvironments()": {
			setupMocks: func() {
				mockProjectService.EXPECT().ListEnvironments(gomock.Eq(mockProjectName)).Times(1).Return(nil, mockError)
			},
			want:            fmt.Errorf("get environments: %w", mockError),
			wantOptsEnvList: nil,
		},
		"should set the opts environment list": {
			setupMocks: func() {
				mockProjectService.EXPECT().ListEnvironments(gomock.Eq(mockProjectName)).Times(1).Return(mockEnvList, nil)
			},
			want:            nil,
			wantOptsEnvList: mockEnvList,
		},
		"should set one element to opts environment list": {
			setupMocks: func() {
				mockProjectService.EXPECT().GetEnvironment(gomock.Eq(mockProjectName), gomock.Eq(mockEnvName)).Return(mockEnvElement, nil)
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
				projectService: mockProjectService,
			}

			got := opts.getProjectEnvironments()

			require.Equal(t, test.want, got)
			require.Equal(t, test.wantOptsEnvList, opts.projectEnvironments)
		})
	}
}

type deleteAppMocks struct {
	projectService *climocks.MockprojectService
	secretsmanager *awsmocks.MockSecretsManager
	sessProvider   *session.Provider
	deployer       *climocks.MockappDeployer
	ws             *climocks.MockwsAppDeleter
	spinner        *climocks.Mockprogress
	appRemover     *climocks.MockappRemover
	imageRemover   *climocks.MockimageRemover
}

func TestDeleteAppOpts_Execute(t *testing.T) {
	mockEnvName := "test"
	mockAppName := "backend"
	mockProjectName := "badgoose"
	mockEnv := &archer.Environment{
		Project:        mockProjectName,
		Name:           mockEnvName,
		ManagerRoleARN: "some-arn",
		Region:         "us-west-2",
	}
	mockEnvs := []*archer.Environment{mockEnv}
	mockProject := &archer.Project{
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
					mocks.projectService.EXPECT().ListEnvironments(gomock.Eq(mockProjectName)).Times(1).Return(mockEnvs, nil),
					// deleteStacks
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtDeleteAppStart, mockAppName, mockEnvName)),
					mocks.deployer.EXPECT().DeleteApp(gomock.Any()).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtDeleteAppComplete, mockAppName, mockEnvName)),
					// emptyECRRepos
					mocks.imageRemover.EXPECT().ClearRepository(mockRepo).Return(nil),

					// removeAppProjectResources
					mocks.projectService.EXPECT().GetProject(mockProjectName).Return(mockProject, nil),
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtDeleteAppResourcesStart, mockAppName, mockProjectName)),
					mocks.appRemover.EXPECT().RemoveAppFromProject(mockProject, mockAppName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtDeleteAppResourcesComplete, mockAppName, mockProjectName)),

					// deleteSSMParam
					mocks.projectService.EXPECT().DeleteApplication(mockProjectName, mockAppName).Return(nil),

					// deleteWorkspaceFile
					mocks.ws.EXPECT().DeleteApp(mockAppName).Return(nil),
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
					mocks.projectService.EXPECT().GetEnvironment(mockProjectName, mockEnvName).Times(1).Return(mockEnv, nil),
					// deleteStacks
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtDeleteAppStart, mockAppName, mockEnvName)),
					mocks.deployer.EXPECT().DeleteApp(gomock.Any()).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtDeleteAppComplete, mockAppName, mockEnvName)),
					// emptyECRRepos
					mocks.imageRemover.EXPECT().ClearRepository(mockRepo).Return(nil),

					// removeAppProjectResources
					mocks.projectService.EXPECT().GetProject(mockProjectName).Return(mockProject, nil),
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtDeleteAppResourcesStart, mockAppName, mockProjectName)),
					mocks.appRemover.EXPECT().RemoveAppFromProject(mockProject, mockAppName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtDeleteAppResourcesComplete, mockAppName, mockProjectName)),

					// deleteSSMParam
					mocks.projectService.EXPECT().DeleteApplication(mockProjectName, mockAppName).Return(nil),

					// deleteWorkspaceFile
					mocks.ws.EXPECT().DeleteApp(mockAppName).Return(nil),
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
					mocks.projectService.EXPECT().GetEnvironment(mockProjectName, mockEnvName).Times(1).Return(mockEnv, nil),
					// deleteStacks
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtDeleteAppStart, mockAppName, mockEnvName)),
					mocks.deployer.EXPECT().DeleteApp(gomock.Any()).Return(testError),
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
			mockProjectService := climocks.NewMockprojectService(ctrl)
			mockSecretsManager := awsmocks.NewMockSecretsManager(ctrl)
			mockWorkspace := climocks.NewMockwsAppDeleter(ctrl)
			mockSession := session.NewProvider()
			mockAppDeployer := climocks.NewMockappDeployer(ctrl)
			mockAppRemover := climocks.NewMockappRemover(ctrl)
			mockSpinner := climocks.NewMockprogress(ctrl)
			mockImageRemover := climocks.NewMockimageRemover(ctrl)

			oldGetAppDeployer := getAppDeployer
			defer func() { getAppDeployer = oldGetAppDeployer }()
			getAppDeployer = func(session *awsSession.Session) appDeployer {
				return mockAppDeployer
			}

			oldGetImageRemover := getImageRemover
			defer func() { getImageRemover = oldGetImageRemover }()
			getImageRemover = func(session *awsSession.Session) imageRemover {
				return mockImageRemover
			}

			mocks := deleteAppMocks{
				projectService: mockProjectService,
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
				projectService:   mockProjectService,
				workspaceService: mockWorkspace,
				sessProvider:     mockSession,
				spinner:          mockSpinner,
				appRemover:       mockAppRemover,
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

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	awsecs "github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/ecs"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type execSvcMocks struct {
	storeSvc           *mocks.Mockstore
	sessProvider       *mocks.MocksessionProvider
	sel                *mocks.MockdeploySelector
	ecsSvcDescriber    *mocks.MockserviceDescriber
	ecsCommandExecutor *mocks.MockecsCommandExecutor
	ssmPluginManager   *mocks.MockssmPluginManager
	prompter           *mocks.Mockprompter
}

func TestSvcExec_Validate(t *testing.T) {
	const (
		mockApp = "my-app"
		mockEnv = "my-env"
		mockSvc = "my-svc"
	)
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		inputApp         string
		inputEnv         string
		inputSvc         string
		skipConfirmation *bool
		setupMocks       func(mocks execSvcMocks)

		wantedError error
	}{
		"skip without installing/updating if yes flag is set to be false": {
			inputApp:         mockApp,
			inputEnv:         mockEnv,
			inputSvc:         mockSvc,
			skipConfirmation: aws.Bool(false),
			setupMocks:       func(m execSvcMocks) {},
		},
		"should bubble error if cannot validate ssm plugin": {
			inputApp: mockApp,
			inputEnv: mockEnv,
			inputSvc: mockSvc,
			setupMocks: func(m execSvcMocks) {
				m.ssmPluginManager.EXPECT().ValidateBinary().Return(mockErr)
			},

			wantedError: fmt.Errorf("validate ssm plugin: some error"),
		},
		"should bubble error if cannot prompt to confirm install": {
			inputApp: mockApp,
			inputEnv: mockEnv,
			inputSvc: mockSvc,
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.ssmPluginManager.EXPECT().ValidateBinary().Return(&exec.ErrSSMPluginNotExist{}),
					m.prompter.EXPECT().Confirm(ssmPluginInstallPrompt, ssmPluginInstallPromptHelp).Return(false, mockErr),
				)
			},

			wantedError: fmt.Errorf("prompt to confirm installing the plugin: some error"),
		},
		"should bubble error if cannot confirm install": {
			inputApp: mockApp,
			inputEnv: mockEnv,
			inputSvc: mockSvc,
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.ssmPluginManager.EXPECT().ValidateBinary().Return(&exec.ErrSSMPluginNotExist{}),
					m.prompter.EXPECT().Confirm(ssmPluginInstallPrompt, ssmPluginInstallPromptHelp).
						Return(false, nil),
				)
			},

			wantedError: errSSMPluginCommandInstallCancelled,
		},
		"should bubble error if cannot install binary": {
			inputApp: mockApp,
			inputEnv: mockEnv,
			inputSvc: mockSvc,
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.ssmPluginManager.EXPECT().ValidateBinary().Return(&exec.ErrSSMPluginNotExist{}),
					m.prompter.EXPECT().Confirm(ssmPluginInstallPrompt, ssmPluginInstallPromptHelp).
						Return(true, nil),
					m.ssmPluginManager.EXPECT().InstallLatestBinary().Return(mockErr),
				)
			},

			wantedError: fmt.Errorf("install ssm plugin: some error"),
		},
		"should bubble error if cannot prompt to confirm update": {
			inputApp: mockApp,
			inputEnv: mockEnv,
			inputSvc: mockSvc,
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.ssmPluginManager.EXPECT().ValidateBinary().Return(&exec.ErrOutdatedSSMPlugin{
						CurrentVersion: "mockCurrentVersion",
						LatestVersion:  "mockLatestVersion",
					}),
					m.prompter.EXPECT().Confirm(fmt.Sprintf(ssmPluginUpdatePrompt, "mockCurrentVersion", "mockLatestVersion"), "").
						Return(false, mockErr),
				)
			},

			wantedError: fmt.Errorf("prompt to confirm updating the plugin: some error"),
		},
		"should proceed if cannot confirm update": {
			inputApp: mockApp,
			inputEnv: mockEnv,
			inputSvc: mockSvc,
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.ssmPluginManager.EXPECT().ValidateBinary().Return(&exec.ErrOutdatedSSMPlugin{
						CurrentVersion: "mockCurrentVersion",
						LatestVersion:  "mockLatestVersion",
					}),
					m.prompter.EXPECT().Confirm(fmt.Sprintf(ssmPluginUpdatePrompt, "mockCurrentVersion", "mockLatestVersion"), "").
						Return(false, nil),
				)
			},
		},
		"should bubble error if cannot update the binary": {
			inputApp: mockApp,
			inputEnv: mockEnv,
			inputSvc: mockSvc,
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.ssmPluginManager.EXPECT().ValidateBinary().Return(&exec.ErrOutdatedSSMPlugin{
						CurrentVersion: "mockCurrentVersion",
						LatestVersion:  "mockLatestVersion",
					}),
					m.prompter.EXPECT().Confirm(fmt.Sprintf(ssmPluginUpdatePrompt, "mockCurrentVersion", "mockLatestVersion"), "").
						Return(true, nil),
					m.ssmPluginManager.EXPECT().InstallLatestBinary().Return(mockErr),
				)
			},

			wantedError: fmt.Errorf("update ssm plugin: some error"),
		},
		"skip validation if app flag is not set": {
			inputEnv: mockEnv,
			inputSvc: mockSvc,
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.ssmPluginManager.EXPECT().ValidateBinary().Return(nil),
				)
			},
		},
		"valid case": {
			inputApp: mockApp,
			inputEnv: mockEnv,
			inputSvc: mockSvc,
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.ssmPluginManager.EXPECT().ValidateBinary().Return(nil),
				)
			},

			wantedError: nil,
		},
		"valid case with ssm plugin installing": {
			inputApp: mockApp,
			inputEnv: mockEnv,
			inputSvc: mockSvc,
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.ssmPluginManager.EXPECT().ValidateBinary().Return(&exec.ErrSSMPluginNotExist{}),
					m.prompter.EXPECT().Confirm(ssmPluginInstallPrompt, ssmPluginInstallPromptHelp).Return(true, nil),
					m.ssmPluginManager.EXPECT().InstallLatestBinary().Return(nil),
				)
			},

			wantedError: nil,
		},
		"valid case with ssm plugin updating and skip confirming to install": {
			inputApp:         mockApp,
			inputEnv:         mockEnv,
			inputSvc:         mockSvc,
			skipConfirmation: aws.Bool(true),
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.ssmPluginManager.EXPECT().ValidateBinary().Return(&exec.ErrOutdatedSSMPlugin{
						CurrentVersion: "mockCurrentVersion",
						LatestVersion:  "mockLatestVersion",
					}),
					m.ssmPluginManager.EXPECT().InstallLatestBinary().Return(nil),
				)
			},

			wantedError: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			mockSSMPluginManager := mocks.NewMockssmPluginManager(ctrl)
			mockPrompter := mocks.NewMockprompter(ctrl)
			mocks := execSvcMocks{
				storeSvc:         mockStoreReader,
				ssmPluginManager: mockSSMPluginManager,
				prompter:         mockPrompter,
			}

			tc.setupMocks(mocks)

			execSvcs := &svcExecOpts{
				execVars: execVars{
					name:             tc.inputSvc,
					appName:          tc.inputApp,
					envName:          tc.inputEnv,
					skipConfirmation: tc.skipConfirmation,
				},
				store:            mockStoreReader,
				ssmPluginManager: mockSSMPluginManager,
				prompter:         mockPrompter,
			}

			// WHEN
			err := execSvcs.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSvcExec_Ask(t *testing.T) {
	const (
		inputApp = "my-app"
		inputEnv = "my-env"
		inputSvc = "my-svc"
	)
	testCases := map[string]struct {
		inputApp string
		inputEnv string
		inputSvc string

		setupMocks func(mocks execSvcMocks)

		wantedApp   string
		wantedEnv   string
		wantedSvc   string
		wantedError error
	}{
		"validate app env and svc with all flags passed in": {
			inputApp: inputApp,
			inputEnv: inputEnv,
			inputSvc: inputSvc,
			setupMocks: func(m execSvcMocks) {
				m.storeSvc.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
				m.storeSvc.EXPECT().GetEnvironment("my-app", "my-env").Return(&config.Environment{Name: "my-env"}, nil)
				m.storeSvc.EXPECT().GetService("my-app", "my-svc").Return(&config.Workload{}, nil)
				m.sel.EXPECT().DeployedService(svcExecNamePrompt, svcExecNameHelpPrompt, "my-app", gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env:  "my-env",
						Name: "my-svc",
					}, nil) // Let prompter handles the case when svc(env) is definite.
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedSvc: inputSvc,
		},
		"prompt for app name": {
			inputEnv: inputEnv,
			inputSvc: inputSvc,
			setupMocks: func(m execSvcMocks) {
				m.sel.EXPECT().Application(svcAppNamePrompt, wkldAppNameHelpPrompt).Return("my-app", nil)
				m.storeSvc.EXPECT().GetApplication(gomock.Any()).Times(0)
				// Don't care about the other calls.
				m.storeSvc.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.storeSvc.EXPECT().GetService(gomock.Any(), gomock.Any()).AnyTimes()
				m.sel.EXPECT().DeployedService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(&selector.DeployedService{
					Env:  "my-env",
					Name: "my-svc",
				}, nil).AnyTimes()
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedSvc: inputSvc,
		},
		"returns error when fail to select apps": {
			setupMocks: func(m execSvcMocks) {
				m.sel.EXPECT().Application(svcAppNamePrompt, wkldAppNameHelpPrompt).Return("", errors.New("some error"))
			},
			wantedError: fmt.Errorf("select application: some error"),
		},
		"prompt for svc and env": {
			inputApp: inputApp,
			setupMocks: func(m execSvcMocks) {
				m.storeSvc.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.storeSvc.EXPECT().GetService(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().DeployedService(svcExecNamePrompt, svcExecNameHelpPrompt, "my-app", gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env:  "my-env",
						Name: "my-svc",
					}, nil)
				// Don't care about the other calls.
				m.storeSvc.EXPECT().GetApplication(gomock.Any()).AnyTimes()
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedSvc: inputSvc,
		},
		"returns error when fail to select services": {
			inputApp: inputApp,
			setupMocks: func(m execSvcMocks) {
				m.sel.EXPECT().DeployedService(svcExecNamePrompt, svcExecNameHelpPrompt, "my-app", gomock.Any(), gomock.Any()).
					Return(nil, fmt.Errorf("some error"))
				// Don't care about the other calls.
				m.storeSvc.EXPECT().GetApplication(gomock.Any()).AnyTimes()
			},

			wantedError: fmt.Errorf("select deployed service for application my-app: some error"),
		},
		"success": {
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.sel.EXPECT().Application(svcAppNamePrompt, wkldAppNameHelpPrompt).Return("my-app", nil),
					m.sel.EXPECT().DeployedService(svcExecNamePrompt, svcExecNameHelpPrompt, "my-app", gomock.Any(), gomock.Any()).
						Return(&selector.DeployedService{
							Env:  "my-env",
							Name: "my-svc",
						}, nil),
				)
			},
			wantedApp: inputApp,
			wantedEnv: inputEnv,
			wantedSvc: inputSvc,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			mockSelector := mocks.NewMockdeploySelector(ctrl)

			mocks := execSvcMocks{
				storeSvc: mockStoreReader,
				sel:      mockSelector,
			}

			tc.setupMocks(mocks)

			execSvcs := &svcExecOpts{
				execVars: execVars{
					name:    tc.inputSvc,
					envName: tc.inputEnv,
					appName: tc.inputApp,
				},
				store: mockStoreReader,
				sel:   mockSelector,
			}

			// WHEN
			err := execSvcs.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedApp, execSvcs.appName, "expected app name to match")
				require.Equal(t, tc.wantedSvc, execSvcs.name, "expected service name to match")
				require.Equal(t, tc.wantedEnv, execSvcs.envName, "expected service name to match")
			}
		})
	}
}

func TestSvcExec_Execute(t *testing.T) {
	const (
		mockTaskARN      = "arn:aws:ecs:us-west-2:123456789:task/mockCluster/mockTaskID"
		mockOtherTaskARN = "arn:aws:ecs:us-west-2:123456789:task/mockCluster/mockTaskID1"
	)
	mockWl := config.Workload{
		App:  "mockApp",
		Name: "mockSvc",
		Type: "Load Balanced Web Service",
	}
	mockRDWSWl := config.Workload{
		App:  "mockApp",
		Name: "mockSvc",
		Type: "Request-Driven Web Service",
	}
	mockError := errors.New("some error")
	testCases := map[string]struct {
		containerName string
		taskID        string
		setupMocks    func(mocks execSvcMocks)

		wantedError error
	}{
		"return error if fail to get workload": {
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetWorkload("mockApp", "mockSvc").Return(nil, mockError),
				)
			},
			wantedError: fmt.Errorf("get workload: some error"),
		},
		"return error if service type is Request-Driven Web Service": {
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetWorkload("mockApp", "mockSvc").Return(&mockRDWSWl, nil),
				)
			},
			wantedError: fmt.Errorf("executing a command in a running container part of a service is not supported for services with type: 'Request-Driven Web Service'"),
		},
		"return error if fail to get environment": {
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetWorkload("mockApp", "mockSvc").Return(&mockWl, nil),
					m.storeSvc.EXPECT().GetEnvironment("mockApp", "mockEnv").Return(nil, mockError),
				)
			},
			wantedError: fmt.Errorf("get environment mockEnv: some error"),
		},
		"return error if fail to describe service": {
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetWorkload("mockApp", "mockSvc").Return(&mockWl, nil),
					m.storeSvc.EXPECT().GetEnvironment("mockApp", "mockEnv").Return(&config.Environment{
						Name: "my-env",
					}, nil),
					m.sessProvider.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{
						Config: &aws.Config{
							Region: aws.String("mockRegion"),
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(nil, mockError),
				)
			},
			wantedError: fmt.Errorf("describe ECS service for mockSvc in environment mockEnv: some error"),
		},
		"return error if no running task found": {
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetWorkload("mockApp", "mockSvc").Return(&mockWl, nil),
					m.storeSvc.EXPECT().GetEnvironment("mockApp", "mockEnv").Return(&config.Environment{
						Name: "my-env",
					}, nil),
					m.sessProvider.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{
						Config: &aws.Config{
							Region: aws.String("mockRegion"),
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(&ecs.ServiceDesc{
						Tasks: []*awsecs.Task{},
					}, nil),
				)
			},
			wantedError: fmt.Errorf("found no running task for service mockSvc in environment mockEnv"),
		},
		"return error if fail to find prefixed task": {
			taskID: "mockTaskID1",
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetWorkload("mockApp", "mockSvc").Return(&mockWl, nil),
					m.storeSvc.EXPECT().GetEnvironment("mockApp", "mockEnv").Return(&config.Environment{
						Name: "my-env",
					}, nil),
					m.sessProvider.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{
						Config: &aws.Config{
							Region: aws.String("mockRegion"),
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(&ecs.ServiceDesc{
						Tasks: []*awsecs.Task{
							{
								TaskArn:    aws.String(mockTaskARN),
								LastStatus: aws.String("RUNNING"),
							},
						},
					}, nil),
				)
			},
			wantedError: fmt.Errorf("found no running task whose ID is prefixed with mockTaskID1"),
		},
		"return error if fail to execute command": {
			containerName: "hello",
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetWorkload("mockApp", "mockSvc").Return(&mockWl, nil),
					m.storeSvc.EXPECT().GetEnvironment("mockApp", "mockEnv").Return(&config.Environment{
						Name: "my-env",
					}, nil),
					m.sessProvider.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{
						Config: &aws.Config{
							Region: aws.String("mockRegion"),
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(&ecs.ServiceDesc{
						ClusterName: "mockCluster",
						Tasks: []*awsecs.Task{
							{
								TaskArn:    aws.String(mockTaskARN),
								LastStatus: aws.String("RUNNING"),
							},
						},
					}, nil),
					m.ecsCommandExecutor.EXPECT().ExecuteCommand(awsecs.ExecuteCommandInput{
						Cluster:   "mockCluster",
						Container: "hello",
						Task:      "mockTaskID",
						Command:   "mockCommand",
					}).Return(mockError),
				)
			},
			wantedError: fmt.Errorf("execute command mockCommand in container hello: some error"),
		},
		"success": {
			setupMocks: func(m execSvcMocks) {
				gomock.InOrder(
					m.storeSvc.EXPECT().GetWorkload("mockApp", "mockSvc").Return(&mockWl, nil),
					m.storeSvc.EXPECT().GetEnvironment("mockApp", "mockEnv").Return(&config.Environment{
						Name: "my-env",
					}, nil),
					m.sessProvider.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{
						Config: &aws.Config{
							Region: aws.String("mockRegion"),
						},
					}, nil),
					m.ecsSvcDescriber.EXPECT().DescribeService("mockApp", "mockEnv", "mockSvc").Return(&ecs.ServiceDesc{
						ClusterName: "mockCluster",
						Tasks: []*awsecs.Task{
							{
								TaskArn:    aws.String(mockTaskARN),
								LastStatus: aws.String("RUNNING"),
							},
							{
								TaskArn:    aws.String(mockOtherTaskARN),
								LastStatus: aws.String("RUNNING"),
							},
						},
					}, nil),
					m.ecsCommandExecutor.EXPECT().ExecuteCommand(awsecs.ExecuteCommandInput{
						Cluster:   "mockCluster",
						Container: "mockSvc",
						Task:      "mockTaskID",
						Command:   "mockCommand",
					}).Return(nil),
				)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			mockSvcDescriber := mocks.NewMockserviceDescriber(ctrl)
			mockCommandExecutor := mocks.NewMockecsCommandExecutor(ctrl)
			mockSessionProvider := mocks.NewMocksessionProvider(ctrl)
			mockNewSvcDescriber := func(_ *session.Session) serviceDescriber {
				return mockSvcDescriber
			}
			mockNewCommandExecutor := func(_ *session.Session) ecsCommandExecutor {
				return mockCommandExecutor
			}

			mocks := execSvcMocks{
				storeSvc:           mockStoreReader,
				ecsCommandExecutor: mockCommandExecutor,
				ecsSvcDescriber:    mockSvcDescriber,
				sessProvider:       mockSessionProvider,
			}

			tc.setupMocks(mocks)

			execSvcs := &svcExecOpts{
				execVars: execVars{
					name:          "mockSvc",
					envName:       "mockEnv",
					appName:       "mockApp",
					command:       "mockCommand",
					containerName: tc.containerName,
					taskID:        tc.taskID,
				},
				store:              mockStoreReader,
				newSvcDescriber:    mockNewSvcDescriber,
				newCommandExecutor: mockNewCommandExecutor,
				randInt:            func(i int) int { return 0 },
				sessProvider:       mockSessionProvider,
			}

			// WHEN
			err := execSvcs.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	awsecs "github.com/aws/aws-sdk-go/service/ecs"
	"github.com/aws/copilot-cli/internal/pkg/aws/ecs"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type execTaskMocks struct {
	storeSvc         *mocks.Mockstore
	configSel        *mocks.MockappEnvSelector
	taskSel          *mocks.MockrunningTaskSelector
	commandExec      *mocks.MockecsCommandExecutor
	ssmPluginManager *mocks.MockssmPluginManager
	provider         *mocks.MocksessionProvider
}

func TestTaskExec_Validate(t *testing.T) {
	const (
		mockApp = "my-app"
		mockEnv = "my-env"
	)
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		inApp       string
		inEnv       string
		inTaskGroup string
		useDefault  bool
		setupMocks  func(mocks execTaskMocks)

		wantedError error
	}{
		"should bubble error if specify both default and app": {
			inApp:      mockApp,
			useDefault: true,
			setupMocks: func(m execTaskMocks) {},

			wantedError: fmt.Errorf("cannot specify both default flag and app or env flags"),
		},
		"should bubble error if specify both default and env": {
			inEnv:      mockEnv,
			useDefault: true,
			setupMocks: func(m execTaskMocks) {},

			wantedError: fmt.Errorf("cannot specify both default flag and app or env flags"),
		},
		"should bubble error if failed to get app": {
			inApp: mockApp,
			setupMocks: func(m execTaskMocks) {
				m.storeSvc.EXPECT().GetApplication(mockApp).Return(nil, mockErr)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"should bubble error if failed to get env": {
			inApp: mockApp,
			inEnv: mockEnv,
			setupMocks: func(m execTaskMocks) {
				m.storeSvc.EXPECT().GetApplication(mockApp).Return(&config.Application{}, nil)
				m.storeSvc.EXPECT().GetEnvironment(mockApp, mockEnv).Return(nil, mockErr)
			},

			wantedError: fmt.Errorf("some error"),
		},
		"skip validation if app flag is not set": {
			inEnv: mockEnv,
			setupMocks: func(m execTaskMocks) {
				m.ssmPluginManager.EXPECT().ValidateBinary().Return(nil)
			},
		},
		"success": {
			inApp: mockApp,
			inEnv: mockEnv,
			setupMocks: func(m execTaskMocks) {
				m.storeSvc.EXPECT().GetApplication(mockApp).Return(&config.Application{}, nil)
				m.storeSvc.EXPECT().GetEnvironment(mockApp, mockEnv).Return(&config.Environment{}, nil)
				m.ssmPluginManager.EXPECT().ValidateBinary().Return(nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			mockSSMValidator := mocks.NewMockssmPluginManager(ctrl)
			mocks := execTaskMocks{
				storeSvc:         mockStoreReader,
				ssmPluginManager: mockSSMValidator,
			}

			tc.setupMocks(mocks)

			execTasks := &taskExecOpts{
				taskExecVars: taskExecVars{
					execVars: execVars{
						name:    tc.inTaskGroup,
						appName: tc.inApp,
						envName: tc.inEnv,
					},
					useDefault: tc.useDefault,
				},
				store:            mockStoreReader,
				ssmPluginManager: mockSSMValidator,
			}

			// WHEN
			err := execTasks.Validate()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTaskExec_Ask(t *testing.T) {
	const (
		mockApp = "my-app"
		mockEnv = "my-env"
	)
	mockTask := &ecs.Task{
		TaskArn: aws.String("mockTaskARN"),
	}
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		inApp       string
		inEnv       string
		inTaskGroup string
		inTaskID    string
		useDefault  bool
		setupMocks  func(mocks execTaskMocks)

		wantedError      error
		wantedTask       *ecs.Task
		wantedUseDefault bool
	}{
		"should bubble error if fail to select task in default cluster": {
			useDefault: true,
			setupMocks: func(m execTaskMocks) {
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
				m.taskSel.EXPECT().RunningTask(taskExecTaskPrompt, taskExecTaskHelpPrompt,
					gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, mockErr)
			},

			wantedError: fmt.Errorf("select running task in default cluster: some error"),
		},
		"should bubble error if fail to select application": {
			setupMocks: func(m execTaskMocks) {
				m.configSel.EXPECT().Application(taskExecAppNamePrompt, taskExecAppNameHelpPrompt, useDefaultClusterOption).
					Return("", mockErr)
			},

			wantedError: fmt.Errorf("select application: some error"),
		},
		"should bubble error if fail to select environment": {
			inApp: mockApp,
			setupMocks: func(m execTaskMocks) {
				m.configSel.EXPECT().Environment(taskExecEnvNamePrompt, taskExecEnvNameHelpPrompt, mockApp, prompt.Option{Value: useDefaultClusterOption}).
					Return("", mockErr)
			},

			wantedError: fmt.Errorf("select environment: some error"),
		},
		"should bubble error if fail to get environment": {
			inApp: mockApp,
			inEnv: mockEnv,
			setupMocks: func(m execTaskMocks) {
				m.storeSvc.EXPECT().GetEnvironment(mockApp, mockEnv).Return(nil, mockErr)
			},

			wantedError: fmt.Errorf("get environment my-env: some error"),
		},
		"should bubble error if fail to select running task in env cluster": {
			inApp: mockApp,
			inEnv: mockEnv,
			setupMocks: func(m execTaskMocks) {
				m.storeSvc.EXPECT().GetEnvironment(mockApp, mockEnv).Return(&config.Environment{}, nil)
				m.provider.EXPECT().FromRole(gomock.Any(), gomock.Any())
				m.taskSel.EXPECT().RunningTask(taskExecTaskPrompt, taskExecTaskHelpPrompt,
					gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, mockErr)
			},

			wantedError: fmt.Errorf("select running task in environment my-env: some error"),
		},
		"success with default flag set": {
			useDefault: true,
			setupMocks: func(m execTaskMocks) {
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
				m.taskSel.EXPECT().RunningTask(taskExecTaskPrompt, taskExecTaskHelpPrompt,
					gomock.Any(), gomock.Any(), gomock.Any()).Return(mockTask, nil)
			},

			wantedTask:       mockTask,
			wantedUseDefault: true,
		},
		"success with default option chose": {
			setupMocks: func(m execTaskMocks) {
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
				m.configSel.EXPECT().Application(taskExecAppNamePrompt, taskExecAppNameHelpPrompt, useDefaultClusterOption).
					Return(useDefaultClusterOption, nil)
				m.taskSel.EXPECT().RunningTask(taskExecTaskPrompt, taskExecTaskHelpPrompt,
					gomock.Any(), gomock.Any(), gomock.Any()).Return(mockTask, nil)
			},

			wantedTask:       mockTask,
			wantedUseDefault: true,
		},
		"success with env cluster": {
			setupMocks: func(m execTaskMocks) {
				m.configSel.EXPECT().Application(taskExecAppNamePrompt, taskExecAppNameHelpPrompt, useDefaultClusterOption).
					Return(mockApp, nil)
				m.configSel.EXPECT().Environment(taskExecEnvNamePrompt, taskExecEnvNameHelpPrompt, mockApp, prompt.Option{Value: useDefaultClusterOption}).
					Return(mockEnv, nil)
				m.storeSvc.EXPECT().GetEnvironment(mockApp, mockEnv).Return(&config.Environment{}, nil)
				m.provider.EXPECT().FromRole(gomock.Any(), gomock.Any())
				m.taskSel.EXPECT().RunningTask(taskExecTaskPrompt, taskExecTaskHelpPrompt,
					gomock.Any(), gomock.Any(), gomock.Any()).Return(mockTask, nil)
			},

			wantedTask:       mockTask,
			wantedUseDefault: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			mockConfigSel := mocks.NewMockappEnvSelector(ctrl)
			mockTaskSel := mocks.NewMockrunningTaskSelector(ctrl)
			mockNewTaskSel := func(_ *session.Session) runningTaskSelector {
				return mockTaskSel
			}
			mockProvider := mocks.NewMocksessionProvider(ctrl)
			mocks := execTaskMocks{
				storeSvc:  mockStoreReader,
				configSel: mockConfigSel,
				taskSel:   mockTaskSel,
				provider:  mockProvider,
			}

			tc.setupMocks(mocks)

			execTasks := &taskExecOpts{
				taskExecVars: taskExecVars{
					execVars: execVars{
						name:    tc.inTaskGroup,
						appName: tc.inApp,
						envName: tc.inEnv,
						taskID:  tc.inTaskID,
					},
					useDefault: tc.useDefault,
				},
				store:      mockStoreReader,
				newTaskSel: mockNewTaskSel,
				configSel:  mockConfigSel,
				provider:   mockProvider,
			}

			// WHEN
			err := execTasks.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedTask, execTasks.task)
				require.Equal(t, tc.wantedUseDefault, execTasks.useDefault)
			}
		})
	}
}

func TestTaskExec_Execute(t *testing.T) {
	const (
		mockApp           = "my-app"
		mockEnv           = "my-env"
		mockCommand       = "mockCommand"
		mockBadTaskARN    = "mockBadTaskARN"
		mockTaskARN       = "arn:aws:ecs:us-west-2:123456789:task/4082490ee6c245e09d2145010aa1ba8d"
		mockTaskID        = "4082490ee6c245e09d2145010aa1ba8d"
		mockClusterARN    = "mockClusterARN"
		mockContainerName = "mockContainerName"
	)
	mockTask := &ecs.Task{
		TaskArn:    aws.String(mockTaskARN),
		ClusterArn: aws.String(mockClusterARN),
		Containers: []*awsecs.Container{
			{
				Name: aws.String(mockContainerName),
			},
		},
	}
	mockErr := errors.New("some error")
	testCases := map[string]struct {
		inTask       *ecs.Task
		inUseDefault bool
		setupMocks   func(mocks execTaskMocks)

		wantedError error
	}{
		"should bubble error if fail to get environment": {
			setupMocks: func(m execTaskMocks) {
				m.storeSvc.EXPECT().GetEnvironment(mockApp, mockEnv).Return(nil, mockErr)
			},

			wantedError: fmt.Errorf("get environment my-env: some error"),
		},
		"should bubble error if fail to parse task id": {
			inUseDefault: true,
			inTask: &ecs.Task{
				TaskArn:    aws.String(mockBadTaskARN),
				ClusterArn: aws.String(mockClusterARN),
				Containers: []*awsecs.Container{
					{
						Name: aws.String(mockContainerName),
					},
				},
			},
			setupMocks: func(m execTaskMocks) {
				m.provider.EXPECT().Default()
			},

			wantedError: fmt.Errorf("parse task ARN mockBadTaskARN: parse ECS task ARN: arn: invalid prefix"),
		},
		"should bubble error if fail to execute commands": {
			inTask:       mockTask,
			inUseDefault: true,
			setupMocks: func(m execTaskMocks) {
				m.provider.EXPECT().Default()
				m.commandExec.EXPECT().ExecuteCommand(ecs.ExecuteCommandInput{
					Cluster:   mockClusterARN,
					Command:   mockCommand,
					Container: mockContainerName,
					Task:      mockTaskID,
				}).Return(mockErr)
			},

			wantedError: fmt.Errorf("execute command mockCommand in container mockContainerName: some error"),
		},
		"success": {
			inTask: mockTask,
			setupMocks: func(m execTaskMocks) {
				m.storeSvc.EXPECT().GetEnvironment(mockApp, mockEnv).Return(&config.Environment{}, nil)
				m.provider.EXPECT().FromRole(gomock.Any(), gomock.Any())
				m.commandExec.EXPECT().ExecuteCommand(ecs.ExecuteCommandInput{
					Cluster:   mockClusterARN,
					Command:   mockCommand,
					Container: mockContainerName,
					Task:      mockTaskID,
				}).Return(nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			mockCommandExec := mocks.NewMockecsCommandExecutor(ctrl)
			mockNewCommandExec := func(_ *session.Session) ecsCommandExecutor {
				return mockCommandExec
			}
			mocks := execTaskMocks{
				storeSvc:    mockStoreReader,
				commandExec: mockCommandExec,
				provider:    mocks.NewMocksessionProvider(ctrl),
			}

			tc.setupMocks(mocks)

			execTasks := &taskExecOpts{
				taskExecVars: taskExecVars{
					execVars: execVars{
						appName: mockApp,
						envName: mockEnv,
						command: mockCommand,
					},
					useDefault: tc.inUseDefault,
				},
				task:               tc.inTask,
				store:              mockStoreReader,
				newCommandExecutor: mockNewCommandExec,
				provider:           mocks.provider,
			}

			// WHEN
			err := execTasks.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

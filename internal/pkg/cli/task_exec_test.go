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
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type execTaskMocks struct {
	storeSvc  *mocks.Mockstore
	configSel *mocks.MockappEnvSelector
	taskSel   *mocks.MockrunningTaskSelector
}

func TestTaskExec_Validate(t *testing.T) {
	const (
		mockApp       = "my-app"
		mockEnv       = "my-env"
		mockTaskGroup = "my-task-group"
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
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStoreReader := mocks.NewMockstore(ctrl)
			mocks := execTaskMocks{
				storeSvc: mockStoreReader,
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
				store: mockStoreReader,
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
		mockApp       = "my-app"
		mockEnv       = "my-env"
		mockTaskGroup = "my-task-group"
		mockTaskID    = "my-task-id"
	)
	mockTask := &awsecs.Task{
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

		wantedError error
		wantedTask  *awsecs.Task
	}{
		"should bubble error if fail to select task in default cluster": {
			useDefault: true,
			setupMocks: func(m execTaskMocks) {
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
				m.configSel.EXPECT().Environment(taskExecEnvNamePrompt, taskExecEnvNameHelpPrompt, mockApp, useDefaultClusterOption).
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
				m.taskSel.EXPECT().RunningTask(taskExecTaskPrompt, taskExecTaskHelpPrompt,
					gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, mockErr)
			},

			wantedError: fmt.Errorf("select running task in environment my-env: some error"),
		},
		"success with default flag set": {
			useDefault: true,
			setupMocks: func(m execTaskMocks) {
				m.taskSel.EXPECT().RunningTask(taskExecTaskPrompt, taskExecTaskHelpPrompt,
					gomock.Any(), gomock.Any(), gomock.Any()).Return(mockTask, nil)
			},

			wantedTask: mockTask,
		},
		"success with default option choosed": {
			setupMocks: func(m execTaskMocks) {
				m.configSel.EXPECT().Application(taskExecAppNamePrompt, taskExecAppNameHelpPrompt, useDefaultClusterOption).
					Return(useDefaultClusterOption, nil)
				m.taskSel.EXPECT().RunningTask(taskExecTaskPrompt, taskExecTaskHelpPrompt,
					gomock.Any(), gomock.Any(), gomock.Any()).Return(mockTask, nil)
			},

			wantedTask: mockTask,
		},
		"success with env cluster": {
			setupMocks: func(m execTaskMocks) {
				m.configSel.EXPECT().Application(taskExecAppNamePrompt, taskExecAppNameHelpPrompt, useDefaultClusterOption).
					Return(mockApp, nil)
				m.configSel.EXPECT().Environment(taskExecEnvNamePrompt, taskExecEnvNameHelpPrompt, mockApp, useDefaultClusterOption).
					Return(mockEnv, nil)
				m.storeSvc.EXPECT().GetEnvironment(mockApp, mockEnv).Return(&config.Environment{}, nil)
				m.taskSel.EXPECT().RunningTask(taskExecTaskPrompt, taskExecTaskHelpPrompt,
					gomock.Any(), gomock.Any(), gomock.Any()).Return(mockTask, nil)
			},

			wantedTask: mockTask,
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
			mocks := execTaskMocks{
				storeSvc:  mockStoreReader,
				configSel: mockConfigSel,
				taskSel:   mockTaskSel,
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
			}

			// WHEN
			err := execTasks.Ask()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantedTask, execTasks.task)
			}
		})
	}
}

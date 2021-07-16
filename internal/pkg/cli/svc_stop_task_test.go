// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"strconv"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestStopTaskSvcOpts_Validate(t *testing.T) {
	tests := map[string]struct {
		appName    string
		envName    string
		name       string
		all        bool
		taskIDs    []string
		setupMocks func(m *mocks.Mockstore)

		want error
	}{
		"with no flag set": {
			appName:    "phonetool",
			setupMocks: func(m *mocks.Mockstore) {},
			want:       errors.New(`any one of the following arguments are required "--all" or  "--tasks"`),
		},
		"with all flags set": {
			appName: "phonetool",
			envName: "test",
			name:    "api",
			all:     true,
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(&config.Environment{Name: "test"}, nil)
				m.EXPECT().GetService("phonetool", "api").Times(1).Return(&config.Workload{
					Name: "api",
				}, nil)
			},
			want: nil,
		},
		"with taskIDs set": {
			appName: "phonetool",
			envName: "test",
			name:    "api",
			taskIDs: []string{"taskId"},
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(&config.Environment{Name: "test"}, nil)
				m.EXPECT().GetService("phonetool", "api").Times(1).Return(&config.Workload{
					Name: "api",
				}, nil)
			},
			want: nil,
		},
		"with no name set": {
			appName: "phonetool",
			name:    "api",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetService("phonetool", "api").Times(1).Return(nil,
					errors.New("some error"))
			},
			want: errors.New("some error"),
		},
		"with no env set": {
			appName: "phonetool",
			envName: "test",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(
					nil, errors.New("unknown env"))
			},
			want: errors.New("get environment test from config store: unknown env"),
		},
		"with both all and taskIds": {
			appName: "phonetool",
			envName: "test",
			name:    "api",
			all:     true,
			taskIDs: []string{"taskId"},
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(&config.Environment{Name: "test"}, nil)
				m.EXPECT().GetService("phonetool", "api").Times(1).Return(&config.Workload{
					Name: "api",
				}, nil)
			},
			want: errors.New(`only one of "-all" or "--tasks" may be used`),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockstore := mocks.NewMockstore(ctrl)

			test.setupMocks(mockstore)

			opts := svcStopTaskOpts{
				wkldStopTaskVars: wkldStopTaskVars{
					all:     test.all,
					name:    test.name,
					envName: test.envName,
					appName: test.appName,
					taskIDs: test.taskIDs,
				},
				store: mockstore,
			}

			err := opts.Validate()

			if test.want != nil {
				require.EqualError(t, err, test.want.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStopTaskSvcOpts_Ask(t *testing.T) {
	const (
		testAppName = "phonetool"
		testSvcName = "api"
	)
	mockError := errors.New("mockError")

	tests := map[string]struct {
		appName string
		envName string
		name    string
		all     bool
		taskIDs []string

		mockSel    func(m *mocks.MockdeploySelector)
		mockPrompt func(m *mocks.Mockprompter)

		wantedName  string
		wantedError error
	}{
		"should ask for app name": {
			appName: "",
			name:    testSvcName,
			mockSel: func(m *mocks.MockdeploySelector) {
				m.EXPECT().Application(svcAppNamePrompt, svcAppNameHelpPrompt).Return(testAppName, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedName: testSvcName,
		},
		"should ask for service name": {
			appName: testAppName,
			name:    "",
			mockSel: func(m *mocks.MockdeploySelector) {
				m.EXPECT().DeployedService(svcStopTaskNamePrompt, "", testAppName, gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env: "mockEnv",
						Svc: testSvcName,
					}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedName: testSvcName,
		},
		"returns error if no services found": {
			appName: testAppName,
			name:    "",
			mockSel: func(m *mocks.MockdeploySelector) {
				m.EXPECT().DeployedService(svcStopTaskNamePrompt, "", testAppName, gomock.Any(), gomock.Any()).
					Return(nil, mockError)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("select service: %w", mockError),
		},
		"returns error if fail to select service": {
			appName: testAppName,
			name:    "",
			mockSel: func(m *mocks.MockdeploySelector) {
				m.EXPECT().DeployedService(svcStopTaskNamePrompt, "", testAppName, gomock.Any(), gomock.Any()).
					Return(nil, mockError)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
			},

			wantedError: fmt.Errorf("select service: %w", mockError),
		},
		"should wrap error returned from prompter confirmation": {
			appName: testAppName,
			name:    testSvcName,
			all:     true,
			mockSel: func(m *mocks.MockdeploySelector) {

			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(fmtSvcStopTasksConfirmPrompt, testSvcName, testAppName),
					svcStopTasksConfirmHelp,
				).Times(1).Return(true, mockError)
			},

			wantedError: fmt.Errorf("svc stop task confirmation prompt: %w", mockError),
		},
		"successful confirmation": {
			appName: testAppName,
			name:    "",
			all:     true,
			mockSel: func(m *mocks.MockdeploySelector) {
				m.EXPECT().DeployedService(svcStopTaskNamePrompt, "", testAppName, gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env: "mockEnv",
						Svc: "",
					}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(fmtSvcStopTasksConfirmPrompt, "", testAppName),
					svcStopTasksConfirmHelp,
				).Times(1).Return(true, nil)
			},

			wantedError: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompter := mocks.NewMockprompter(ctrl)
			mockSel := mocks.NewMockdeploySelector(ctrl)
			test.mockPrompt(mockPrompter)
			test.mockSel(mockSel)

			opts := svcStopTaskOpts{
				wkldStopTaskVars: wkldStopTaskVars{
					all:     test.all,
					name:    test.name,
					envName: test.envName,
					appName: test.appName,
					taskIDs: test.taskIDs,
				},
				prompt: mockPrompter,
				sel:    mockSel,
			}

			got := opts.AskInput()

			if got != nil {
				require.Equal(t, test.wantedError, got)
			} else {
				require.Equal(t, test.wantedName, opts.name)
			}
		})
	}
}

type stopJobMocks struct {
	store        *mocks.Mockstore
	sessProvider *sessions.Provider
	spinner      *mocks.Mockprogress
	ecs          *mocks.MocktaskStopper
}

func TestStopTaskSvcOpts_Execute(t *testing.T) {
	mockSvcName := "backend"
	mockEnvName := "test"
	mockAppName := "badgoose"

	taskIDs := []string{"taskIDs"}

	tests := map[string]struct {
		appName string
		envName string
		name    string
		all     bool
		taskIDs []string

		setupMocks func(mocks stopJobMocks)

		wantedError error
	}{
		"happy path with all flag passed in": {
			appName: mockAppName,
			envName: mockEnvName,
			name:    mockSvcName,
			all:     true,
			setupMocks: func(mocks stopJobMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetEnvironment(mockAppName, mockEnvName).Return(&config.Environment{Name: mockEnvName}, nil),
					mocks.spinner.EXPECT().Start(fmt.Sprintf("Stopping all running tasks in %s.", color.HighlightUserInput(mockSvcName))),
					mocks.ecs.EXPECT().StopWorkloadTasks(mockAppName, mockEnvName, mockSvcName, taskStopUserInitiated).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf("Stopped all running tasks in %s\n", color.HighlightUserInput(mockSvcName))),
				)
			},
			wantedError: nil,
		},
		"happy path with taskIds passed in": {
			appName: mockAppName,
			envName: mockEnvName,
			name:    mockSvcName,
			taskIDs: taskIDs,
			setupMocks: func(mocks stopJobMocks) {
				tasksLen := strconv.Itoa(len(taskIDs))
				gomock.InOrder(
					mocks.store.EXPECT().GetEnvironment(mockAppName, mockEnvName).Return(&config.Environment{Name: mockEnvName}, nil),
					mocks.spinner.EXPECT().Start(fmt.Sprintf("Stopping %s task(s)", tasksLen)),
					mocks.ecs.EXPECT().StopTasksWithTaskIds(mockAppName, mockEnvName, taskIDs, taskStopUserInitiated).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessln("Task(s) are stopped successfully")),
				)
			},
			wantedError: nil,
		},
		"error with all flag passed in": {
			appName: mockAppName,
			envName: mockEnvName,
			name:    mockSvcName,
			all:     true,
			setupMocks: func(mocks stopJobMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetEnvironment(mockAppName, mockEnvName).Return(&config.Environment{Name: mockEnvName}, nil),
					mocks.spinner.EXPECT().Start(fmt.Sprintf("Stopping all running tasks in %s.", color.HighlightUserInput(mockSvcName))),
					mocks.ecs.EXPECT().StopWorkloadTasks(mockAppName, mockEnvName, mockSvcName, taskStopUserInitiated).Return(mockError),
					mocks.spinner.EXPECT().Stop(log.Serrorf("Error stopping running tasks in %s.\n", mockSvcName)),
				)
			},
			wantedError: fmt.Errorf("stop running tasks in family %s: %w", mockSvcName, mockError),
		},
		"error with taskIds passed in": {
			appName: mockAppName,
			envName: mockEnvName,
			name:    mockSvcName,
			taskIDs: taskIDs,
			setupMocks: func(mocks stopJobMocks) {
				tasksLen := strconv.Itoa(len(taskIDs))
				gomock.InOrder(
					mocks.store.EXPECT().GetEnvironment(mockAppName, mockEnvName).Return(&config.Environment{Name: mockEnvName}, nil),
					mocks.spinner.EXPECT().Start(fmt.Sprintf("Stopping %s task(s)", tasksLen)),
					mocks.ecs.EXPECT().StopTasksWithTaskIds(mockAppName, mockEnvName, taskIDs, taskStopUserInitiated).Return(mockError),
					mocks.spinner.EXPECT().Stop(log.Serrorf("Error stopping running tasks in %s.\n", mockSvcName)),
				)
			},
			wantedError: fmt.Errorf("stop running tasks by ids %w", mockError),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			mockstore := mocks.NewMockstore(ctrl)
			mockSession := sessions.NewProvider()
			mockSpinner := mocks.NewMockprogress(ctrl)
			mockTaskStopper := mocks.NewMocktaskStopper(ctrl)
			mockPrompter := mocks.NewMockprompter(ctrl)
			mockSel := mocks.NewMockdeploySelector(ctrl)
			mockGetECS := func(_ *session.Session) taskStopper {
				return mockTaskStopper
			}

			mocks := stopJobMocks{
				store:        mockstore,
				sessProvider: mockSession,
				spinner:      mockSpinner,
				ecs:          mockTaskStopper,
			}

			test.setupMocks(mocks)

			opts := svcStopTaskOpts{
				wkldStopTaskVars: wkldStopTaskVars{
					all:     test.all,
					name:    test.name,
					envName: test.envName,
					appName: test.appName,
					taskIDs: test.taskIDs,
				},
				prompt:         mockPrompter,
				sel:            mockSel,
				store:          mockstore,
				sess:           mockSession,
				spinner:        mockSpinner,
				newTaskStopper: mockGetECS,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if test.wantedError != nil {
				require.EqualError(t, err, test.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

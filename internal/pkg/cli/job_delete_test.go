// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeleteJobOpts_Validate(t *testing.T) {
	mockError := errors.New("some error")

	tests := map[string]struct {
		inAppName  string
		inEnvName  string
		inName     string
		setupMocks func(m *mocks.Mockstore)

		want error
	}{
		"with no flag set": {
			inAppName:  "phonetool",
			setupMocks: func(m *mocks.Mockstore) {},
			want:       nil,
		},
		"with all flags set": {
			inAppName: "phonetool",
			inEnvName: "test",
			inName:    "resizer",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(&config.Environment{Name: "test"}, nil)
				m.EXPECT().GetJob("phonetool", "resizer").Times(1).Return(&config.Workload{
					Name: "resizer",
				}, nil)
			},
			want: nil,
		},
		"with env flag set": {
			inAppName: "phonetool",
			inEnvName: "test",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(&config.Environment{Name: "test"}, nil)
			},
			want: nil,
		},
		"with job flag set": {
			inAppName: "phonetool",
			inName:    "resizer",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetJob("phonetool", "resizer").Times(1).Return(&config.Workload{
					Name: "resizer",
				}, nil)
			},
			want: nil,
		},
		"with unknown environment": {
			inAppName: "phonetool",
			inEnvName: "test",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(nil, errors.New("unknown env"))
			},
			want: errors.New("get environment test from config store: unknown env"),
		},
		"should return error if fail to get job name": {
			inAppName: "phonetool",
			inName:    "resizer",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetJob("phonetool", "resizer").Times(1).Return(nil, mockError)
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

			opts := deleteJobOpts{
				deleteJobVars: deleteJobVars{
					appName: test.inAppName,
					name:    test.inName,
					envName: test.inEnvName,
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

func TestDeleteJobOpts_Ask(t *testing.T) {
	const (
		testAppName = "phonetool"
		testJobName = "resizer"
	)
	mockError := errors.New("mockError")

	tests := map[string]struct {
		skipConfirmation bool
		inName           string
		envName          string
		appName          string

		mockPrompt func(m *mocks.Mockprompter)
		mockSel    func(m *mocks.MockconfigSelector)

		wantedName  string
		wantedError error
	}{
		"should ask for app name": {
			appName:          "",
			inName:           testJobName,
			skipConfirmation: true,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application("Which application's job would you like to delete?", "").Return(testAppName, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedName: testJobName,
		},
		"should ask for job name": {
			appName:          testAppName,
			inName:           "",
			skipConfirmation: true,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Job("Which job would you like to delete?", "", testAppName).Return(testJobName, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedName: testJobName,
		},
		"returns error if no jobs found": {
			appName:          testAppName,
			inName:           "",
			skipConfirmation: true,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Job("Which job would you like to delete?", "", testAppName).Return("", mockError)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("select job: %w", mockError),
		},
		"returns error if fail to select job": {
			appName:          testAppName,
			inName:           "",
			skipConfirmation: true,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Job("Which job would you like to delete?", "", testAppName).Return("", mockError)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("select job: %w", mockError),
		},
		"should skip confirmation": {
			appName:          testAppName,
			inName:           testJobName,
			skipConfirmation: true,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Job(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedName: testJobName,
		},
		"should wrap error returned from prompter confirmation": {
			appName:          testAppName,
			inName:           testJobName,
			skipConfirmation: false,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Job(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(fmtJobDeleteConfirmPrompt, testJobName, testAppName),
					jobDeleteConfirmHelp,
					gomock.Any(),
				).Times(1).Return(true, mockError)
			},

			wantedError: fmt.Errorf("job delete confirmation prompt: %w", mockError),
		},
		"should return error if user does not confirm job delete": {
			appName:          testAppName,
			inName:           testJobName,
			skipConfirmation: false,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Job(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(fmtJobDeleteConfirmPrompt, testJobName, testAppName),
					jobDeleteConfirmHelp,
					gomock.Any(),
				).Times(1).Return(false, nil)
			},

			wantedError: errJobDeleteCancelled,
		},
		"should return error nil if user confirms job delete": {
			appName:          testAppName,
			inName:           testJobName,
			skipConfirmation: false,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Job(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(fmtJobDeleteConfirmPrompt, testJobName, testAppName),
					jobDeleteConfirmHelp,
					gomock.Any(),
				).Times(1).Return(true, nil)
			},

			wantedName: testJobName,
		},
		"should return error nil if user confirms job delete --env": {
			appName:          testAppName,
			inName:           testJobName,
			envName:          "test",
			skipConfirmation: false,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Job(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(fmtJobDeleteFromEnvConfirmPrompt, testJobName, "test"),
					fmt.Sprintf(fmtJobDeleteFromEnvConfirmHelp, "test"),
					gomock.Any(),
				).Times(1).Return(true, nil)
			},

			wantedName: testJobName,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompter := mocks.NewMockprompter(ctrl)
			mockSel := mocks.NewMockconfigSelector(ctrl)
			test.mockPrompt(mockPrompter)
			test.mockSel(mockSel)

			opts := deleteJobOpts{
				deleteJobVars: deleteJobVars{
					skipConfirmation: test.skipConfirmation,
					appName:          test.appName,
					name:             test.inName,
					envName:          test.envName,
				},
				prompt: mockPrompter,
				sel:    mockSel,
			}

			got := opts.Ask()

			if got != nil {
				require.Equal(t, test.wantedError, got)
			} else {
				require.Equal(t, test.wantedName, opts.name)
			}
		})
	}
}

type deleteJobMocks struct {
	store          *mocks.Mockstore
	secretsmanager *mocks.MocksecretsManager
	sessProvider   *mocks.MocksessionProvider
	appCFN         *mocks.MockjobRemoverFromApp
	spinner        *mocks.Mockprogress
	jobCFN         *mocks.MockwlDeleter
	ecr            *mocks.MockimageRemover
	ecs            *mocks.MocktaskStopper
}

func TestDeleteJobOpts_Execute(t *testing.T) {
	mockJobName := "resizer"
	mockEnvName := "test"
	mockAppName := "badgoose"
	mockEnv := &config.Environment{
		App:            mockAppName,
		Name:           mockEnvName,
		ManagerRoleARN: "some-arn",
		Region:         "us-west-2",
	}
	mockEnvs := []*config.Environment{mockEnv}
	mockApp := &config.Application{
		Name: mockAppName,
	}

	mockRepo := fmt.Sprintf("%s/%s", mockAppName, mockJobName)
	testError := errors.New("some error")

	tests := map[string]struct {
		inAppName string
		inEnvName string
		inJobName string

		setupMocks func(mocks deleteJobMocks)

		wantedError error
	}{
		"happy path with no environment passed in as flag": {
			inAppName: mockAppName,
			inJobName: mockJobName,
			setupMocks: func(mocks deleteJobMocks) {
				gomock.InOrder(
					// appEnvironments
					mocks.store.EXPECT().ListEnvironments(gomock.Eq(mockAppName)).Times(1).Return(mockEnvs, nil),

					mocks.sessProvider.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{}, nil),
					// deleteStacks
					mocks.jobCFN.EXPECT().DeleteWorkload(gomock.Any()).Return(nil),
					// delete orphan tasks
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtJobTasksStopStart, mockJobName, mockEnvName)),
					mocks.ecs.EXPECT().StopWorkloadTasks(mockAppName, mockEnvName, mockJobName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtJobTasksStopComplete, mockJobName, mockEnvName)),

					mocks.sessProvider.EXPECT().DefaultWithRegion(gomock.Any()).Return(&session.Session{}, nil),
					// emptyECRRepos
					mocks.ecr.EXPECT().ClearRepository(mockRepo).Return(nil),
					// removeJobFromApp
					mocks.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil),
					mocks.appCFN.EXPECT().RemoveJobFromApp(mockApp, mockJobName).Return(nil),

					// deleteSSMParam
					mocks.store.EXPECT().DeleteJob(mockAppName, mockJobName).Return(nil),
				)
			},
			wantedError: nil,
		},
		// A job can be deployed to multiple
		// environments - and deleting it in one
		// should not delete it form the entire app.
		"happy path with environment passed in as flag": {
			inAppName: mockAppName,
			inJobName: mockJobName,
			inEnvName: mockEnvName,
			setupMocks: func(mocks deleteJobMocks) {
				gomock.InOrder(
					// appEnvironments
					mocks.store.EXPECT().GetEnvironment(mockAppName, mockEnvName).Times(1).Return(mockEnv, nil),
					mocks.sessProvider.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{}, nil),
					// deleteStacks
					mocks.jobCFN.EXPECT().DeleteWorkload(gomock.Any()).Return(nil),
					// delete orphan tasks
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtJobTasksStopStart, mockJobName, mockEnvName)),
					mocks.ecs.EXPECT().StopWorkloadTasks(mockAppName, mockEnvName, mockJobName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtJobTasksStopComplete, mockJobName, mockEnvName)),

					// It should **not** emptyECRRepos
					mocks.ecr.EXPECT().ClearRepository(gomock.Any()).Return(nil).Times(0),

					// It should **not** removeJobFromApp
					mocks.appCFN.EXPECT().RemoveJobFromApp(gomock.Any(), gomock.Any()).Return(nil).Times(0),

					// It should **not** deleteSSMParam
					mocks.store.EXPECT().DeleteJob(gomock.Any(), gomock.Any()).Return(nil).Times(0),
				)
			},
			wantedError: nil,
		},
		"errors when deleting stack": {
			inAppName: mockAppName,
			inJobName: mockJobName,
			inEnvName: mockEnvName,
			setupMocks: func(mocks deleteJobMocks) {
				gomock.InOrder(
					// appEnvironments
					mocks.store.EXPECT().GetEnvironment(mockAppName, mockEnvName).Times(1).Return(mockEnv, nil),
					mocks.sessProvider.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{
						Config: &aws.Config{
							Region: aws.String("mockRegion"),
						},
					}, nil),
					// deleteStacks
					mocks.jobCFN.EXPECT().DeleteWorkload(gomock.Any()).Return(testError),
				)
			},
			wantedError: fmt.Errorf("delete job stack: %w", testError),
		},
		"errors when deleting orphan tasks: failed to stop tasks": {
			inAppName: mockAppName,
			inJobName: mockJobName,
			inEnvName: mockEnvName,
			setupMocks: func(mocks deleteJobMocks) {
				gomock.InOrder(
					// appEnvironments
					mocks.store.EXPECT().GetEnvironment(mockAppName, mockEnvName).Times(1).Return(mockEnv, nil),
					mocks.sessProvider.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{
						Config: &aws.Config{
							Region: aws.String("mockRegion"),
						},
					}, nil),
					// deleteStacks
					mocks.jobCFN.EXPECT().DeleteWorkload(gomock.Any()).Return(nil),
					// delete orphan tasks
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtJobTasksStopStart, mockJobName, mockEnvName)),
					mocks.ecs.EXPECT().StopWorkloadTasks(mockAppName, mockEnvName, mockJobName).Return(testError),
					mocks.spinner.EXPECT().Stop(log.Serrorf(fmtJobTasksStopFailed, mockJobName, mockEnvName, fmt.Errorf("some error"))),
				)
			},
			wantedError: fmt.Errorf("stop tasks for environment test: some error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			mockstore := mocks.NewMockstore(ctrl)
			mockSecretsManager := mocks.NewMocksecretsManager(ctrl)
			mockSession := mocks.NewMocksessionProvider(ctrl)
			mockAppCFN := mocks.NewMockjobRemoverFromApp(ctrl)
			mockJobCFN := mocks.NewMockwlDeleter(ctrl)
			mockSpinner := mocks.NewMockprogress(ctrl)
			mockImageRemover := mocks.NewMockimageRemover(ctrl)
			mockTaskStopper := mocks.NewMocktaskStopper(ctrl)
			mockGetJobCFN := func(_ *session.Session) wlDeleter {
				return mockJobCFN
			}
			mockGetImageRemover := func(_ *session.Session) imageRemover {
				return mockImageRemover
			}
			mockNewTaskStopper := func(_ *session.Session) taskStopper {
				return mockTaskStopper
			}

			mocks := deleteJobMocks{
				store:          mockstore,
				secretsmanager: mockSecretsManager,
				sessProvider:   mockSession,
				appCFN:         mockAppCFN,
				spinner:        mockSpinner,
				jobCFN:         mockJobCFN,
				ecr:            mockImageRemover,
				ecs:            mockTaskStopper,
			}

			test.setupMocks(mocks)

			opts := deleteJobOpts{
				deleteJobVars: deleteJobVars{
					appName: test.inAppName,
					name:    test.inJobName,
					envName: test.inEnvName,
				},
				store:           mockstore,
				sess:            mockSession,
				spinner:         mockSpinner,
				appCFN:          mockAppCFN,
				newWlDeleter:    mockGetJobCFN,
				newImageRemover: mockGetImageRemover,
				newTaskStopper:  mockNewTaskStopper,
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

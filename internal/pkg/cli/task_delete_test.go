// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/aws/session"

	awscfn "github.com/aws/copilot-cli/internal/pkg/aws/cloudformation"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type validateMocks struct {
	store    *mocks.Mockstore
	cfn      *mocks.MocktaskStackManager
	provider *mocks.MocksessionProvider
}

func TestDeleteTaskOpts_Validate(t *testing.T) {

	testCases := map[string]struct {
		inAppName        string
		inEnvName        string
		inName           string
		inDefaultCluster bool
		setupMocks       func(m validateMocks)

		want error
	}{
		"with only app flag": {
			inAppName: "phonetool",
			setupMocks: func(m validateMocks) {
				m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
			},
			want: nil,
		},
		"with no flags": {
			setupMocks: func(m validateMocks) {},
			want:       nil,
		},
		"with app/env flags set": {
			inAppName: "phonetool",
			inEnvName: "test",
			setupMocks: func(m validateMocks) {
				m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.store.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{Name: "test", App: "phonetool"}, nil)
			},
			want: nil,
		},
		"with all flags": {
			inAppName: "phonetool",
			inEnvName: "test",
			inName:    "oneoff",
			setupMocks: func(m validateMocks) {
				m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.store.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{Name: "test", App: "phonetool"}, nil)
				m.store.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{Name: "test", App: "phonetool"}, nil)
				m.provider.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{}, nil)
				m.cfn.EXPECT().GetTaskStack("oneoff")
			},
			want: nil,
		},
		"task does not exist": {
			inAppName: "phonetool",
			inEnvName: "test",
			inName:    "oneoff",
			want:      errors.New("get task: some error"),
			setupMocks: func(m validateMocks) {
				m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil)
				m.store.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{Name: "test", App: "phonetool"}, nil)
				m.store.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{Name: "test", App: "phonetool"}, nil)
				m.provider.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{}, nil)
				m.cfn.EXPECT().GetTaskStack("oneoff").Return(nil, errors.New("some error"))
			},
		},
		"with default cluster flag set": {
			inDefaultCluster: true,
			inName:           "oneoff",
			setupMocks: func(m validateMocks) {
				m.provider.EXPECT().Default().Return(&session.Session{}, nil)
				m.cfn.EXPECT().GetTaskStack("oneoff")
			},
			want: nil,
		},
		"with default cluster and env flag": {
			inDefaultCluster: true,
			inEnvName:        "test",
			inAppName:        "phonetool",
			setupMocks:       func(m validateMocks) {},
			want:             errors.New("cannot specify both `--app` and `--default`"),
		},
		"with error getting app": {
			inAppName: "phonetool",
			inEnvName: "test",
			setupMocks: func(m validateMocks) {
				m.store.EXPECT().GetApplication("phonetool").Return(nil, errors.New("some error"))
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
			mocktaskStackManager := mocks.NewMocktaskStackManager(ctrl)

			mocks := validateMocks{
				store:    mockstore,
				cfn:      mocktaskStackManager,
				provider: mocks.NewMocksessionProvider(ctrl),
			}

			tc.setupMocks(mocks)

			opts := deleteTaskOpts{
				deleteTaskVars: deleteTaskVars{
					skipConfirmation: false,
					app:              tc.inAppName,
					env:              tc.inEnvName,
					name:             tc.inName,
					defaultCluster:   tc.inDefaultCluster,
				},
				store: mockstore,
				newStackManager: func(_ *session.Session) taskStackManager {
					return mocktaskStackManager
				},
				provider: mocks.provider,
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
				m.EXPECT().Task(taskDeleteNamePrompt, "", gomock.Any()).Return("abc", nil)
			},
			mockSess: func(m *mocks.MocksessionProvider) {
				m.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{}, nil)
			},
			mockPrompter: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm("Are you sure you want to delete abc from application phonetool and environment test?", gomock.Any(), gomock.Any()).Return(true, nil)
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
				m.EXPECT().Task(taskDeleteNamePrompt, "", gomock.Any()).Return("abc", nil)
			},
			mockSess: func(m *mocks.MocksessionProvider) {
				m.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{}, nil)
			},
			mockPrompter: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm("Are you sure you want to delete abc from application phonetool and environment test?", gomock.Any(), gomock.Any()).Return(false, nil)
			},
			wantErr: "task delete cancelled - no changes made",
		},
		"default flag specified": {
			inDefaultCluster: true,

			mockStore: func(m *mocks.Mockstore) {
			},
			mockSel: func(m *mocks.MockwsSelector) {},
			mockTaskSelect: func(m *mocks.MockcfTaskSelector) {
				m.EXPECT().Task(taskDeleteNamePrompt, "", gomock.Any()).Return("abc", nil)
			},
			mockSess: func(m *mocks.MocksessionProvider) {
				m.EXPECT().Default().Return(&session.Session{}, nil)
			},
			mockPrompter: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm("Are you sure you want to delete abc from the default cluster?", gomock.Any(), gomock.Any()).Return(true, nil)
			},
		},
		"no flags specified": {
			mockStore: func(m *mocks.Mockstore) {
				// This call is in GetSession when an environment is specified and we need to get the Manager Role's session.
				m.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{Name: "test", App: "phonetool"}, nil)
			},
			mockSel: func(m *mocks.MockwsSelector) {
				m.EXPECT().Application(taskDeleteAppPrompt, "", appEnvOptionNone).Return("phonetool", nil)
				m.EXPECT().Environment(taskDeleteEnvPrompt, "", "phonetool", prompt.Option{Value: appEnvOptionNone}).Return("test", nil)
			},
			mockTaskSelect: func(m *mocks.MockcfTaskSelector) {
				m.EXPECT().Task(taskDeleteNamePrompt, "", gomock.Any()).Return("abc", nil)
			},
			mockSess: func(m *mocks.MocksessionProvider) {
				m.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{}, nil)
			},
			mockPrompter: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm("Are you sure you want to delete abc from application phonetool and environment test?", gomock.Any(), gomock.Any()).Return(true, nil)
			},
		},
		"no flags specified (default path)": {
			mockStore: func(m *mocks.Mockstore) {},
			mockSel: func(m *mocks.MockwsSelector) {
				m.EXPECT().Application(taskDeleteAppPrompt, "", appEnvOptionNone).Return(appEnvOptionNone, nil)
			},
			mockTaskSelect: func(m *mocks.MockcfTaskSelector) {
				m.EXPECT().Task(taskDeleteNamePrompt, "", gomock.Any()).Return("abc", nil)
			},
			mockSess: func(m *mocks.MocksessionProvider) {
				m.EXPECT().Default().Return(&session.Session{}, nil)
			},
			mockPrompter: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm("Are you sure you want to delete abc from the default cluster?", gomock.Any(), gomock.Any()).Return(true, nil)
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

				store:    mockStore,
				sel:      mockSel,
				provider: mockSess,
				prompt:   mockPrompt,

				newTaskSel: func(sess *session.Session) cfTaskSelector { return mockTaskSel },
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

type deleteTaskMocks struct {
	store   *mocks.Mockstore
	sess    *mocks.MocksessionProvider
	ecr     *mocks.MockimageRemover
	s3      *mocks.MockbucketEmptier
	ecs     *mocks.MocktaskStopper
	cfn     *mocks.MocktaskStackManager
	spinner *mocks.Mockprogress
}

func TestDeleteTaskOpts_Execute(t *testing.T) {
	mockApp := "phonetool"
	mockEnvName := "pdx"
	mockTaskName := "hide-snacks"
	mockTaskStackName := "task-hide-snacks"
	mockTaskRepoName := "copilot-hide-snacks"
	mockManagerARN := "arn:aws:iam:us-west-2:123456789:role/abc"

	mockEnv := &config.Environment{
		Name:           mockEnvName,
		Region:         "us-west-2",
		ManagerRoleARN: mockManagerARN,
	}

	mockAppEnvTask := &deploy.TaskStackInfo{
		App:     mockApp,
		Env:     mockEnvName,
		RoleARN: mockManagerARN,

		StackName:  mockTaskStackName,
		BucketName: "arn:aws:s3:::bucket",
	}
	mockDefaultTask := deploy.TaskStackInfo{
		StackName:  mockTaskStackName,
		BucketName: "arn:aws:s3:::bucket",
	}
	mockDefaultTaskNoBucket := deploy.TaskStackInfo{
		StackName: mockTaskStackName,
	}
	mockError := errors.New("some error")

	testCases := map[string]struct {
		inDefault bool
		inApp     string
		inEnv     string
		inName    string

		setupMocks func(mocks deleteTaskMocks)

		wantedErr error
	}{
		"success with app/env": {
			inApp:  mockApp,
			inEnv:  mockEnvName,
			inName: mockTaskName,

			setupMocks: func(m deleteTaskMocks) {
				m.sess.EXPECT().FromRole(mockEnv.ManagerRoleARN, mockEnv.Region).Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("mockRegion"),
					},
				}, nil)
				m.sess.EXPECT().DefaultWithRegion("mockRegion").Return(&session.Session{}, nil)
				gomock.InOrder(
					m.store.EXPECT().GetEnvironment(mockApp, mockEnvName).Return(mockEnv, nil),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecs.EXPECT().StopOneOffTasks(mockApp, mockEnvName, mockTaskName).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecr.EXPECT().ClearRepository(mockTaskRepoName).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.cfn.EXPECT().GetTaskStack(mockTaskName).Return(mockAppEnvTask, nil),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.s3.EXPECT().EmptyBucket(gomock.Any()).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.cfn.EXPECT().DeleteTask(*mockAppEnvTask).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
				)
			},
		},
		"success with default cluster": {
			inDefault: true,
			inName:    mockTaskName,

			setupMocks: func(m deleteTaskMocks) {
				gomock.InOrder(
					m.sess.EXPECT().Default().Return(&session.Session{}, nil),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecs.EXPECT().StopDefaultClusterTasks(mockTaskName).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecr.EXPECT().ClearRepository(mockTaskRepoName).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.cfn.EXPECT().GetTaskStack(mockTaskName).Return(&mockDefaultTask, nil),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.s3.EXPECT().EmptyBucket(gomock.Any()).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.cfn.EXPECT().DeleteTask(mockDefaultTask).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
				)
			},
		},
		"success with default cluster and no S3 bucket": {
			inDefault: true,
			inName:    mockTaskName,

			setupMocks: func(m deleteTaskMocks) {
				gomock.InOrder(
					m.sess.EXPECT().Default().Return(&session.Session{}, nil),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecs.EXPECT().StopDefaultClusterTasks(mockTaskName).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecr.EXPECT().ClearRepository(mockTaskRepoName).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.cfn.EXPECT().GetTaskStack(mockTaskName).Return(&mockDefaultTaskNoBucket, nil),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.cfn.EXPECT().DeleteTask(mockDefaultTaskNoBucket).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
				)
			},
		},
		"error when getting environment": {
			inApp:  mockApp,
			inEnv:  mockEnvName,
			inName: mockTaskName,

			wantedErr: errors.New("get session: some error"),

			setupMocks: func(m deleteTaskMocks) {
				gomock.InOrder(
					m.store.EXPECT().GetEnvironment(mockApp, mockEnvName).Return(nil, mockError),
				)
			},
		},
		"error deleting task stack": {
			inApp:  mockApp,
			inEnv:  mockEnvName,
			inName: mockTaskName,

			wantedErr: errors.New("delete stack for task hide-snacks: some error"),

			setupMocks: func(m deleteTaskMocks) {
				m.sess.EXPECT().FromRole(mockEnv.ManagerRoleARN, mockEnv.Region).Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("mockRegion"),
					},
				}, nil)
				m.sess.EXPECT().DefaultWithRegion("mockRegion").Return(&session.Session{}, nil)
				gomock.InOrder(
					m.store.EXPECT().GetEnvironment(mockApp, mockEnvName).Return(mockEnv, nil),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecs.EXPECT().StopOneOffTasks(mockApp, mockEnvName, mockTaskName).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecr.EXPECT().ClearRepository(mockTaskRepoName).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.cfn.EXPECT().GetTaskStack(mockTaskName).Return(mockAppEnvTask, nil),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.s3.EXPECT().EmptyBucket(gomock.Any()).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.cfn.EXPECT().DeleteTask(*mockAppEnvTask).Return(mockError),
					m.spinner.EXPECT().Stop(gomock.Any()),
				)
			},
		},
		"task stack does not exist (idempotency check)": {
			inApp:  mockApp,
			inEnv:  mockEnvName,
			inName: mockTaskName,

			setupMocks: func(m deleteTaskMocks) {
				mockErrStackNotFound := awscfn.ErrStackNotFound{}
				m.sess.EXPECT().FromRole(mockEnv.ManagerRoleARN, mockEnv.Region).Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("mockRegion"),
					},
				}, nil)
				m.sess.EXPECT().DefaultWithRegion("mockRegion").Return(&session.Session{}, nil)
				gomock.InOrder(
					m.store.EXPECT().GetEnvironment(mockApp, mockEnvName).Return(mockEnv, nil),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecs.EXPECT().StopOneOffTasks(mockApp, mockEnvName, mockTaskName).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecr.EXPECT().ClearRepository(mockTaskRepoName).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.cfn.EXPECT().GetTaskStack(mockTaskName).Return(nil, &mockErrStackNotFound),
				)
			},
		},
		"error clearing ecr repo": {
			inApp:  mockApp,
			inEnv:  mockEnvName,
			inName: mockTaskName,

			wantedErr: errors.New("empty ECR repository for task hide-snacks: some error"),

			setupMocks: func(m deleteTaskMocks) {
				m.sess.EXPECT().FromRole(mockEnv.ManagerRoleARN, mockEnv.Region).Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("mockRegion"),
					},
				}, nil)
				m.sess.EXPECT().DefaultWithRegion("mockRegion").Return(&session.Session{}, nil)
				gomock.InOrder(
					m.store.EXPECT().GetEnvironment(mockApp, mockEnvName).Return(mockEnv, nil),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecs.EXPECT().StopOneOffTasks(mockApp, mockEnvName, mockTaskName).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecr.EXPECT().ClearRepository(mockTaskRepoName).Return(mockError),
					m.spinner.EXPECT().Stop(gomock.Any()),
				)
			},
		},
		"error clearing s3 bucket": {
			inApp:  mockApp,
			inEnv:  mockEnvName,
			inName: mockTaskName,

			wantedErr: errors.New("empty S3 bucket for task hide-snacks: some error"),

			setupMocks: func(m deleteTaskMocks) {
				m.sess.EXPECT().FromRole(mockEnv.ManagerRoleARN, mockEnv.Region).Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("mockRegion"),
					},
				}, nil)
				m.sess.EXPECT().DefaultWithRegion("mockRegion").Return(&session.Session{}, nil)
				gomock.InOrder(
					m.store.EXPECT().GetEnvironment(mockApp, mockEnvName).Return(mockEnv, nil),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecs.EXPECT().StopOneOffTasks(mockApp, mockEnvName, mockTaskName).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecr.EXPECT().ClearRepository(mockTaskRepoName).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.cfn.EXPECT().GetTaskStack(mockTaskName).Return(mockAppEnvTask, nil),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.s3.EXPECT().EmptyBucket(gomock.Any()).Return(mockError),
					m.spinner.EXPECT().Stop(gomock.Any()),
				)
			},
		},
		"error stopping app/env tasks": {
			inApp:  mockApp,
			inEnv:  mockEnvName,
			inName: mockTaskName,

			wantedErr: errors.New("stop running tasks in family hide-snacks: some error"),

			setupMocks: func(m deleteTaskMocks) {
				m.sess.EXPECT().FromRole(mockEnv.ManagerRoleARN, mockEnv.Region).Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("mockRegion"),
					},
				}, nil)
				gomock.InOrder(
					m.store.EXPECT().GetEnvironment(mockApp, mockEnvName).Return(mockEnv, nil),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecs.EXPECT().StopOneOffTasks(mockApp, mockEnvName, mockTaskName).Return(mockError),
					m.spinner.EXPECT().Stop(gomock.Any()),
				)
			},
		},
		"error getting task stack": {
			inApp:  mockApp,
			inEnv:  mockEnvName,
			inName: mockTaskName,

			wantedErr: errors.New("some error"),

			setupMocks: func(m deleteTaskMocks) {
				m.sess.EXPECT().FromRole(mockEnv.ManagerRoleARN, mockEnv.Region).Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("mockRegion"),
					},
				}, nil)
				m.sess.EXPECT().DefaultWithRegion("mockRegion").Return(&session.Session{}, nil)
				gomock.InOrder(
					m.store.EXPECT().GetEnvironment(mockApp, mockEnvName).Return(mockEnv, nil),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecs.EXPECT().StopOneOffTasks(mockApp, mockEnvName, mockTaskName).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecr.EXPECT().ClearRepository(mockTaskRepoName).Return(nil),
					m.spinner.EXPECT().Stop(gomock.Any()),
					m.cfn.EXPECT().GetTaskStack(mockTaskName).Return(nil, mockError),
				)
			},
		},
		"error stopping default cluster tasks": {
			inDefault: true,
			inName:    mockTaskName,

			wantedErr: errors.New("stop running tasks in family hide-snacks: some error"),

			setupMocks: func(m deleteTaskMocks) {
				m.sess.EXPECT().Default().Return(&session.Session{}, nil)
				gomock.InOrder(
					m.spinner.EXPECT().Start(gomock.Any()),
					m.ecs.EXPECT().StopDefaultClusterTasks(mockTaskName).Return(mockError),
					m.spinner.EXPECT().Stop(gomock.Any()),
				)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			mockstore := mocks.NewMockstore(ctrl)
			mockECR := mocks.NewMockimageRemover(ctrl)
			mockS3 := mocks.NewMockbucketEmptier(ctrl)
			mockCFN := mocks.NewMocktaskStackManager(ctrl)
			mockECS := mocks.NewMocktaskStopper(ctrl)
			//mockSession := sessions.NewProvider()
			mockSession := mocks.NewMocksessionProvider(ctrl)
			mockSpinner := mocks.NewMockprogress(ctrl)

			mockGetECR := func(_ *session.Session) imageRemover {
				return mockECR
			}
			mockGetS3 := func(_ *session.Session) bucketEmptier {
				return mockS3
			}
			mockGetECS := func(_ *session.Session) taskStopper {
				return mockECS
			}
			mockGetCFN := func(_ *session.Session) taskStackManager {
				return mockCFN
			}

			mocks := deleteTaskMocks{
				store:   mockstore,
				sess:    mockSession,
				ecr:     mockECR,
				s3:      mockS3,
				ecs:     mockECS,
				cfn:     mockCFN,
				spinner: mockSpinner,
			}

			tc.setupMocks(mocks)

			opts := deleteTaskOpts{
				deleteTaskVars: deleteTaskVars{
					app:            tc.inApp,
					env:            tc.inEnv,
					name:           tc.inName,
					defaultCluster: tc.inDefault,
				},
				store:    mockstore,
				provider: mockSession,
				spinner:  mockSpinner,

				newImageRemover:  mockGetECR,
				newBucketEmptier: mockGetS3,
				newStackManager:  mockGetCFN,
				newTaskStopper:   mockGetECS,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
			}

		})
	}
}

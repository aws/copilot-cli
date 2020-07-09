// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeleteSvcOpts_Validate(t *testing.T) {
	mockError := errors.New("some error")

	tests := map[string]struct {
		inAppName  string
		inEnvName  string
		inName     string
		setupMocks func(m *mocks.Mockstore)

		want error
	}{
		"should return errNoAppInWorkspace": {
			setupMocks: func(m *mocks.Mockstore) {},
			inName:     "api",
			want:       errNoAppInWorkspace,
		},
		"with no flag set": {
			inAppName:  "phonetool",
			setupMocks: func(m *mocks.Mockstore) {},
			want:       nil,
		},
		"with all flag set": {
			inAppName: "phonetool",
			inName:    "api",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetService("phonetool", "api").Times(1).Return(&config.Service{
					Name: "api",
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
		"with unknown environment": {
			inAppName: "phonetool",
			inEnvName: "test",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").Return(nil, errors.New("unknown env"))
			},
			want: errors.New("get environment test from config store: unknown env"),
		},
		"should return error if fail to get service name": {
			inAppName: "phonetool",
			inName:    "api",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetService("phonetool", "api").Times(1).Return(nil, mockError)
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

			opts := deleteSvcOpts{
				deleteSvcVars: deleteSvcVars{
					GlobalOpts: &GlobalOpts{
						appName: test.inAppName,
					},
					Name:    test.inName,
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

func TestDeleteSvcOpts_Ask(t *testing.T) {
	const (
		testAppName = "phonetool"
		testSvcName = "api"
	)
	mockError := errors.New("mockError")

	tests := map[string]struct {
		skipConfirmation bool
		inName           string

		mockstore  func(m *mocks.Mockstore)
		mockPrompt func(m *mocks.Mockprompter)

		wantedName  string
		wantedError error
	}{
		"should ask for service name": {
			inName:           "",
			skipConfirmation: true,
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices(testAppName).Return([]*config.Service{
					{
						Name: testSvcName,
					},
					{
						Name: "otherservice",
					},
				}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(svcDeleteNamePrompt, "", []string{testSvcName, "otherservice"}).Times(1).Return(testSvcName, nil)
			},

			wantedName: testSvcName,
		},
		"should skip asking for service name if only one service found": {
			inName:           "",
			skipConfirmation: true,
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices(testAppName).Return([]*config.Service{
					{
						Name: testSvcName,
					},
				}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedName: testSvcName,
		},
		"returns error if no services found": {
			inName:           "",
			skipConfirmation: true,
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices(testAppName).Return([]*config.Service{}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("couldn't find any services in the application phonetool"),
		},
		"returns error if fail to select service": {
			inName:           "",
			skipConfirmation: true,
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices(testAppName).Return([]*config.Service{
					{
						Name: testSvcName,
					},
					{
						Name: "otherservice",
					},
				}, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(svcDeleteNamePrompt, "", []string{testSvcName, "otherservice"}).Times(1).Return("", mockError)
			},

			wantedError: fmt.Errorf("select service to delete: %w", mockError),
		},
		"should skip confirmation": {
			inName:           testSvcName,
			skipConfirmation: true,
			mockstore:        func(m *mocks.Mockstore) {},
			mockPrompt:       func(m *mocks.Mockprompter) {},

			wantedName: testSvcName,
		},
		"should wrap error returned from prompter confirmation": {
			inName:           testSvcName,
			skipConfirmation: false,
			mockstore:        func(m *mocks.Mockstore) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(fmtSvcDeleteConfirmPrompt, testSvcName, testAppName),
					svcDeleteConfirmHelp,
				).Times(1).Return(true, mockError)
			},

			wantedError: fmt.Errorf("svc delete confirmation prompt: %w", mockError),
		},
		"should return error if user does not confirm svc delete": {
			inName:           testSvcName,
			skipConfirmation: false,
			mockstore:        func(m *mocks.Mockstore) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(fmtSvcDeleteConfirmPrompt, testSvcName, testAppName),
					svcDeleteConfirmHelp,
				).Times(1).Return(false, nil)
			},

			wantedError: errSvcDeleteCancelled,
		},
		"should return error nil if user confirms svc delete": {
			inName:           testSvcName,
			skipConfirmation: false,
			mockstore:        func(m *mocks.Mockstore) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(fmtSvcDeleteConfirmPrompt, testSvcName, testAppName),
					svcDeleteConfirmHelp,
				).Times(1).Return(true, nil)
			},

			wantedName: testSvcName,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompter := mocks.NewMockprompter(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			test.mockPrompt(mockPrompter)
			test.mockstore(mockStore)

			opts := deleteSvcOpts{
				deleteSvcVars: deleteSvcVars{
					SkipConfirmation: test.skipConfirmation,
					GlobalOpts: &GlobalOpts{
						appName: testAppName,
						prompt:  mockPrompter,
					},
					Name: test.inName,
				},
				store: mockStore,
			}

			got := opts.Ask()

			if got != nil {
				require.Equal(t, test.wantedError, got)
			} else {
				require.Equal(t, test.wantedName, opts.Name)
			}
		})
	}
}

type deleteSvcMocks struct {
	store          *mocks.Mockstore
	secretsmanager *mocks.MocksecretsManager
	sessProvider   *session.Provider
	appCFN         *mocks.MocksvcRemoverFromApp
	spinner        *mocks.Mockprogress
	svcCFN         *mocks.MocksvcDeleter
	ecr            *mocks.MockimageRemover
}

func TestDeleteSvcOpts_Execute(t *testing.T) {
	mockSvcName := "backend"
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

	mockRepo := fmt.Sprintf("%s/%s", mockAppName, mockSvcName)
	testError := errors.New("some error")

	tests := map[string]struct {
		inAppName string
		inEnvName string
		inSvcName string

		setupMocks func(mocks deleteSvcMocks)

		wantedError error
	}{
		"happy path with no environment passed in as flag": {
			inAppName: mockAppName,
			inSvcName: mockSvcName,
			setupMocks: func(mocks deleteSvcMocks) {
				gomock.InOrder(
					// appEnvironments
					mocks.store.EXPECT().ListEnvironments(gomock.Eq(mockAppName)).Times(1).Return(mockEnvs, nil),
					// deleteStacks
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtSvcDeleteStart, mockSvcName, mockEnvName)),
					mocks.svcCFN.EXPECT().DeleteService(gomock.Any()).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtSvcDeleteComplete, mockSvcName, mockEnvName)),
					// emptyECRRepos
					mocks.ecr.EXPECT().ClearRepository(mockRepo).Return(nil),

					// removeSvcFromApp
					mocks.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil),
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtSvcDeleteResourcesStart, mockSvcName, mockAppName)),
					mocks.appCFN.EXPECT().RemoveServiceFromApp(mockApp, mockSvcName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtSvcDeleteResourcesComplete, mockSvcName, mockAppName)),

					// deleteSSMParam
					mocks.store.EXPECT().DeleteService(mockAppName, mockSvcName).Return(nil),
				)
			},
			wantedError: nil,
		},
		"happy path with environment passed in as flag": {
			inAppName: mockAppName,
			inSvcName: mockSvcName,
			inEnvName: mockEnvName,
			setupMocks: func(mocks deleteSvcMocks) {
				gomock.InOrder(
					// appEnvironments
					mocks.store.EXPECT().GetEnvironment(mockAppName, mockEnvName).Times(1).Return(mockEnv, nil),
					// deleteStacks
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtSvcDeleteStart, mockSvcName, mockEnvName)),
					mocks.svcCFN.EXPECT().DeleteService(gomock.Any()).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtSvcDeleteComplete, mockSvcName, mockEnvName)),
					// emptyECRRepos
					mocks.ecr.EXPECT().ClearRepository(mockRepo).Return(nil),

					// removeSvcFromApp
					mocks.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil),
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtSvcDeleteResourcesStart, mockSvcName, mockAppName)),
					mocks.appCFN.EXPECT().RemoveServiceFromApp(mockApp, mockSvcName).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtSvcDeleteResourcesComplete, mockSvcName, mockAppName)),

					// deleteSSMParam
					mocks.store.EXPECT().DeleteService(mockAppName, mockSvcName).Return(nil),
				)
			},
			wantedError: nil,
		},
		"errors when deleting stack": {
			inAppName: mockAppName,
			inSvcName: mockSvcName,
			inEnvName: mockEnvName,
			setupMocks: func(mocks deleteSvcMocks) {
				gomock.InOrder(
					// appEnvironments
					mocks.store.EXPECT().GetEnvironment(mockAppName, mockEnvName).Times(1).Return(mockEnv, nil),
					// deleteStacks
					mocks.spinner.EXPECT().Start(fmt.Sprintf(fmtSvcDeleteStart, mockSvcName, mockEnvName)),
					mocks.svcCFN.EXPECT().DeleteService(gomock.Any()).Return(testError),
					mocks.spinner.EXPECT().Stop(log.Serrorf(fmtSvcDeleteFailed, mockSvcName, mockEnvName, testError)),
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
			mockSession := session.NewProvider()
			mockAppCFN := mocks.NewMocksvcRemoverFromApp(ctrl)
			mockSvcCFN := mocks.NewMocksvcDeleter(ctrl)
			mockSpinner := mocks.NewMockprogress(ctrl)
			mockImageRemover := mocks.NewMockimageRemover(ctrl)
			mockGetSvcCFN := func(_ *awssession.Session) svcDeleter {
				return mockSvcCFN
			}

			mockGetImageRemover := func(_ *awssession.Session) imageRemover {
				return mockImageRemover
			}
			mocks := deleteSvcMocks{
				store:          mockstore,
				secretsmanager: mockSecretsManager,
				sessProvider:   mockSession,
				appCFN:         mockAppCFN,
				spinner:        mockSpinner,
				svcCFN:         mockSvcCFN,
				ecr:            mockImageRemover,
			}

			test.setupMocks(mocks)

			opts := deleteSvcOpts{
				deleteSvcVars: deleteSvcVars{
					GlobalOpts: &GlobalOpts{
						appName: test.inAppName,
					},
					Name:    test.inSvcName,
					EnvName: test.inEnvName,
				},
				store:     mockstore,
				sess:      mockSession,
				spinner:   mockSpinner,
				appCFN:    mockAppCFN,
				getSvcCFN: mockGetSvcCFN,
				getECR:    mockGetImageRemover,
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

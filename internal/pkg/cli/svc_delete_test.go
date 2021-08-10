// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
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
		"with no flag set": {
			inAppName:  "phonetool",
			setupMocks: func(m *mocks.Mockstore) {},
			want:       nil,
		},
		"with all flags set": {
			inAppName: "phonetool",
			inEnvName: "test",
			inName:    "api",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(&config.Environment{Name: "test"}, nil)
				m.EXPECT().GetService("phonetool", "api").Times(1).Return(&config.Workload{
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
		"with svc flag set": {
			inAppName: "phonetool",
			inName:    "api",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetService("phonetool", "api").Times(1).Return(&config.Workload{
					Name: "api",
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

func TestDeleteSvcOpts_Ask(t *testing.T) {
	const (
		testAppName = "phonetool"
		testSvcName = "api"
	)
	mockError := errors.New("mockError")

	tests := map[string]struct {
		skipConfirmation bool
		inName           string
		envName          string
		appName          string

		mockSel    func(m *mocks.MockconfigSelector)
		mockPrompt func(m *mocks.Mockprompter)

		wantedName  string
		wantedError error
	}{
		"should ask for app name": {
			appName:          "",
			inName:           testSvcName,
			skipConfirmation: true,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Application(svcAppNamePrompt, svcAppNameHelpPrompt).Return(testAppName, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedName: testSvcName,
		},
		"should ask for service name": {
			appName:          testAppName,
			inName:           "",
			skipConfirmation: true,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Service("Which service would you like to delete?", "", testAppName).Return(testSvcName, nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedName: testSvcName,
		},
		"returns error if no services found": {
			appName:          testAppName,
			inName:           "",
			skipConfirmation: true,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Service("Which service would you like to delete?", "", testAppName).Return("", mockError)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedError: fmt.Errorf("select service: %w", mockError),
		},
		"returns error if fail to select service": {
			appName:          testAppName,
			inName:           "",
			skipConfirmation: true,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Service("Which service would you like to delete?", "", testAppName).Return("", mockError)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
			},

			wantedError: fmt.Errorf("select service: %w", mockError),
		},
		"should skip confirmation": {
			appName:          testAppName,
			inName:           testSvcName,
			skipConfirmation: true,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Service(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockPrompt: func(m *mocks.Mockprompter) {},

			wantedName: testSvcName,
		},
		"should wrap error returned from prompter confirmation": {
			appName:          testAppName,
			inName:           testSvcName,
			skipConfirmation: false,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Service(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(fmtSvcDeleteConfirmPrompt, testSvcName, testAppName),
					svcDeleteConfirmHelp,
					gomock.Any(),
				).Times(1).Return(true, mockError)
			},

			wantedError: fmt.Errorf("svc delete confirmation prompt: %w", mockError),
		},
		"should return error if user does not confirm svc delete": {
			appName:          testAppName,
			inName:           testSvcName,
			skipConfirmation: false,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Service(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(fmtSvcDeleteConfirmPrompt, testSvcName, testAppName),
					svcDeleteConfirmHelp,
					gomock.Any(),
				).Times(1).Return(false, nil)
			},

			wantedError: errSvcDeleteCancelled,
		},
		"should return error nil if user confirms svc delete": {
			appName:          testAppName,
			inName:           testSvcName,
			skipConfirmation: false,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Service(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(fmtSvcDeleteConfirmPrompt, testSvcName, testAppName),
					svcDeleteConfirmHelp,
					gomock.Any(),
				).Times(1).Return(true, nil)
			},

			wantedName: testSvcName,
		},
		"should return error nil if user confirms svc delete --env": {
			appName:          testAppName,
			inName:           testSvcName,
			envName:          "test",
			skipConfirmation: false,
			mockSel: func(m *mocks.MockconfigSelector) {
				m.EXPECT().Service(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Confirm(
					fmt.Sprintf(fmtSvcDeleteFromEnvConfirmPrompt, testSvcName, "test"),
					fmt.Sprintf(svcDeleteFromEnvConfirmHelp, "test"),
					gomock.Any(),
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
			mockSel := mocks.NewMockconfigSelector(ctrl)
			test.mockPrompt(mockPrompter)
			test.mockSel(mockSel)

			opts := deleteSvcOpts{
				deleteSvcVars: deleteSvcVars{
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

type deleteSvcMocks struct {
	store          *mocks.Mockstore
	secretsmanager *mocks.MocksecretsManager
	sessProvider   *sessions.Provider
	appCFN         *mocks.MocksvcRemoverFromApp
	spinner        *mocks.Mockprogress
	svcCFN         *mocks.MockwlDeleter
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
					mocks.svcCFN.EXPECT().DeleteWorkload(gomock.Any()).Return(nil),
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
		// A service can be deployed to multiple
		// environments - and deleting it in one
		// should not delete it form the entire app.
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
					mocks.svcCFN.EXPECT().DeleteWorkload(gomock.Any()).Return(nil),
					mocks.spinner.EXPECT().Stop(log.Ssuccessf(fmtSvcDeleteComplete, mockSvcName, mockEnvName)),

					// It should **not** emptyECRRepos
					mocks.ecr.EXPECT().ClearRepository(gomock.Any()).Return(nil).Times(0),

					// It should **not** removeSvcFromApp
					mocks.appCFN.EXPECT().RemoveServiceFromApp(gomock.Any(), gomock.Any()).Return(nil).Times(0),

					// It should **not** deleteSSMParam
					mocks.store.EXPECT().DeleteService(gomock.Any(), gomock.Any()).Return(nil).Times(0),
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
					mocks.svcCFN.EXPECT().DeleteWorkload(gomock.Any()).Return(testError),
					mocks.spinner.EXPECT().Stop(log.Serrorf(fmtSvcDeleteFailed, mockSvcName, mockEnvName, testError)),
				)
			},
			wantedError: fmt.Errorf("delete service: %w", testError),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			mockstore := mocks.NewMockstore(ctrl)
			mockSecretsManager := mocks.NewMocksecretsManager(ctrl)
			mockSession := sessions.NewProvider()
			mockAppCFN := mocks.NewMocksvcRemoverFromApp(ctrl)
			mockSvcCFN := mocks.NewMockwlDeleter(ctrl)
			mockSpinner := mocks.NewMockprogress(ctrl)
			mockImageRemover := mocks.NewMockimageRemover(ctrl)
			mockGetSvcCFN := func(_ *session.Session) wlDeleter {
				return mockSvcCFN
			}

			mockGetImageRemover := func(_ *session.Session) imageRemover {
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
					appName: test.inAppName,
					name:    test.inSvcName,
					envName: test.inEnvName,
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
				require.NoError(t, err)
			}
		})
	}
}

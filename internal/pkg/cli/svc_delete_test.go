// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/cli/clean/cleantest"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestDeleteSvcOpts_Validate(t *testing.T) {
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
			inAppName:  "phonetool",
			inEnvName:  "test",
			inName:     "api",
			setupMocks: func(m *mocks.Mockstore) {},
			want:       nil,
		},
		"with env flag set": {
			inAppName:  "phonetool",
			inEnvName:  "test",
			setupMocks: func(m *mocks.Mockstore) {},
			want:       nil,
		},
		"with svc flag set": {
			inAppName:  "phonetool",
			inName:     "api",
			setupMocks: func(m *mocks.Mockstore) {},
			want:       nil,
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

type svcDeleteAskMocks struct {
	store  *mocks.Mockstore
	prompt *mocks.Mockprompter
	sel    *mocks.MockconfigSelector
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
		setUpMocks func(m *svcDeleteAskMocks)

		wantedName  string
		wantedError error
	}{
		"validate app name if passed as a flag": {
			appName:          testAppName,
			inName:           testSvcName,
			skipConfirmation: true,
			setUpMocks: func(m *svcDeleteAskMocks) {
				m.store.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{}, nil)
				m.sel.EXPECT().Application(gomock.Any(), gomock.Any()).Times(0)
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedName: testSvcName,
		},
		"error validating app name": {
			appName:          testAppName,
			inName:           testSvcName,
			skipConfirmation: true,
			setUpMocks: func(m *svcDeleteAskMocks) {
				m.store.EXPECT().GetApplication(gomock.Any()).Return(nil, &config.ErrNoSuchApplication{})
				m.sel.EXPECT().Application(gomock.Any(), gomock.Any()).Times(0)
			},
			wantedError: &config.ErrNoSuchApplication{},
		},
		"should ask for app name": {
			appName:          "",
			inName:           testSvcName,
			skipConfirmation: true,
			setUpMocks: func(m *svcDeleteAskMocks) {
				m.sel.EXPECT().Application(svcAppNamePrompt, wkldAppNameHelpPrompt).Return(testAppName, nil)
				m.store.EXPECT().GetApplication(gomock.Any()).Times(0)
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).AnyTimes()
			},
			wantedName: testSvcName,
		},
		"validate service name if passed as a flag": {
			appName:          testAppName,
			inName:           testSvcName,
			skipConfirmation: true,
			setUpMocks: func(m *svcDeleteAskMocks) {
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).Return(&config.Workload{}, nil)
				m.sel.EXPECT().Service(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedName: testSvcName,
		},
		"error validating service name": {
			appName:          testAppName,
			inName:           testSvcName,
			skipConfirmation: true,
			setUpMocks: func(m *svcDeleteAskMocks) {
				m.store.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{}, nil)
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedError: errors.New("some error"),
		},
		"should ask for service name": {
			appName:          testAppName,
			inName:           "",
			skipConfirmation: true,
			setUpMocks: func(m *svcDeleteAskMocks) {
				m.sel.EXPECT().Service("Which service would you like to delete?", "", testAppName).Return(testSvcName, nil)
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).Times(0)
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
			},
			wantedName: testSvcName,
		},
		"returns error if no services found": {
			appName:          testAppName,
			inName:           "",
			skipConfirmation: true,
			setUpMocks: func(m *svcDeleteAskMocks) {
				m.sel.EXPECT().Service("Which service would you like to delete?", "", testAppName).Return("", mockError)
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
			},
			wantedError: fmt.Errorf("select service: %w", mockError),
		},
		"returns error if fail to select service": {
			appName:          testAppName,
			inName:           "",
			skipConfirmation: true,
			setUpMocks: func(m *svcDeleteAskMocks) {
				m.sel.EXPECT().Service("Which service would you like to delete?", "", testAppName).Return("", mockError)
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
			},
			wantedError: fmt.Errorf("select service: %w", mockError),
		},
		"should skip confirmation": {
			appName:          testAppName,
			inName:           testSvcName,
			skipConfirmation: true,
			setUpMocks: func(m *svcDeleteAskMocks) {
				m.prompt.EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any).Times(0)
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).AnyTimes()
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
			},
			wantedName: testSvcName,
		},
		"should wrap error returned from prompter confirmation": {
			appName:          testAppName,
			inName:           testSvcName,
			skipConfirmation: false,
			setUpMocks: func(m *svcDeleteAskMocks) {
				m.prompt.EXPECT().Confirm(
					fmt.Sprintf(fmtSvcDeleteConfirmPrompt, testSvcName, testAppName),
					svcDeleteConfirmHelp,
					gomock.Any(),
				).Times(1).Return(true, mockError)
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).AnyTimes()
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()

			},
			wantedError: fmt.Errorf("svc delete confirmation prompt: %w", mockError),
		},
		"should return error if user does not confirm svc delete": {
			appName:          testAppName,
			inName:           testSvcName,
			skipConfirmation: false,
			setUpMocks: func(m *svcDeleteAskMocks) {
				m.prompt.EXPECT().Confirm(
					fmt.Sprintf(fmtSvcDeleteConfirmPrompt, testSvcName, testAppName),
					svcDeleteConfirmHelp,
					gomock.Any(),
				).Times(1).Return(false, nil)
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).AnyTimes()
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
			},
			wantedError: errSvcDeleteCancelled,
		},
		"should return error nil if user confirms svc delete": {
			appName:          testAppName,
			inName:           testSvcName,
			skipConfirmation: false,
			setUpMocks: func(m *svcDeleteAskMocks) {
				m.prompt.EXPECT().Confirm(
					fmt.Sprintf(fmtSvcDeleteConfirmPrompt, testSvcName, testAppName),
					svcDeleteConfirmHelp,
					gomock.Any(),
				).Times(1).Return(true, nil)
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).AnyTimes()
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
			},
			wantedName: testSvcName,
		},
		"should return error nil if user confirms svc delete --env": {
			appName:          testAppName,
			inName:           testSvcName,
			envName:          "test",
			skipConfirmation: false,
			setUpMocks: func(m *svcDeleteAskMocks) {
				m.store.EXPECT().GetEnvironment(testAppName, "test").Return(&config.Environment{}, nil)
				m.prompt.EXPECT().Confirm(
					fmt.Sprintf(fmtSvcDeleteFromEnvConfirmPrompt, testSvcName, "test"),
					fmt.Sprintf(svcDeleteFromEnvConfirmHelp, "test"),
					gomock.Any(),
				).Times(1).Return(true, nil)
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).AnyTimes()
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
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
			mockStore := mocks.NewMockstore(ctrl)

			m := &svcDeleteAskMocks{
				sel:    mockSel,
				prompt: mockPrompter,
				store:  mockStore,
			}
			test.setUpMocks(m)
			opts := deleteSvcOpts{
				deleteSvcVars: deleteSvcVars{
					skipConfirmation: test.skipConfirmation,
					appName:          test.appName,
					name:             test.inName,
					envName:          test.envName,
				},
				prompt: m.prompt,
				sel:    m.sel,
				store:  m.store,
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
	sessProvider   *mocks.MocksessionProvider
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
		opts      *deleteSvcOpts

		wkldCleaner cleaner
		setupMocks  func(mocks deleteSvcMocks)

		wantedError error
	}{
		"happy path with no environment passed in as flag": {
			opts: &deleteSvcOpts{
				deleteSvcVars: deleteSvcVars{
					appName: mockAppName,
					name:    mockSvcName,
				},
				newSvcCleaner: func(*session.Session, *config.Environment, string) cleaner {
					return &cleantest.Succeeds{}
				},
			},
			setupMocks: func(mocks deleteSvcMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetWorkload(mockAppName, mockSvcName).Return(&config.Workload{
						Type: manifestinfo.LoadBalancedWebServiceType,
					}, nil),

					// appEnvironments
					mocks.store.EXPECT().ListEnvironments(gomock.Eq(mockAppName)).Times(1).Return(mockEnvs, nil),

					mocks.sessProvider.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{}, nil),
					// deleteStacks
					mocks.svcCFN.EXPECT().DeleteWorkload(gomock.Any()).Return(nil),

					mocks.sessProvider.EXPECT().DefaultWithRegion(gomock.Any()).Return(&session.Session{}, nil),

					// emptyECRRepos
					mocks.ecr.EXPECT().ClearRepository(mockRepo).Return(nil),

					// removeSvcFromApp
					mocks.store.EXPECT().GetApplication(mockAppName).Return(mockApp, nil),
					mocks.appCFN.EXPECT().RemoveServiceFromApp(mockApp, mockSvcName).Return(nil),

					// deleteSSMParam
					mocks.store.EXPECT().DeleteService(mockAppName, mockSvcName).Return(nil),
				)
			},
			wantedError: nil,
		},
		// A service can be deployed to multiple
		// environments - and deleting it in one
		// should not delete it from the entire app.
		"happy path with environment passed in as flag": {
			opts: &deleteSvcOpts{
				deleteSvcVars: deleteSvcVars{
					appName: mockAppName,
					envName: mockEnvName,
					name:    mockSvcName,
				},
				newSvcCleaner: func(*session.Session, *config.Environment, string) cleaner {
					return &cleantest.Succeeds{}
				},
			},
			setupMocks: func(mocks deleteSvcMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetWorkload(mockAppName, mockSvcName).Return(&config.Workload{
						Type: manifestinfo.LoadBalancedWebServiceType,
					}, nil),

					// appEnvironments
					mocks.store.EXPECT().GetEnvironment(mockAppName, mockEnvName).Times(1).Return(mockEnv, nil),

					mocks.sessProvider.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{}, nil),
					// deleteStacks
					mocks.svcCFN.EXPECT().DeleteWorkload(gomock.Any()).Return(nil),

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
		"error getting workload": {
			opts: &deleteSvcOpts{
				deleteSvcVars: deleteSvcVars{
					appName: mockAppName,
					name:    mockSvcName,
				},
			},
			setupMocks: func(mocks deleteSvcMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetWorkload(mockAppName, mockSvcName).Return(nil, errors.New("some error")),
				)
			},
			wantedError: errors.New("get workload: some error"),
		},
		"error cleaning workload": {
			opts: &deleteSvcOpts{
				deleteSvcVars: deleteSvcVars{
					appName: mockAppName,
					envName: mockEnvName,
					name:    mockSvcName,
				},
				newSvcCleaner: func(*session.Session, *config.Environment, string) cleaner {
					return &cleantest.Fails{}
				},
			},
			setupMocks: func(mocks deleteSvcMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetWorkload(mockAppName, mockSvcName).Return(&config.Workload{
						Type: manifestinfo.LoadBalancedWebServiceType,
					}, nil),
					mocks.store.EXPECT().GetEnvironment(mockAppName, mockEnvName).Times(1).Return(mockEnv, nil),
					mocks.sessProvider.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{}, nil),
				)
			},
			wantedError: errors.New("clean resources: an error"),
		},
		"errors when deleting stack": {
			opts: &deleteSvcOpts{
				deleteSvcVars: deleteSvcVars{
					appName: mockAppName,
					envName: mockEnvName,
					name:    mockSvcName,
				},
				newSvcCleaner: func(*session.Session, *config.Environment, string) cleaner {
					return &cleantest.Succeeds{}
				},
			},
			setupMocks: func(mocks deleteSvcMocks) {
				gomock.InOrder(
					mocks.store.EXPECT().GetWorkload(mockAppName, mockSvcName).Return(&config.Workload{
						Type: manifestinfo.LoadBalancedWebServiceType,
					}, nil),

					// appEnvironments
					mocks.store.EXPECT().GetEnvironment(mockAppName, mockEnvName).Times(1).Return(mockEnv, nil),

					mocks.sessProvider.EXPECT().FromRole(gomock.Any(), gomock.Any()).Return(&session.Session{}, nil),
					// deleteStacks
					mocks.svcCFN.EXPECT().DeleteWorkload(gomock.Any()).Return(testError),
				)
			},
			wantedError: fmt.Errorf("delete service: %w", testError),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := deleteSvcMocks{
				store:          mocks.NewMockstore(ctrl),
				secretsmanager: mocks.NewMocksecretsManager(ctrl),
				sessProvider:   mocks.NewMocksessionProvider(ctrl),
				appCFN:         mocks.NewMocksvcRemoverFromApp(ctrl),
				spinner:        mocks.NewMockprogress(ctrl),
				svcCFN:         mocks.NewMockwlDeleter(ctrl),
				ecr:            mocks.NewMockimageRemover(ctrl),
			}

			tc.setupMocks(mocks)

			tc.opts.store = mocks.store
			tc.opts.sess = mocks.sessProvider
			tc.opts.spinner = mocks.spinner
			tc.opts.appCFN = mocks.appCFN
			tc.opts.getSvcCFN = func(_ *session.Session) wlDeleter {
				return mocks.svcCFN
			}
			tc.opts.getECR = func(_ *session.Session) imageRemover {
				return mocks.ecr
			}

			// WHEN
			err := tc.opts.Execute()

			// THEN
			if tc.wantedError != nil {
				require.EqualError(t, err, tc.wantedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

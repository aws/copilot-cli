// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
)

func TestResumeSvcOpts_Validate(t *testing.T) {
	// NOTE: No optional flag to `copilot svc pause` needs to be validated.
}

type svcResumeAskMock struct {
	store *mocks.Mockstore
	sel   *mocks.MockdeploySelector
}

func TestResumeSvcOpts_Ask(t *testing.T) {
	const (
		testAppName = "phonetool"
		testEnvName = "test"
		testSvcName = "api"
	)
	mockError := fmt.Errorf("mockError")

	tests := map[string]struct {
		skipConfirmation bool
		inputSvc         string
		inputEnv         string
		inputApp         string

		setupMocks func(m svcResumeAskMock)

		wantedApp   string
		wantedEnv   string
		wantedSvc   string
		wantedError error
	}{
		"validate app env and svc with all flags passed in": {
			inputApp: testAppName,
			inputSvc: testSvcName,
			inputEnv: testEnvName,
			setupMocks: func(m svcResumeAskMock) {
				gomock.InOrder(
					m.store.EXPECT().GetApplication("phonetool").Return(&config.Application{Name: "phonetool"}, nil),
					m.store.EXPECT().GetEnvironment("phonetool", "test").Return(&config.Environment{Name: "test"}, nil),
					m.store.EXPECT().GetService("phonetool", "api").Return(&config.Workload{}, nil),
				)
				m.sel.EXPECT().DeployedService(fmt.Sprintf(svcResumeSvcNamePrompt, testAppName), svcResumeSvcNameHelpPrompt, "phonetool", gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env:  "test",
						Name: "api",
					}, nil) // Let prompter handles the case when svc(env) is definite.
			},
			wantedApp: testAppName,
			wantedEnv: testEnvName,
			wantedSvc: testSvcName,
		},
		"prompt for app name": {
			inputEnv:         testEnvName,
			inputSvc:         testSvcName,
			skipConfirmation: true,
			setupMocks: func(m svcResumeAskMock) {
				m.sel.EXPECT().Application(svcAppNamePrompt, wkldAppNameHelpPrompt).Return("phonetool", nil)
				m.store.EXPECT().GetApplication(gomock.Any()).Times(0)
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).AnyTimes()
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).AnyTimes()
				m.sel.EXPECT().DeployedService(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env:  testEnvName,
						Name: testSvcName,
					}, nil).AnyTimes()
			},
			wantedApp: testAppName,
			wantedEnv: testEnvName,
			wantedSvc: testSvcName,
		},
		"errors if failed to select application": {
			skipConfirmation: true,
			setupMocks: func(m svcResumeAskMock) {
				m.sel.EXPECT().Application(svcAppNamePrompt, wkldAppNameHelpPrompt).Return("", errors.New("some error"))
			},
			wantedError: fmt.Errorf("select application: some error"),
		},
		"prompt for service and env": {
			inputApp:         testAppName,
			inputEnv:         "",
			inputSvc:         "",
			skipConfirmation: true,
			setupMocks: func(m svcResumeAskMock) {
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().DeployedService("Which service of phonetool would you like to resume?",
					svcResumeSvcNameHelpPrompt, testAppName, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(&selector.DeployedService{
						Env:  testEnvName,
						Name: testSvcName,
					}, nil)
			},
			wantedApp: testAppName,
			wantedEnv: testEnvName,
			wantedSvc: testSvcName,
		},
		"returns error if fails to select service": {
			inputApp:         testAppName,
			inputEnv:         "",
			inputSvc:         "",
			skipConfirmation: true,

			setupMocks: func(m svcResumeAskMock) {
				m.store.EXPECT().GetApplication(gomock.Any()).AnyTimes()
				m.store.EXPECT().GetEnvironment(gomock.Any(), gomock.Any()).Times(0)
				m.store.EXPECT().GetService(gomock.Any(), gomock.Any()).Times(0)
				m.sel.EXPECT().DeployedService("Which service of phonetool would you like to resume?",
					svcResumeSvcNameHelpPrompt, testAppName, gomock.Any(), gomock.Any(), gomock.Any()).
					Return(nil, mockError)
			},
			wantedError: fmt.Errorf("select deployed service for application phonetool: %w", mockError),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := svcResumeAskMock{
				store: mocks.NewMockstore(ctrl),
				sel:   mocks.NewMockdeploySelector(ctrl),
			}
			test.setupMocks(m)
			opts := resumeSvcOpts{
				resumeSvcVars: resumeSvcVars{
					appName: test.inputApp,
					svcName: test.inputSvc,
					envName: test.inputEnv,
				},
				sel:   m.sel,
				store: m.store,
			}

			err := opts.Ask()

			// THEN
			if test.wantedError != nil {
				require.EqualError(t, err, test.wantedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, test.wantedApp, opts.appName, "expected app name to match")
				require.Equal(t, test.wantedSvc, opts.svcName, "expected service name to match")
				require.Equal(t, test.wantedEnv, opts.envName, "expected service name to match")
			}
		})
	}
}

type resumeSvcMocks struct {
	store              *mocks.Mockstore
	spinner            *mocks.Mockprogress
	serviceResumer     *mocks.MockserviceResumer
	apprunnerDescriber *mocks.MockapprunnerServiceDescriber
}

func TestResumeSvcOpts_Execute(t *testing.T) {
	const (
		testAppName = "phonetool"
		testEnvName = "test"
		testSvcName = "phonetool"
		testSvcARN  = "service-arn"
	)
	mockError := fmt.Errorf("mockError")

	tests := map[string]struct {
		appName string
		envName string
		svcName string

		setupMocks func(mocks *resumeSvcMocks)

		wantedError error
	}{
		"happy path": {
			appName: testAppName,
			envName: testEnvName,
			svcName: testSvcName,
			setupMocks: func(m *resumeSvcMocks) {
				m.apprunnerDescriber.EXPECT().ServiceARN(testEnvName).Return(testSvcARN, nil)
				gomock.InOrder(
					m.spinner.EXPECT().Start("Resuming service phonetool in environment test."),
					m.serviceResumer.EXPECT().ResumeService(testSvcARN).Return(nil),
					m.spinner.EXPECT().Stop(log.Ssuccessf("Resumed service phonetool in environment test.\n")),
				)
			},
			wantedError: nil,
		},
		"return error if fails to retrieve service ARN": {
			appName: testAppName,
			envName: testEnvName,
			svcName: testSvcName,
			setupMocks: func(m *resumeSvcMocks) {
				m.apprunnerDescriber.EXPECT().ServiceARN(testEnvName).Return("", mockError)
			},
			wantedError: mockError,
		},
		"should display failure spinner and return error if ResumeService fails": {
			appName: testAppName,
			envName: testEnvName,
			svcName: testSvcName,
			setupMocks: func(m *resumeSvcMocks) {
				m.apprunnerDescriber.EXPECT().ServiceARN(testEnvName).Return(testSvcARN, nil)
				gomock.InOrder(
					m.spinner.EXPECT().Start("Resuming service phonetool in environment test."),
					m.serviceResumer.EXPECT().ResumeService(testSvcARN).Return(mockError),
					m.spinner.EXPECT().Stop(log.Serrorf("Failed to resume service phonetool in environment test: mockError\n")),
				)
			},
			wantedError: mockError,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			// GIVEN
			mockstore := mocks.NewMockstore(ctrl)
			mockSpinner := mocks.NewMockprogress(ctrl)
			mockserviceResumer := mocks.NewMockserviceResumer(ctrl)
			mockapprunnerDescriber := mocks.NewMockapprunnerServiceDescriber(ctrl)

			mocks := &resumeSvcMocks{
				store:              mockstore,
				spinner:            mockSpinner,
				serviceResumer:     mockserviceResumer,
				apprunnerDescriber: mockapprunnerDescriber,
			}

			test.setupMocks(mocks)

			opts := resumeSvcOpts{
				resumeSvcVars: resumeSvcVars{
					appName: test.appName,
					envName: test.envName,
					svcName: test.svcName,
				},
				store:              mockstore,
				spinner:            mockSpinner,
				serviceResumer:     mockserviceResumer,
				apprunnerDescriber: mockapprunnerDescriber,
				initClients: func() error {
					return nil
				},
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

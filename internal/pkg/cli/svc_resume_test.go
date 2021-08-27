// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/term/selector"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestResumeSvcOpts_Validate(t *testing.T) {
	mockError := fmt.Errorf("some error")

	tests := map[string]struct {
		inAppName  string
		inEnvName  string
		inName     string
		setupMocks func(m *mocks.Mockstore)

		want error
	}{
		"skip validation if app flag is not set": {
			inEnvName:  "test",
			inName:     "frontend",
			setupMocks: func(m *mocks.Mockstore) {},
		},
		"with no flag set": {
			inAppName: "phonetool",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			want: nil,
		},
		"with all flags set": {
			inAppName: "phonetool",
			inEnvName: "test",
			inName:    "frontend",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(&config.Environment{Name: "test"}, nil)
				m.EXPECT().GetService("phonetool", "frontend").Times(1).Return(&config.Workload{
					Name: "frontend",
				}, nil)
			},
			want: nil,
		},
		"with env flag set": {
			inAppName: "phonetool",
			inEnvName: "test",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
				m.EXPECT().GetEnvironment("phonetool", "test").
					Return(&config.Environment{Name: "test"}, nil)
			},
			want: nil,
		},
		"with svc flag set": {
			inAppName: "phonetool",
			inName:    "api",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
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
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
				m.EXPECT().GetEnvironment("phonetool", "test").Return(nil, mockError)
			},
			want: mockError,
		},
		"should return error if fail to get service name": {
			inAppName: "phonetool",
			inName:    "api",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
				m.EXPECT().GetService("phonetool", "api").Times(1).Return(nil, mockError)
			},
			want: mockError,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockstore := mocks.NewMockstore(ctrl)

			test.setupMocks(mockstore)

			opts := resumeSvcOpts{
				resumeSvcVars: resumeSvcVars{
					appName: test.inAppName,
					svcName: test.inName,
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

func TestResumeSvcOpts_Ask(t *testing.T) {
	const (
		testAppName = "phonetool"
		testEnvName = "test"
		testSvcName = "api"
	)
	mockError := fmt.Errorf("mockError")

	tests := map[string]struct {
		skipConfirmation bool
		svcName          string
		envName          string
		appName          string

		mockSel func(m *mocks.MockdeploySelector)

		wantedAppName string
		wantedEnvName string
		wantedSvcName string
		wantedError   error
	}{
		"should ask for app name": {
			appName:          "",
			envName:          testEnvName,
			svcName:          testSvcName,
			skipConfirmation: true,
			mockSel: func(m *mocks.MockdeploySelector) {
				m.EXPECT().Application(svcAppNamePrompt, svcAppNameHelpPrompt).Return(testAppName, nil)
				m.EXPECT().DeployedService(
					"Which service of phonetool would you like to resume?",
					svcResumeSvcNameHelpPrompt,
					testAppName,
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(&selector.DeployedService{
					Svc: testSvcName,
					Env: testEnvName,
				}, nil)
			},

			wantedAppName: testAppName,
		},
		"should ask for service name": {
			appName:          testAppName,
			envName:          "",
			svcName:          "",
			skipConfirmation: true,
			mockSel: func(m *mocks.MockdeploySelector) {
				m.EXPECT().DeployedService(
					"Which service of phonetool would you like to resume?",
					svcResumeSvcNameHelpPrompt,
					testAppName,
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(&selector.DeployedService{
					Svc: testSvcName,
					Env: testEnvName,
				}, nil)
			},

			wantedSvcName: testSvcName,
		},
		"returns error if fails to select service": {
			appName:          testAppName,
			envName:          "",
			svcName:          "",
			skipConfirmation: true,
			mockSel: func(m *mocks.MockdeploySelector) {
				m.EXPECT().DeployedService(
					"Which service of phonetool would you like to resume?",
					svcResumeSvcNameHelpPrompt,
					testAppName,
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(nil, mockError)
			},

			wantedError: fmt.Errorf("select deployed service for application phonetool: %w", mockError),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSel := mocks.NewMockdeploySelector(ctrl)
			test.mockSel(mockSel)

			opts := resumeSvcOpts{
				resumeSvcVars: resumeSvcVars{
					appName: test.appName,
					svcName: test.svcName,
					envName: test.envName,
				},
				sel: mockSel,
			}

			err := opts.Ask()

			if test.wantedError != nil {
				require.Equal(t, test.wantedError, err)
			} else {
				require.NoError(t, err)
			}

			if test.wantedAppName != "" {
				require.Equal(t, test.wantedAppName, opts.appName)
			}

			if test.wantedEnvName != "" {
				require.Equal(t, test.wantedEnvName, opts.envName)
			}

			if test.wantedSvcName != "" {
				require.Equal(t, test.wantedSvcName, opts.svcName)
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
				m.apprunnerDescriber.EXPECT().ServiceARN().Return(testSvcARN, nil)
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
				m.apprunnerDescriber.EXPECT().ServiceARN().Return("", mockError)
			},
			wantedError: mockError,
		},
		"should display failure spinner and return error if ResumeService fails": {
			appName: testAppName,
			envName: testEnvName,
			svcName: testSvcName,
			setupMocks: func(m *resumeSvcMocks) {
				m.apprunnerDescriber.EXPECT().ServiceARN().Return(testSvcARN, nil)
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
			// foo := mocks.NewMock

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

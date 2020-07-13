// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/identity"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestInitAppOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inAppName string
		expect    func(opts *initAppOpts)

		wantedAppName string
		wantedErr     string
	}{
		"errors if summary exists and differs from app argument": {
			inAppName: "testname",
			expect: func(opts *initAppOpts) {
				opts.ws.(*mocks.MockwsAppManager).EXPECT().Summary().Return(&workspace.Summary{Application: "metrics"}, nil)
				opts.store.(*mocks.Mockstore).EXPECT().ListApplications().Times(0)
			},
			wantedErr: "workspace already registered with metrics",
		},
		"use argument if there is no summary": {
			inAppName: "metrics",
			expect: func(opts *initAppOpts) {
				opts.ws.(*mocks.MockwsAppManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.store.(*mocks.Mockstore).EXPECT().ListApplications().Times(0)
			},
			wantedAppName: "metrics",
		},
		"return error from new app name": {
			expect: func(opts *initAppOpts) {
				opts.ws.(*mocks.MockwsAppManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.store.(*mocks.Mockstore).EXPECT().ListApplications().Return([]*config.Application{}, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("my error"))
				opts.prompt.(*mocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				opts.prompt.(*mocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "prompt get application name: my error",
		},
		"enter new app name if no existing apps": {
			expect: func(opts *initAppOpts) {
				opts.ws.(*mocks.MockwsAppManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.store.(*mocks.Mockstore).EXPECT().ListApplications().Return([]*config.Application{}, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				opts.prompt.(*mocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedAppName: "metrics",
		},
		"return error from app selection": {
			expect: func(opts *initAppOpts) {
				opts.ws.(*mocks.MockwsAppManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.store.(*mocks.Mockstore).EXPECT().ListApplications().Return([]*config.Application{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("my error"))
			},
			wantedErr: "prompt select application name: my error",
		},
		"use from existing apps": {
			expect: func(opts *initAppOpts) {
				opts.ws.(*mocks.MockwsAppManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.store.(*mocks.Mockstore).EXPECT().ListApplications().Return([]*config.Application{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
			},
			wantedAppName: "metrics",
		},
		"enter new app name if user opts out of selection": {
			expect: func(opts *initAppOpts) {
				opts.ws.(*mocks.MockwsAppManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.store.(*mocks.Mockstore).EXPECT().ListApplications().Return([]*config.Application{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				opts.prompt.(*mocks.Mockprompter).EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
			},
			wantedAppName: "metrics",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			opts := &initAppOpts{
				initAppVars: initAppVars{
					AppName: tc.inAppName,
				},
				store:  mocks.NewMockstore(ctrl),
				ws:     mocks.NewMockwsAppManager(ctrl),
				prompt: mocks.NewMockprompter(ctrl),
			}
			tc.expect(opts)

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedAppName, opts.AppName)
			}
		})
	}
}

func TestInitAppOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName      string
		inDomainName   string
		mockRoute53Svc func(m *mocks.MockdomainValidator)
		mockStore      func(m *mocks.Mockstore)

		wantedError string
	}{
		"skip everything": {
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {},
			mockStore:      func(m *mocks.Mockstore) {},

			wantedError: "",
		},
		"valid app name": {
			inAppName:      "metrics",
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("metrics").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "metrics",
				})
			},
			wantedError: "",
		},
		"invalid app name": {
			inAppName:      "123chicken",
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {},
			mockStore:      func(m *mocks.Mockstore) {},

			wantedError: "application name 123chicken is invalid: value must start with a letter and contain only lower-case letters, numbers, and hyphens",
		},
		"errors if application with different domain already exists": {
			inAppName:      "metrics",
			inDomainName:   "badDomain.com",
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("metrics").Return(&config.Application{
					Name:   "metrics",
					Domain: "domain.com",
				}, nil)
			},

			wantedError: "application named metrics already exists with a different domain name domain.com",
		},
		"errors if failed to get application": {
			inAppName:      "metrics",
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("metrics").Return(nil, errors.New("some error"))
			},

			wantedError: "get application metrics: some error",
		},
		"valid domain name": {
			inDomainName: "mockDomain.com",
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {
				m.EXPECT().DomainExists("mockDomain.com").Return(true, nil)
			},
			mockStore:   func(m *mocks.Mockstore) {},
			wantedError: "",
		},
		"invalid domain name that does not exist": {
			inDomainName: "badMockDomain.com",
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {
				m.EXPECT().DomainExists("badMockDomain.com").Return(false, nil)
			},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: "no hosted zone found for badMockDomain.com",
		},
		"errors if failed to validate domain name": {
			inDomainName: "mockDomain.com",
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {
				m.EXPECT().DomainExists("mockDomain.com").Return(false, errors.New("some error"))
			},
			mockStore: func(m *mocks.Mockstore) {},

			wantedError: "some error",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockRoute53Svc := mocks.NewMockdomainValidator(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			tc.mockRoute53Svc(mockRoute53Svc)
			tc.mockStore(mockStore)
			opts := &initAppOpts{
				route53: mockRoute53Svc,
				store:   mockStore,
				initAppVars: initAppVars{
					AppName:    tc.inAppName,
					DomainName: tc.inDomainName,
				},
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedError != "" {
				require.EqualError(t, err, tc.wantedError)
			} else {
				require.Nil(t, err)
			}
		})
	}
}

func TestInitAppOpts_Execute(t *testing.T) {
	mockError := fmt.Errorf("error")

	testCases := map[string]struct {
		inDomainName string

		expectedError error
		mocking       func(t *testing.T,
			mockstore *mocks.Mockstore, mockWorkspace *mocks.MockwsAppManager,
			mockIdentityService *mocks.MockidentityService, mockDeployer *mocks.MockappDeployer,
			mockProgress *mocks.Mockprogress)
	}{
		"with a successful call to add app": {
			inDomainName: "amazon.com",

			mocking: func(t *testing.T, mockstore *mocks.Mockstore, mockWorkspace *mocks.MockwsAppManager,
				mockIdentityService *mocks.MockidentityService, mockDeployer *mocks.MockappDeployer,
				mockProgress *mocks.Mockprogress) {
				mockIdentityService.
					EXPECT().
					Get().
					Return(identity.Caller{
						Account: "12345",
					}, nil)
				mockstore.
					EXPECT().
					CreateApplication(&config.Application{
						AccountID: "12345",
						Name:      "myapp",
						Domain:    "amazon.com",
						Tags: map[string]string{
							"owner": "boss",
						},
					})
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("myapp")).Return(nil)
				mockProgress.EXPECT().Start(fmt.Sprintf(fmtAppInitStart, "myapp"))
				mockDeployer.EXPECT().
					DeployApp(&deploy.CreateAppInput{
						Name:       "myapp",
						AccountID:  "12345",
						DomainName: "amazon.com",
						AdditionalTags: map[string]string{
							"owner": "boss",
						},
					}).Return(nil)
				mockProgress.EXPECT().Stop(log.Ssuccessf(fmtAppInitComplete, "myapp"))
			},
		},
		"should return error from workspace.Create": {
			expectedError: mockError,
			mocking: func(t *testing.T, mockstore *mocks.Mockstore, mockWorkspace *mocks.MockwsAppManager,
				mockIdentityService *mocks.MockidentityService, mockDeployer *mocks.MockappDeployer,
				mockProgress *mocks.Mockprogress) {
				mockIdentityService.
					EXPECT().
					Get().
					Return(identity.Caller{
						Account: "12345",
					}, nil)
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("myapp")).
					Return(mockError)
			},
		},
		"with an error while deploying myapp": {
			expectedError: mockError,
			mocking: func(t *testing.T, mockstore *mocks.Mockstore, mockWorkspace *mocks.MockwsAppManager,
				mockIdentityService *mocks.MockidentityService, mockDeployer *mocks.MockappDeployer,
				mockProgress *mocks.Mockprogress) {
				mockIdentityService.
					EXPECT().
					Get().
					Return(identity.Caller{
						Account: "12345",
					}, nil)
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("myapp")).Return(nil)
				mockProgress.EXPECT().Start(fmt.Sprintf(fmtAppInitStart, "myapp"))
				mockDeployer.EXPECT().
					DeployApp(gomock.Any()).Return(mockError)
				mockProgress.EXPECT().Stop(log.Serrorf(fmtAppInitFailed, "myapp"))
			},
		},
		"should return error from CreateApplication": {
			expectedError: mockError,
			mocking: func(t *testing.T, mockstore *mocks.Mockstore, mockWorkspace *mocks.MockwsAppManager,
				mockIdentityService *mocks.MockidentityService, mockDeployer *mocks.MockappDeployer,
				mockProgress *mocks.Mockprogress) {
				mockIdentityService.
					EXPECT().
					Get().
					Return(identity.Caller{
						Account: "12345",
					}, nil)
				mockstore.
					EXPECT().
					CreateApplication(gomock.Any()).
					Return(mockError)
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("myapp")).Return(nil)
				mockProgress.EXPECT().Start(fmt.Sprintf(fmtAppInitStart, "myapp"))
				mockDeployer.EXPECT().
					DeployApp(gomock.Any()).Return(nil)
				mockProgress.EXPECT().Stop(log.Ssuccessf(fmtAppInitComplete, "myapp"))
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockstore := mocks.NewMockstore(ctrl)
			mockWorkspace := mocks.NewMockwsAppManager(ctrl)
			mockIdentityService := mocks.NewMockidentityService(ctrl)
			mockDeployer := mocks.NewMockappDeployer(ctrl)
			mockProgress := mocks.NewMockprogress(ctrl)

			opts := &initAppOpts{
				initAppVars: initAppVars{
					AppName:    "myapp",
					DomainName: tc.inDomainName,
					ResourceTags: map[string]string{
						"owner": "boss",
					},
				},
				store:    mockstore,
				identity: mockIdentityService,
				cfn:      mockDeployer,
				ws:       mockWorkspace,
				prog:     mockProgress,
			}
			tc.mocking(t, mockstore, mockWorkspace, mockIdentityService, mockDeployer, mockProgress)

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.expectedError == nil {
				require.NoError(t, err)
			} else {
				require.True(t, errors.Is(err, tc.expectedError))
			}
		})
	}
}

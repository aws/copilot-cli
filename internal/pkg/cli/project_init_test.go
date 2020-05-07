// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestInitProjectOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inProjectName string
		expect        func(opts *initProjectOpts)

		wantedProjectName string
		wantedErr         string
	}{
		"errors if summary exists and differs from project flag": {
			inProjectName: "testname",
			expect: func(opts *initProjectOpts) {
				opts.ws.(*mocks.MockwsProjectManager).EXPECT().Summary().Return(&workspace.Summary{Application: "metrics"}, nil)
				opts.storeClient.(*mocks.MockstoreClient).EXPECT().ListApplications().Times(0)
			},
			wantedErr: "workspace already registered with metrics",
		},
		"use flag if there is no summary": {
			inProjectName: "metrics",
			expect: func(opts *initProjectOpts) {
				opts.ws.(*mocks.MockwsProjectManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.storeClient.(*mocks.MockstoreClient).EXPECT().ListApplications().Times(0)
			},
			wantedProjectName: "metrics",
		},
		"return error from new project name": {
			expect: func(opts *initProjectOpts) {
				opts.ws.(*mocks.MockwsProjectManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.storeClient.(*mocks.MockstoreClient).EXPECT().ListApplications().Return([]*config.Application{}, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("my error"))
				opts.prompt.(*mocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				opts.prompt.(*mocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "prompt get project name: my error",
		},
		"enter new project name if no existing projects": {
			expect: func(opts *initProjectOpts) {
				opts.ws.(*mocks.MockwsProjectManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.storeClient.(*mocks.MockstoreClient).EXPECT().ListApplications().Return([]*config.Application{}, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				opts.prompt.(*mocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedProjectName: "metrics",
		},
		"return error from project selection": {
			expect: func(opts *initProjectOpts) {
				opts.ws.(*mocks.MockwsProjectManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.storeClient.(*mocks.MockstoreClient).EXPECT().ListApplications().Return([]*config.Application{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("my error"))
			},
			wantedErr: "prompt select project name: my error",
		},
		"use existing projects": {
			expect: func(opts *initProjectOpts) {
				opts.ws.(*mocks.MockwsProjectManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.storeClient.(*mocks.MockstoreClient).EXPECT().ListApplications().Return([]*config.Application{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
			},
			wantedProjectName: "metrics",
		},
		"enter new project name if user opts out of selection": {
			expect: func(opts *initProjectOpts) {
				opts.ws.(*mocks.MockwsProjectManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.storeClient.(*mocks.MockstoreClient).EXPECT().ListApplications().Return([]*config.Application{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
				opts.prompt.(*mocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				opts.prompt.(*mocks.Mockprompter).EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
			},
			wantedProjectName: "metrics",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			opts := &initProjectOpts{
				initProjectVars: initProjectVars{
					ProjectName: tc.inProjectName,
				},
				storeClient: mocks.NewMockstoreClient(ctrl),
				ws:          mocks.NewMockwsProjectManager(ctrl),
				prompt:      mocks.NewMockprompter(ctrl),
			}
			tc.expect(opts)

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedErr != "" {
				require.EqualError(t, err, tc.wantedErr)
			} else {
				require.Nil(t, err)
				require.Equal(t, tc.wantedProjectName, opts.ProjectName)
			}
		})
	}
}

func TestInitProjectOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inProjectName  string
		inDomainName   string
		mockRoute53Svc func(m *mocks.MockdomainValidator)
		mockStore      func(m *mocks.MockstoreClient)

		wantedError string
	}{
		"skip everything": {
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {},
			mockStore:      func(m *mocks.MockstoreClient) {},

			wantedError: "",
		},
		"valid project name": {
			inProjectName:  "metrics",
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {},
			mockStore: func(m *mocks.MockstoreClient) {
				m.EXPECT().GetApplication("metrics").Return(nil, &config.ErrNoSuchApplication{
					ApplicationName: "metrics",
				})
			},

			wantedError: "",
		},
		"invalid project name": {
			inProjectName:  "123chicken",
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {},
			mockStore:      func(m *mocks.MockstoreClient) {},

			wantedError: "project name 123chicken is invalid: value must start with a letter and contain only lower-case letters, numbers, and hyphens",
		},
		"errors if project with different domain already exists": {
			inProjectName:  "metrics",
			inDomainName:   "badDomain.com",
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {},
			mockStore: func(m *mocks.MockstoreClient) {
				m.EXPECT().GetApplication("metrics").Return(&config.Application{
					Name:   "metrics",
					Domain: "domain.com",
				}, nil)
			},

			wantedError: "project named metrics already exists with a different domain name domain.com",
		},
		"errors if failed to get project": {
			inProjectName:  "metrics",
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {},
			mockStore: func(m *mocks.MockstoreClient) {
				m.EXPECT().GetApplication("metrics").Return(nil, errors.New("some error"))
			},

			wantedError: "get project metrics: some error",
		},
		"valid domain name": {
			inDomainName: "mockDomain.com",
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {
				m.EXPECT().DomainExists("mockDomain.com").Return(true, nil)
			},
			mockStore: func(m *mocks.MockstoreClient) {},

			wantedError: "",
		},
		"invalid domain name that does not exist": {
			inDomainName: "badMockDomain.com",
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {
				m.EXPECT().DomainExists("badMockDomain.com").Return(false, nil)
			},
			mockStore: func(m *mocks.MockstoreClient) {},

			wantedError: "no hosted zone found for badMockDomain.com",
		},
		"errors if failed to validate domain name": {
			inDomainName: "mockDomain.com",
			mockRoute53Svc: func(m *mocks.MockdomainValidator) {
				m.EXPECT().DomainExists("mockDomain.com").Return(false, errors.New("some error"))
			},
			mockStore: func(m *mocks.MockstoreClient) {},

			wantedError: "some error",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockRoute53Svc := mocks.NewMockdomainValidator(ctrl)
			mockStore := mocks.NewMockstoreClient(ctrl)
			tc.mockRoute53Svc(mockRoute53Svc)
			tc.mockStore(mockStore)
			opts := &initProjectOpts{
				route53Svc:  mockRoute53Svc,
				storeClient: mockStore,
				initProjectVars: initProjectVars{
					ProjectName: tc.inProjectName,
					DomainName:  tc.inDomainName,
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

func TestInitProjectOpts_Execute(t *testing.T) {
	mockError := fmt.Errorf("error")

	testCases := map[string]struct {
		inDomainName string

		expectedError error
		mocking       func(t *testing.T,
			mockStoreClient *mocks.MockstoreClient, mockWorkspace *mocks.MockwsProjectManager,
			mockIdentityService *mocks.MockidentityService, mockDeployer *mocks.MockprojectDeployer,
			mockProgress *mocks.Mockprogress)
	}{
		"with a successful call to add project": {
			inDomainName: "amazon.com",

			mocking: func(t *testing.T, mockStoreClient *mocks.MockstoreClient, mockWorkspace *mocks.MockwsProjectManager,
				mockIdentityService *mocks.MockidentityService, mockDeployer *mocks.MockprojectDeployer,
				mockProgress *mocks.Mockprogress) {
				mockIdentityService.
					EXPECT().
					Get().
					Return(identity.Caller{
						Account: "12345",
					}, nil)
				mockStoreClient.
					EXPECT().
					CreateApplication(&config.Application{
						AccountID: "12345",
						Name:      "project",
						Domain:    "amazon.com",
						Tags: map[string]string{
							"owner": "boss",
						},
					})
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("project")).Return(nil)
				mockProgress.EXPECT().Start(fmt.Sprintf(fmtDeployProjectStart, "project"))
				mockDeployer.EXPECT().
					DeployApp(&deploy.CreateAppInput{
						Name:       "project",
						AccountID:  "12345",
						DomainName: "amazon.com",
						AdditionalTags: map[string]string{
							"owner": "boss",
						},
					}).Return(nil)
				mockProgress.EXPECT().Stop(log.Ssuccessf(fmtDeployProjectComplete, "project"))
			},
		},
		"should return error from workspace.Create": {
			expectedError: mockError,
			mocking: func(t *testing.T, mockStoreClient *mocks.MockstoreClient, mockWorkspace *mocks.MockwsProjectManager,
				mockIdentityService *mocks.MockidentityService, mockDeployer *mocks.MockprojectDeployer,
				mockProgress *mocks.Mockprogress) {
				mockIdentityService.
					EXPECT().
					Get().
					Return(identity.Caller{
						Account: "12345",
					}, nil)
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("project")).
					Return(mockError)
			},
		},
		"with an error while deploying project": {
			expectedError: mockError,
			mocking: func(t *testing.T, mockStoreClient *mocks.MockstoreClient, mockWorkspace *mocks.MockwsProjectManager,
				mockIdentityService *mocks.MockidentityService, mockDeployer *mocks.MockprojectDeployer,
				mockProgress *mocks.Mockprogress) {
				mockIdentityService.
					EXPECT().
					Get().
					Return(identity.Caller{
						Account: "12345",
					}, nil)
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("project")).Return(nil)
				mockProgress.EXPECT().Start(fmt.Sprintf(fmtDeployProjectStart, "project"))
				mockDeployer.EXPECT().
					DeployApp(gomock.Any()).Return(mockError)
				mockProgress.EXPECT().Stop(log.Serrorf(fmtDeployProjectFailed, "project"))
			},
		},
		"should return error from CreateProject": {
			expectedError: mockError,
			mocking: func(t *testing.T, mockStoreClient *mocks.MockstoreClient, mockWorkspace *mocks.MockwsProjectManager,
				mockIdentityService *mocks.MockidentityService, mockDeployer *mocks.MockprojectDeployer,
				mockProgress *mocks.Mockprogress) {
				mockIdentityService.
					EXPECT().
					Get().
					Return(identity.Caller{
						Account: "12345",
					}, nil)
				mockStoreClient.
					EXPECT().
					CreateApplication(gomock.Any()).
					Return(mockError)
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("project")).Return(nil)
				mockProgress.EXPECT().Start(fmt.Sprintf(fmtDeployProjectStart, "project"))
				mockDeployer.EXPECT().
					DeployApp(gomock.Any()).Return(nil)
				mockProgress.EXPECT().Stop(log.Ssuccessf(fmtDeployProjectComplete, "project"))
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockStoreClient := mocks.NewMockstoreClient(ctrl)
			mockWorkspace := mocks.NewMockwsProjectManager(ctrl)
			mockIdentityService := mocks.NewMockidentityService(ctrl)
			mockDeployer := mocks.NewMockprojectDeployer(ctrl)
			mockProgress := mocks.NewMockprogress(ctrl)

			opts := &initProjectOpts{
				initProjectVars: initProjectVars{
					ProjectName: "project",
					DomainName:  tc.inDomainName,
					ResourceTags: map[string]string{
						"owner": "boss",
					},
				},
				storeClient: mockStoreClient,
				identity:    mockIdentityService,
				deployer:    mockDeployer,
				ws:          mockWorkspace,
				prog:        mockProgress,
			}
			tc.mocking(t, mockStoreClient, mockWorkspace, mockIdentityService, mockDeployer, mockProgress)

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

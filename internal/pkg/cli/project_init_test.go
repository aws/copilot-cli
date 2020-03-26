// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
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
				opts.ws.(*climocks.MockwsProjectManager).EXPECT().Summary().Return(&workspace.Summary{ProjectName: "metrics"}, nil)
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Times(0)
			},
			wantedErr: "workspace already registered with metrics",
		},
		"use flag if there is no summary": {
			inProjectName: "metrics",
			expect: func(opts *initProjectOpts) {
				opts.ws.(*climocks.MockwsProjectManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Times(0)
			},
			wantedProjectName: "metrics",
		},
		"return error from new project name": {
			expect: func(opts *initProjectOpts) {
				opts.ws.(*climocks.MockwsProjectManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Return([]*archer.Project{}, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("my error"))
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "prompt get project name: my error",
		},
		"enter new project name if no existing projects": {
			expect: func(opts *initProjectOpts) {
				opts.ws.(*climocks.MockwsProjectManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Return([]*archer.Project{}, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedProjectName: "metrics",
		},
		"return error from project selection": {
			expect: func(opts *initProjectOpts) {
				opts.ws.(*climocks.MockwsProjectManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Return([]*archer.Project{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("my error"))
			},
			wantedErr: "prompt select project name: my error",
		},
		"use existing projects": {
			expect: func(opts *initProjectOpts) {
				opts.ws.(*climocks.MockwsProjectManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Return([]*archer.Project{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
			},
			wantedProjectName: "metrics",
		},
		"enter new project name if user opts out of selection": {
			expect: func(opts *initProjectOpts) {
				opts.ws.(*climocks.MockwsProjectManager).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Return([]*archer.Project{
					{
						Name: "metrics",
					},
					{
						Name: "payments",
					},
				}, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
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
				projectStore: mocks.NewMockProjectStore(ctrl),
				ws:           climocks.NewMockwsProjectManager(ctrl),
				prompt:       climocks.NewMockprompter(ctrl),
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
		mockRoute53Svc func(m *climocks.MockdomainValidator)
		mockStore      func(m *mocks.MockProjectStore)

		wantedError string
	}{
		"skip everything": {
			mockRoute53Svc: func(m *climocks.MockdomainValidator) {},
			mockStore:      func(m *mocks.MockProjectStore) {},

			wantedError: "",
		},
		"valid project name": {
			inProjectName:  "metrics",
			mockRoute53Svc: func(m *climocks.MockdomainValidator) {},
			mockStore: func(m *mocks.MockProjectStore) {
				m.EXPECT().GetProject("metrics").Return(nil, &store.ErrNoSuchProject{
					ProjectName: "metrics",
				})
			},

			wantedError: "",
		},
		"invalid project name": {
			inProjectName:  "123chicken",
			mockRoute53Svc: func(m *climocks.MockdomainValidator) {},
			mockStore:      func(m *mocks.MockProjectStore) {},

			wantedError: "project name 123chicken is invalid: value must start with a letter and contain only lower-case letters, numbers, and hyphens",
		},
		"errors if project with different domain already exists": {
			inProjectName:  "metrics",
			inDomainName:   "badDomain.com",
			mockRoute53Svc: func(m *climocks.MockdomainValidator) {},
			mockStore: func(m *mocks.MockProjectStore) {
				m.EXPECT().GetProject("metrics").Return(&archer.Project{
					Name:   "metrics",
					Domain: "domain.com",
				}, nil)
			},

			wantedError: "project named metrics already exists with a different domain name domain.com",
		},
		"errors if failed to get project": {
			inProjectName:  "metrics",
			mockRoute53Svc: func(m *climocks.MockdomainValidator) {},
			mockStore: func(m *mocks.MockProjectStore) {
				m.EXPECT().GetProject("metrics").Return(nil, errors.New("some error"))
			},

			wantedError: "get project metrics: some error",
		},
		"valid domain name": {
			inDomainName: "mockDomain.com",
			mockRoute53Svc: func(m *climocks.MockdomainValidator) {
				m.EXPECT().DomainExists("mockDomain.com").Return(true, nil)
			},
			mockStore: func(m *mocks.MockProjectStore) {},

			wantedError: "",
		},
		"invalid domain name that does not exist": {
			inDomainName: "badMockDomain.com",
			mockRoute53Svc: func(m *climocks.MockdomainValidator) {
				m.EXPECT().DomainExists("badMockDomain.com").Return(false, nil)
			},
			mockStore: func(m *mocks.MockProjectStore) {},

			wantedError: "no hosted zone found for badMockDomain.com",
		},
		"errors if failed to validate domain name": {
			inDomainName: "mockDomain.com",
			mockRoute53Svc: func(m *climocks.MockdomainValidator) {
				m.EXPECT().DomainExists("mockDomain.com").Return(false, errors.New("some error"))
			},
			mockStore: func(m *mocks.MockProjectStore) {},

			wantedError: "some error",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockRoute53Svc := climocks.NewMockdomainValidator(ctrl)
			mockStore := mocks.NewMockProjectStore(ctrl)
			tc.mockRoute53Svc(mockRoute53Svc)
			tc.mockStore(mockStore)
			opts := &initProjectOpts{
				route53Svc:   mockRoute53Svc,
				projectStore: mockStore,
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
			mockProjectStore *mocks.MockProjectStore, mockWorkspace *climocks.MockwsProjectManager,
			mockIdentityService *climocks.MockidentityService, mockDeployer *climocks.MockprojectDeployer,
			mockProgress *climocks.Mockprogress)
	}{
		"with a successful call to add project": {
			inDomainName: "amazon.com",

			mocking: func(t *testing.T, mockProjectStore *mocks.MockProjectStore, mockWorkspace *climocks.MockwsProjectManager,
				mockIdentityService *climocks.MockidentityService, mockDeployer *climocks.MockprojectDeployer,
				mockProgress *climocks.Mockprogress) {
				mockIdentityService.
					EXPECT().
					Get().
					Return(identity.Caller{
						Account: "12345",
					}, nil)
				mockProjectStore.
					EXPECT().
					CreateProject(&archer.Project{
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
					DeployProject(&deploy.CreateProjectInput{
						Project:    "project",
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
			mocking: func(t *testing.T, mockProjectStore *mocks.MockProjectStore, mockWorkspace *climocks.MockwsProjectManager,
				mockIdentityService *climocks.MockidentityService, mockDeployer *climocks.MockprojectDeployer,
				mockProgress *climocks.Mockprogress) {
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
			mocking: func(t *testing.T, mockProjectStore *mocks.MockProjectStore, mockWorkspace *climocks.MockwsProjectManager,
				mockIdentityService *climocks.MockidentityService, mockDeployer *climocks.MockprojectDeployer,
				mockProgress *climocks.Mockprogress) {
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
					DeployProject(gomock.Any()).Return(mockError)
				mockProgress.EXPECT().Stop(log.Serrorf(fmtDeployProjectFailed, "project"))
			},
		},
		"should return error from CreateProject": {
			expectedError: mockError,
			mocking: func(t *testing.T, mockProjectStore *mocks.MockProjectStore, mockWorkspace *climocks.MockwsProjectManager,
				mockIdentityService *climocks.MockidentityService, mockDeployer *climocks.MockprojectDeployer,
				mockProgress *climocks.Mockprogress) {
				mockIdentityService.
					EXPECT().
					Get().
					Return(identity.Caller{
						Account: "12345",
					}, nil)
				mockProjectStore.
					EXPECT().
					CreateProject(gomock.Any()).
					Return(mockError)
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("project")).Return(nil)
				mockProgress.EXPECT().Start(fmt.Sprintf(fmtDeployProjectStart, "project"))
				mockDeployer.EXPECT().
					DeployProject(gomock.Any()).Return(nil)
				mockProgress.EXPECT().Stop(log.Ssuccessf(fmtDeployProjectComplete, "project"))
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockProjectStore := mocks.NewMockProjectStore(ctrl)
			mockWorkspace := climocks.NewMockwsProjectManager(ctrl)
			mockIdentityService := climocks.NewMockidentityService(ctrl)
			mockDeployer := climocks.NewMockprojectDeployer(ctrl)
			mockProgress := climocks.NewMockprogress(ctrl)

			opts := &initProjectOpts{
				initProjectVars: initProjectVars{
					ProjectName: "project",
					DomainName:  tc.inDomainName,
					ResourceTags: map[string]string{
						"owner": "boss",
					},
				},
				projectStore: mockProjectStore,
				identity:     mockIdentityService,
				deployer:     mockDeployer,
				ws:           mockWorkspace,
				prog:         mockProgress,
			}
			tc.mocking(t, mockProjectStore, mockWorkspace, mockIdentityService, mockDeployer, mockProgress)

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

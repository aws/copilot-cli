// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/aws/identity"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/deploy"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestInitProjectOpts_Ask(t *testing.T) {
	testCases := map[string]struct {
		inProjectName string
		expect        func(opts *InitProjectOpts)

		wantedProjectName string
		wantedErr         string
	}{
		"override flag from project name if summary exists": {
			inProjectName: "testname",
			expect: func(opts *InitProjectOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(&archer.WorkspaceSummary{ProjectName: "metrics"}, nil)
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Times(0)
			},
			wantedProjectName: "metrics",
		},
		"use flag if there is no summary": {
			inProjectName: "metrics",
			expect: func(opts *InitProjectOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Times(0)
			},
			wantedProjectName: "metrics",
		},
		"return error from new project name": {
			expect: func(opts *InitProjectOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Return([]*archer.Project{}, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return("", errors.New("my error"))
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: "prompt get project name: my error",
		},
		"enter new project name if no existing projects": {
			expect: func(opts *InitProjectOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
				opts.projectStore.(*mocks.MockProjectStore).EXPECT().ListProjects().Return([]*archer.Project{}, nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return("metrics", nil)
				opts.prompt.(*climocks.Mockprompter).EXPECT().Confirm(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				opts.prompt.(*climocks.Mockprompter).EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedProjectName: "metrics",
		},
		"return error from project selection": {
			expect: func(opts *InitProjectOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
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
			expect: func(opts *InitProjectOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
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
			expect: func(opts *InitProjectOpts) {
				opts.ws.(*mocks.MockWorkspace).EXPECT().Summary().Return(nil, errors.New("no existing workspace"))
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

			opts := &InitProjectOpts{
				ProjectName:  tc.inProjectName,
				projectStore: mocks.NewMockProjectStore(ctrl),
				ws:           mocks.NewMockWorkspace(ctrl),
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
			}
			require.Equal(t, tc.wantedProjectName, opts.ProjectName)
		})
	}
}

func TestInitProjectOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inProjectName string
		wantedError   error
	}{
		"valid project name": {
			inProjectName: "metrics",
		},
		"invalid project name": {
			inProjectName: "123chicken",
			wantedError:   errValueBadFormat,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &InitProjectOpts{
				ProjectName: tc.inProjectName,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			require.True(t, errors.Is(err, tc.wantedError))
		})
	}
}

func TestInitProjectOpts_Execute(t *testing.T) {
	mockError := fmt.Errorf("error")

	testCases := map[string]struct {
		expectedProject archer.Project
		expectedError   error
		mocking         func(t *testing.T,
			mockProjectStore *mocks.MockProjectStore, mockWorkspace *mocks.MockWorkspace,
			mockIdentityService *climocks.MockidentityService, mockDeployer *climocks.MockprojectDeployer,
			mockProgress *climocks.Mockprogress)
	}{
		"with a succesfull call to add project": {
			expectedProject: archer.Project{
				Name: "project",
			},
			mocking: func(t *testing.T, mockProjectStore *mocks.MockProjectStore, mockWorkspace *mocks.MockWorkspace,
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
					Do(func(project *archer.Project) {
						require.Equal(t, project.Name, "project")
					})
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("project")).Return(nil)
				mockProgress.EXPECT().Start(fmt.Sprintf(fmtDeployProjectStart, "project"))
				mockDeployer.EXPECT().
					DeployProject(&deploy.CreateProjectInput{
						Project:   "project",
						AccountID: "12345",
					}).Return(nil)
				mockProgress.EXPECT().Stop(log.Ssuccessf(fmtDeployProjectComplete, "project"))
			},
		},
		"with an error while deploying project": {
			expectedError: mockError,
			mocking: func(t *testing.T, mockProjectStore *mocks.MockProjectStore, mockWorkspace *mocks.MockWorkspace,
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
					Do(func(project *archer.Project) {
						require.Equal(t, project.Name, "project")
					})
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("project")).Return(nil)
				mockProgress.EXPECT().Start(fmt.Sprintf(fmtDeployProjectStart, "project"))
				mockDeployer.EXPECT().
					DeployProject(&deploy.CreateProjectInput{
						Project:   "project",
						AccountID: "12345",
					}).Return(mockError)
				mockProgress.EXPECT().Stop(log.Serrorf(fmtDeployProjectFailed, "project"))
			},
		},
		"should ignore ErrProjectAlreadyExists from CreateProject": {
			expectedProject: archer.Project{
				Name: "project",
			},
			mocking: func(t *testing.T, mockProjectStore *mocks.MockProjectStore, mockWorkspace *mocks.MockWorkspace,
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
					Do(func(project *archer.Project) {
						require.Equal(t, project.Name, "project")
					}).
					Return(&store.ErrProjectAlreadyExists{ProjectName: "project"})
				mockWorkspace.
					EXPECT().
					Create(gomock.Any())
				mockProgress.EXPECT().Start(fmt.Sprintf(fmtDeployProjectStart, "project"))
				mockDeployer.EXPECT().
					DeployProject(&deploy.CreateProjectInput{
						Project:   "project",
						AccountID: "12345",
					}).Return(nil)
				mockProgress.EXPECT().Stop(log.Ssuccessf(fmtDeployProjectComplete, "project"))
			},
		},

		"should return error from CreateProject": {
			expectedError: mockError,
			mocking: func(t *testing.T, mockProjectStore *mocks.MockProjectStore, mockWorkspace *mocks.MockWorkspace,
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
					Create(gomock.Any()).
					Times(0)
				mockProgress.EXPECT().Start(gomock.Any()).Times(0)
				mockDeployer.EXPECT().DeployProject(gomock.Any()).Times(0)
			},
		},

		"should return error from workspace.Create": {
			expectedError: mockError,
			mocking: func(t *testing.T, mockProjectStore *mocks.MockProjectStore, mockWorkspace *mocks.MockWorkspace,
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
					Return(nil)
				mockWorkspace.
					EXPECT().
					Create(gomock.Eq("project")).
					Return(mockError)
				mockProgress.EXPECT().Start(gomock.Any()).Times(0)
				mockDeployer.EXPECT().DeployProject(gomock.Any()).Times(0)
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockProjectStore := mocks.NewMockProjectStore(ctrl)
			mockWorkspace := mocks.NewMockWorkspace(ctrl)
			mockIdentityService := climocks.NewMockidentityService(ctrl)
			mockDeployer := climocks.NewMockprojectDeployer(ctrl)
			mockProgress := climocks.NewMockprogress(ctrl)

			opts := &InitProjectOpts{
				ProjectName:  "project",
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

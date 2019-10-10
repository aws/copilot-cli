// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/workspace"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestInit_Ask(t *testing.T) {
	var mockProjectStore *mocks.MockProjectStore
	var mockPrompter *climocks.Mockprompter

	testCases := map[string]struct {
		inputProject     string
		inputApp         string
		inputType        string
		setupMocks       func()
		wantedProject    string
		wantedApp        string
		wantedType       string
		existingProjects []string
	}{
		"with no flags set and no projects": {
			setupMocks: func() {
				gomock.InOrder(
					mockPrompter.EXPECT().
						Get(
							gomock.Eq("What would you like to call your project?"),
							gomock.Any(),
							gomock.Any()).
						Return("heartbeat", nil).
						Times(1),
					mockPrompter.EXPECT().
						SelectOne(
							gomock.Eq("What type of application do you want to make?"),
							gomock.Any(),
							gomock.Eq([]string{manifest.LoadBalancedWebApplication})).
						Return("Load Balanced Web App", nil).
						Times(1),
					mockPrompter.EXPECT().
						Get(
							gomock.Eq("What do you want to call this Load Balanced Web App?"),
							gomock.Any(),
							gomock.Any()).
						Return("api", nil).
						Times(1))
			},
			wantedProject: "heartbeat",
			wantedApp:     "api",
			wantedType:    "Load Balanced Web App",
		},
		"with no flags set and existing projects": {
			setupMocks: func() {
				gomock.InOrder(
					mockPrompter.EXPECT().Confirm(
						"Would you like to create a new app in one of your existing projects?",
						gomock.Any(),
						gomock.Any()).
						Return(true, nil),
					mockPrompter.EXPECT().
						SelectOne(
							gomock.Eq("Which one do you want to add a new application to?"),
							gomock.Any(),
							gomock.Eq([]string{"heartbeat"})).
						Return("heartbeat", nil).
						Times(1),
					mockPrompter.EXPECT().
						SelectOne(
							gomock.Eq("What type of application do you want to make?"),
							gomock.Any(),
							gomock.Eq([]string{manifest.LoadBalancedWebApplication})).
						Return("Load Balanced Web App", nil).
						Times(1),
					mockPrompter.EXPECT().
						Get(
							gomock.Eq("What do you want to call this Load Balanced Web App?"),
							gomock.Any(),
							gomock.Any()).
						Return("api", nil).
						Times(1))
			},
			existingProjects: []string{"heartbeat"},
			wantedProject:    "heartbeat",
			wantedApp:        "api",
			wantedType:       "Load Balanced Web App",
		},
		"with only project flag set": {
			setupMocks: func() {
				gomock.InOrder(
					mockPrompter.EXPECT().
						SelectOne(
							gomock.Eq("What type of application do you want to make?"),
							gomock.Any(),
							gomock.Eq([]string{manifest.LoadBalancedWebApplication})).
						Return("Load Balanced Web App", nil).
						Times(1),
					mockPrompter.EXPECT().
						Get(
							gomock.Eq("What do you want to call this Load Balanced Web App?"),
							gomock.Any(),
							gomock.Any()).
						Return("api", nil).
						Times(1))
			},
			inputProject:  "heartbeat",
			wantedProject: "heartbeat",
			wantedApp:     "api",
			wantedType:    "Load Balanced Web App",
		},
		"with project and app flag set": {
			setupMocks: func() {
				mockPrompter.EXPECT().
					SelectOne(
						gomock.Eq("What type of application do you want to make?"),
						gomock.Any(),
						gomock.Eq([]string{manifest.LoadBalancedWebApplication})).
					Return("Load Balanced Web App", nil).
					Times(1)
			},
			inputProject:  "heartbeat",
			inputApp:      "api",
			wantedProject: "heartbeat",
			wantedApp:     "api",
			wantedType:    "Load Balanced Web App",
		},
		"with project, app and template flag set": {
			setupMocks:    func() {},
			inputProject:  "heartbeat",
			inputApp:      "api",
			inputType:     "Load Balanced Web App",
			wantedProject: "heartbeat",
			wantedApp:     "api",
			wantedType:    "Load Balanced Web App",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockProjectStore = mocks.NewMockProjectStore(ctrl)
			mockPrompter = climocks.NewMockprompter(ctrl)

			app := &InitAppOpts{
				Project:          tc.inputProject,
				AppName:          tc.inputApp,
				AppType:          tc.inputType,
				prompter:         mockPrompter,
				projStore:        mockProjectStore,
				existingProjects: tc.existingProjects,
			}
			tc.setupMocks()

			err := app.Ask()

			require.NoError(t, err)
			require.Equal(t, tc.wantedProject, app.Project, "expected project names to match")
			require.Equal(t, tc.wantedApp, app.AppName, "expected app names to match")
			require.Equal(t, tc.wantedType, app.AppType, "expected template names to match")
		})
	}
}

func TestInit_Prepare(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockProjectStore := mocks.NewMockProjectStore(ctrl)
	mockWorkspace := mocks.NewMockWorkspace(ctrl)
	defer ctrl.Finish()

	testCases := map[string]struct {
		inputOpts              InitAppOpts
		mocking                func()
		wantedExistingProjects []string
		wantedProject          string
	}{
		"with no project flag, empty workspace and existing projects": {
			inputOpts: InitAppOpts{
				AppName: "frontend",
			},
			wantedExistingProjects: []string{"project1", "project2"},
			mocking: func() {
				mockWorkspace.
					EXPECT().
					Summary().
					Return(nil, &workspace.ErrWorkspaceNotFound{})
				mockProjectStore.
					EXPECT().
					ListProjects().
					Return([]*archer.Project{
						&archer.Project{Name: "project1"},
						&archer.Project{Name: "project2"},
					}, nil)

			},
		},
		"with no project flag, empty workspace and error finding projects": {
			inputOpts: InitAppOpts{
				AppName: "frontend",
			},
			wantedExistingProjects: []string{},
			mocking: func() {
				mockWorkspace.
					EXPECT().
					Summary().
					Return(nil, &workspace.ErrWorkspaceNotFound{})

				mockProjectStore.
					EXPECT().
					ListProjects().
					Return(nil, fmt.Errorf("error loading projects"))

			},
		},
		"with no project flag and existing workspace": {
			inputOpts: InitAppOpts{
				AppName: "frontend",
			},
			wantedProject: "MyProject",
			mocking: func() {
				mockWorkspace.
					EXPECT().
					Summary().
					Return(&archer.WorkspaceSummary{
						ProjectName: "MyProject",
					}, nil)
				// No calls to project store should be made if we determine
				// the project from the workspace.
				mockProjectStore.
					EXPECT().
					ListProjects().
					Return(nil, fmt.Errorf("error loading projects")).
					Times(0)

			},
		},

		"with project flag": {
			inputOpts: InitAppOpts{
				AppName: "frontend",
				Project: "MyProject",
			},
			wantedProject: "MyProject",
			mocking: func() {
				mockWorkspace.
					EXPECT().
					Summary().
					Return(&archer.WorkspaceSummary{
						ProjectName: "MyOtherProject",
					}, nil).
					Times(0)
				// No calls to project store should be made if we determine
				// the project from the workspace.
				mockProjectStore.
					EXPECT().
					ListProjects().
					Return(nil, fmt.Errorf("error loading projects")).
					Times(0)

			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.mocking()
			tc.inputOpts.projStore = mockProjectStore
			tc.inputOpts.ws = mockWorkspace
			tc.inputOpts.Prepare()
			require.ElementsMatch(t, tc.wantedExistingProjects, tc.inputOpts.existingProjects)
			require.Equal(t, tc.wantedProject, tc.inputOpts.Project)
		})
	}
}

func TestInit_Validate(t *testing.T) {
	testCases := map[string]struct {
		inputOpts       InitAppOpts
		wantedErrPrefix string
	}{
		"with valid project and app": {
			inputOpts: InitAppOpts{
				AppName: "frontend",
				Project: "coolproject",
			},
		},
		"with invalid project name": {
			inputOpts: InitAppOpts{
				AppName: "coolapp",
				Project: "!!!!",
			},
			wantedErrPrefix: "project name !!!! is invalid",
		},
		"with invalid app name": {
			inputOpts: InitAppOpts{
				AppName: "!!!",
				Project: "coolproject",
			},
			wantedErrPrefix: "application name !!! is invalid",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.inputOpts.Validate()
			if tc.wantedErrPrefix != "" {
				require.Regexp(t, "^"+tc.wantedErrPrefix+".*", err.Error())
			} else {
				require.NoError(t, err, "There shouldn't have been an error")
			}
		})
	}
}

func TestInit_Execute(t *testing.T) {
	var mockProjectStore *mocks.MockProjectStore
	var mockEnvStore *mocks.MockEnvironmentStore
	var mockWorkspace *mocks.MockWorkspace
	var mockDeployer *mocks.MockEnvironmentDeployer
	var mockProgress *climocks.Mockprogress
	var mockPrompter *climocks.Mockprompter

	mockError := fmt.Errorf("error")

	testCases := map[string]struct {
		inputOpts  InitAppOpts
		setupMocks func()
		want       error
	}{
		"should return error if project error is not ProjectAlreadyExist": {
			inputOpts: InitAppOpts{
				AppName:               "frontend",
				Project:               "project1",
				AppType:               "Load Balanced Web App",
				ShouldDeploy:          false,
				promptForShouldDeploy: false,
				existingProjects:      []string{},
			},
			setupMocks: func() {
				gomock.InOrder(
					mockProjectStore.
						EXPECT().
						CreateProject(gomock.Any()).
						Return(mockError),
				)
			},
			want: mockError,
		},
		"should continue if project already exists": {
			inputOpts: InitAppOpts{
				AppName:               "frontend",
				Project:               "project1",
				AppType:               "Load Balanced Web App",
				ShouldDeploy:          false,
				promptForShouldDeploy: false,
				existingProjects:      []string{},
			},
			setupMocks: func() {
				gomock.InOrder(
					mockProjectStore.
						EXPECT().
						CreateProject(gomock.Any()).
						Return(&store.ErrProjectAlreadyExists{
							ProjectName: "project1",
						}),
					mockWorkspace.
						EXPECT().
						Create(gomock.Eq("project1")),
					mockWorkspace.
						EXPECT().
						WriteManifest(gomock.Any(), "frontend").Return("/frontend", nil),
				)
			},
			want: nil,
		},
		"prompt for should deploy": {
			inputOpts: InitAppOpts{
				Project:               "project3",
				AppName:               "frontend",
				AppType:               "Load Balanced Web App",
				ShouldDeploy:          false,
				promptForShouldDeploy: true,
				existingProjects:      []string{"project1", "project2"},
			},
			setupMocks: func() {
				gomock.InOrder(
					mockProjectStore.
						EXPECT().
						CreateProject(gomock.Eq(&archer.Project{Name: "project3"})).
						Return(nil).
						Times(1),
					mockWorkspace.
						EXPECT().
						Create(gomock.Eq("project3")),
					mockWorkspace.
						EXPECT().
						WriteManifest(gomock.Any(), "frontend").
						Return("/frontend", nil),
					mockPrompter.EXPECT().
						Confirm("Would you like to deploy a staging environment?",
							gomock.Any()).
						Return(false, nil).
						Times(1))
			},
			want: nil,
		},
		"should echo error returned from call to workspace.Create": {
			inputOpts: InitAppOpts{
				AppType:               "Load Balanced Web App",
				Project:               "project3",
				AppName:               "frontend",
				ShouldDeploy:          false,
				promptForShouldDeploy: false,
				existingProjects:      []string{"project1", "project2"},
			},
			setupMocks: func() {
				gomock.InOrder(
					mockProjectStore.
						EXPECT().
						CreateProject(gomock.Eq(&archer.Project{Name: "project3"})).
						Return(nil).
						Times(1),
					mockWorkspace.
						EXPECT().
						Create(gomock.Eq("project3")).
						Return(mockError),
					mockWorkspace.
						EXPECT().
						WriteManifest(gomock.Any(), "frontend").
						Times(0))
			},
			want: mockError,
		},
		"should echo error returned from call to envDeployer.DeployEnvironment": {
			inputOpts: InitAppOpts{
				AppName:               "frontend",
				Project:               "project3",
				AppType:               "Load Balanced Web App",
				ShouldDeploy:          true,
				promptForShouldDeploy: false,
				existingProjects:      []string{"project1", "project2"},
			},
			setupMocks: func() {
				gomock.InOrder(
					mockProjectStore.
						EXPECT().
						CreateProject(gomock.Eq(&archer.Project{Name: "project3"})).
						Return(nil).
						Times(1),
					mockWorkspace.
						EXPECT().
						Create(gomock.Eq("project3")).
						Return(nil).
						Times(1),
					mockWorkspace.
						EXPECT().
						WriteManifest(gomock.Any(), "frontend").
						Return("/frontend", nil).
						Times(1),
					mockProgress.EXPECT().Start(gomock.Any()),
					mockDeployer.EXPECT().
						DeployEnvironment(gomock.Eq(&archer.Environment{
							Project:            "project3",
							Name:               defaultEnvironmentName,
							PublicLoadBalancer: true,
						})).
						Return(mockError).
						Times(1),
					mockProgress.EXPECT().Stop(gomock.Any()),
				)
			},
			want: mockError,
		},
		"should echo error returned from call to envDeployer.Wait": {
			inputOpts: InitAppOpts{
				AppName:               "frontend",
				Project:               "project3",
				AppType:               "Load Balanced Web App",
				ShouldDeploy:          true,
				promptForShouldDeploy: false,
				existingProjects:      []string{"project1", "project2"},
			},
			setupMocks: func() {
				gomock.InOrder(
					mockProjectStore.
						EXPECT().
						CreateProject(gomock.Eq(&archer.Project{Name: "project3"})).
						Return(nil).
						Times(1),
					mockWorkspace.
						EXPECT().
						Create(gomock.Eq("project3")).
						Return(nil).
						Times(1),
					mockWorkspace.
						EXPECT().
						WriteManifest(gomock.Any(), "frontend").
						Return("/frontend", nil).
						Times(1),
					mockProgress.EXPECT().Start(gomock.Any()),
					mockDeployer.EXPECT().
						DeployEnvironment(gomock.Eq(&archer.Environment{
							Project:            "project3",
							Name:               defaultEnvironmentName,
							PublicLoadBalancer: true,
						})).
						Return(nil).
						Times(1),
					mockProgress.EXPECT().Stop(gomock.Any()),
					mockProgress.EXPECT().Start(gomock.Any()),
					mockDeployer.EXPECT().
						WaitForEnvironmentCreation(gomock.Eq(&archer.Environment{
							Project:            "project3",
							Name:               defaultEnvironmentName,
							PublicLoadBalancer: true,
						})).
						Return(mockError).
						Times(1),
					mockProgress.EXPECT().Stop(gomock.Any()),
				)
			},
			want: mockError,
		},
		"should echo error returned from call to envStore.Create": {
			inputOpts: InitAppOpts{
				AppName:               "frontend",
				Project:               "project3",
				AppType:               "Load Balanced Web App",
				ShouldDeploy:          true,
				promptForShouldDeploy: false,
				existingProjects:      []string{"project1", "project2"},
			},
			setupMocks: func() {
				gomock.InOrder(
					mockProjectStore.
						EXPECT().
						CreateProject(gomock.Eq(&archer.Project{Name: "project3"})).
						Return(nil).
						Times(1),
					mockWorkspace.
						EXPECT().
						Create(gomock.Eq("project3")).
						Return(nil).
						Times(1),
					mockWorkspace.
						EXPECT().
						WriteManifest(gomock.Any(), "frontend").
						Return("/frontend", nil).
						Times(1),
					mockProgress.EXPECT().Start(gomock.Any()),
					mockDeployer.EXPECT().
						DeployEnvironment(gomock.Eq(&archer.Environment{
							Project:            "project3",
							Name:               defaultEnvironmentName,
							PublicLoadBalancer: true,
						})).
						Return(nil).
						Times(1),
					mockProgress.EXPECT().Stop(gomock.Any()),
					mockProgress.EXPECT().Start(gomock.Any()),
					mockDeployer.EXPECT().
						WaitForEnvironmentCreation(gomock.Eq(&archer.Environment{
							Project:            "project3",
							Name:               defaultEnvironmentName,
							PublicLoadBalancer: true,
						})).
						Return(nil).
						Times(1),
					mockEnvStore.
						EXPECT().
						CreateEnvironment(gomock.Any()).
						Return(mockError).
						Times(1),
					mockProgress.EXPECT().Stop(gomock.Any()),
				)
			},
			want: mockError,
		},
		"should create a new test environment": {
			inputOpts: InitAppOpts{
				AppName:               "frontend",
				Project:               "project3",
				AppType:               "Load Balanced Web App",
				ShouldDeploy:          true,
				promptForShouldDeploy: false,
				existingProjects:      []string{"project1", "project2"},
			},
			setupMocks: func() {
				gomock.InOrder(
					mockProjectStore.
						EXPECT().
						CreateProject(gomock.Eq(&archer.Project{Name: "project3"})).
						Return(nil).
						Times(1),
					mockWorkspace.
						EXPECT().
						Create(gomock.Eq("project3")).
						Return(nil).
						Times(1),
					mockWorkspace.
						EXPECT().
						WriteManifest(gomock.Any(), "frontend").
						Return("/frontend", nil).
						Times(1),
					mockProgress.EXPECT().Start(gomock.Any()),
					mockDeployer.EXPECT().
						DeployEnvironment(gomock.Eq(&archer.Environment{
							Project:            "project3",
							Name:               defaultEnvironmentName,
							PublicLoadBalancer: true,
						})).
						Return(nil).
						Times(1),
					mockProgress.EXPECT().Stop(gomock.Any()),
					mockProgress.EXPECT().Start(gomock.Any()),
					mockDeployer.EXPECT().
						WaitForEnvironmentCreation(gomock.Eq(&archer.Environment{
							Project:            "project3",
							Name:               defaultEnvironmentName,
							PublicLoadBalancer: true,
						})).
						Return(nil).
						Times(1),
					mockEnvStore.
						EXPECT().
						CreateEnvironment(gomock.Any()).
						Return(nil).
						Times(1),
					mockProgress.EXPECT().Stop(gomock.Any()),
				)
			},
			want: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mockProjectStore = mocks.NewMockProjectStore(ctrl)
			mockEnvStore = mocks.NewMockEnvironmentStore(ctrl)
			mockWorkspace = mocks.NewMockWorkspace(ctrl)
			mockProgress = climocks.NewMockprogress(ctrl)
			mockDeployer = mocks.NewMockEnvironmentDeployer(ctrl)
			mockPrompter = climocks.NewMockprompter(ctrl)

			tc.inputOpts.projStore = mockProjectStore
			tc.inputOpts.envStore = mockEnvStore
			tc.inputOpts.prog = mockProgress
			tc.inputOpts.envDeployer = mockDeployer
			tc.inputOpts.ws = mockWorkspace
			tc.inputOpts.prompter = mockPrompter
			tc.setupMocks()

			got := tc.inputOpts.Execute()

			if tc.want != nil {
				t.Log(got)
				require.True(t, errors.Is(got, tc.want))
			} else {
				require.Nil(t, got)
			}
		})
	}
}

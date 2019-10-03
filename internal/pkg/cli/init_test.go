// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"testing"

	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/archer"
	climocks "github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/cli/mocks"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/manifest"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/store"
	"github.com/aws/PRIVATE-amazon-ecs-archer/internal/pkg/workspace"
	"github.com/aws/PRIVATE-amazon-ecs-archer/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestInit_Ask(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProjectStore := mocks.NewMockProjectStore(ctrl)
	mockPrompter := climocks.NewMockprompter(ctrl)

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
							gomock.Eq("What is your project's name?"),
							gomock.Eq("Applications under the same project share the same VPC and ECS Cluster and are discoverable via service discovery."),
							gomock.Any()).
						Return("heartbeat", nil).
						Times(1),
					mockPrompter.EXPECT().
						Get(
							gomock.Eq("What is your application's name?"),
							gomock.Eq("Collection of AWS services to achieve a business capability. Must be unique within a project."),
							gomock.Any()).
						Return("api", nil).
						Times(1),
					mockPrompter.EXPECT().
						SelectOne(
							gomock.Eq("Which template would you like to use?"),
							gomock.Eq("Pre-defined infrastructure templates."),
							gomock.Eq([]string{manifest.LoadBalancedWebApplication})).
						Return("Load Balanced Web App", nil).
						Times(1))
			},
			wantedProject: "heartbeat",
			wantedApp:     "api",
			wantedType:    "Load Balanced Web App",
		},
		"with no flags set and existing projects": {
			setupMocks: func() {
				gomock.InOrder(
					mockPrompter.EXPECT().
						SelectOne(
							gomock.Eq("Which project should we use?"),
							gomock.Eq("Choose a project to create a new application in. Applications in the same project share the same VPC, ECS Cluster and are discoverable via service discovery"),
							gomock.Eq([]string{"heartbeat"})).
						Return("heartbeat", nil).
						Times(1),
					mockPrompter.EXPECT().
						Get(
							gomock.Eq("What is your application's name?"),
							gomock.Eq("Collection of AWS services to achieve a business capability. Must be unique within a project."),
							gomock.Any()).
						Return("api", nil).
						Times(1),
					mockPrompter.EXPECT().
						SelectOne(
							gomock.Eq("Which template would you like to use?"),
							gomock.Eq("Pre-defined infrastructure templates."),
							gomock.Eq([]string{manifest.LoadBalancedWebApplication})).
						Return("Load Balanced Web App", nil).
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
						Get(
							gomock.Eq("What is your application's name?"),
							gomock.Eq("Collection of AWS services to achieve a business capability. Must be unique within a project."),
							gomock.Any()).
						Return("api", nil).
						Times(1),
					mockPrompter.EXPECT().
						SelectOne(
							gomock.Eq("Which template would you like to use?"),
							gomock.Eq("Pre-defined infrastructure templates."),
							gomock.Eq([]string{manifest.LoadBalancedWebApplication})).
						Return("Load Balanced Web App", nil).
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
						gomock.Eq("Which template would you like to use?"),
						gomock.Eq("Pre-defined infrastructure templates."),
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
			wantedErrPrefix: "project name invalid",
		},
		"with invalid app name": {
			inputOpts: InitAppOpts{
				AppName: "!!!",
				Project: "coolproject",
			},
			wantedErrPrefix: "application name invalid",
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

//TODO this test currently doesn't mock out the manifest writer.
// Since that part will change soon, I don't have tests for the
// manifest writer parts yet.
func TestInit_Execute(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockProjectStore := mocks.NewMockProjectStore(ctrl)
	mockEnvStore := mocks.NewMockEnvironmentStore(ctrl)
	mockWorkspace := mocks.NewMockWorkspace(ctrl)
	mockProgress := climocks.NewMockprogress(ctrl)
	mockDeployer := mocks.NewMockEnvironmentDeployer(ctrl)
	mockPrompter := climocks.NewMockprompter(ctrl)

	mockError := fmt.Errorf("error")

	testCases := map[string]struct {
		inputOpts  InitAppOpts
		setupMocks func()
		want       error
	}{
		"should not prompt to create test environment given project and environments": {
			inputOpts: InitAppOpts{
				AppName:          "frontend",
				Project:          "project1",
				AppType:          "Load Balanced Web App",
				existingProjects: []string{"project1", "project2"},
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
						WriteManifest(gomock.Any(), "frontend"),
					mockEnvStore.
						EXPECT().
						ListEnvironments(gomock.Eq("project1")).
						Return([]*archer.Environment{
							{Name: "test"},
						}, nil).
						Times(1))
			},
			want: nil,
		},
		"should not prompt to create test environment when customer skips it": {
			inputOpts: InitAppOpts{
				AppName:          "frontend",
				Project:          "project1",
				AppType:          "Load Balanced Web App",
				ShouldSkipDeploy: true,
				existingProjects: []string{},
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
						WriteManifest(gomock.Any(), "frontend"),
					mockEnvStore.
						EXPECT().
						ListEnvironments(gomock.Eq("project1")).
						Return([]*archer.Environment{
							{Name: "test"},
						}, nil).
						Times(0))
			},
			want: nil,
		},
		"should create a new project, workspace and manifest without a test environment": {
			inputOpts: InitAppOpts{
				Project:          "project3",
				AppName:          "frontend",
				AppType:          "Load Balanced Web App",
				existingProjects: []string{"project1", "project2"},
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
						WriteManifest(gomock.Any(), "frontend"),
					mockEnvStore.
						EXPECT().
						ListEnvironments(gomock.Eq("project3")).
						Return(make([]*archer.Environment, 0), nil).
						Times(1),
					mockPrompter.EXPECT().
						Confirm("Would you like to set up a test environment?",
							"You can deploy your app into your test environment.").
						Return(false, nil).
						Times(1))
			},
			want: nil,
		},
		"should echo error returned from call to CreateProject": {
			inputOpts: InitAppOpts{
				AppType:          "Load Balanced Web App",
				Project:          "project3",
				AppName:          "frontend",
				existingProjects: []string{"project1", "project2"},
			},
			setupMocks: func() {
				gomock.InOrder(
					mockProjectStore.
						EXPECT().
						CreateProject(gomock.Eq(&archer.Project{Name: "project3"})).
						Return(mockError),
					mockWorkspace.
						EXPECT().
						Create(gomock.Eq("project3")).
						Times(0),
					mockWorkspace.
						EXPECT().
						WriteManifest(gomock.Any(), "frontend").
						Times(0))
			},
			want: mockError,
		},
		"should echo error returned from call to workspace.Create": {
			inputOpts: InitAppOpts{
				AppType:          "Load Balanced Web App",
				Project:          "project3",
				AppName:          "frontend",
				existingProjects: []string{"project1", "project2"},
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
				AppName:          "frontend",
				Project:          "project3",
				AppType:          "Load Balanced Web App",
				existingProjects: []string{"project1", "project2"},
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
						Times(1),
					mockEnvStore.
						EXPECT().
						ListEnvironments(gomock.Eq("project3")).
						Return([]*archer.Environment{}, nil).
						Times(1),
					mockPrompter.EXPECT().
						Confirm("Would you like to set up a test environment?",
							"You can deploy your app into your test environment.").
						Return(true, nil).
						Times(1),
					mockProgress.EXPECT().Start("Preparing deployment...").Times(1),
					mockDeployer.EXPECT().
						DeployEnvironment(gomock.Eq(&archer.Environment{
							Project:            "project3",
							Name:               defaultEnvironmentName,
							PublicLoadBalancer: true,
						})).
						Return(mockError).
						Times(1),
					mockProgress.EXPECT().Stop("Error!").Times(1))
			},
			want: mockError,
		},
		"should echo error returned from call to envDeployer.Wait": {
			inputOpts: InitAppOpts{
				AppName:          "frontend",
				Project:          "project3",
				AppType:          "Load Balanced Web App",
				existingProjects: []string{"project1", "project2"},
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
						Times(1),
					mockEnvStore.
						EXPECT().
						ListEnvironments(gomock.Eq("project3")).
						Return([]*archer.Environment{}, nil).
						Times(1),
					mockPrompter.EXPECT().
						Confirm("Would you like to set up a test environment?",
							"You can deploy your app into your test environment.").
						Return(true, nil).
						Times(1),
					mockProgress.EXPECT().Start("Preparing deployment...").Times(1),
					mockDeployer.EXPECT().
						DeployEnvironment(gomock.Eq(&archer.Environment{
							Project:            "project3",
							Name:               defaultEnvironmentName,
							PublicLoadBalancer: true,
						})).
						Return(nil).
						Times(1),
					mockProgress.EXPECT().Stop("Done!").Times(1),
					mockProgress.EXPECT().Start("Deploying env...").Times(1),
					mockDeployer.EXPECT().
						WaitForEnvironmentCreation(gomock.Eq(&archer.Environment{
							Project:            "project3",
							Name:               defaultEnvironmentName,
							PublicLoadBalancer: true,
						})).
						Return(mockError).
						Times(1),
					mockProgress.EXPECT().Stop("Error!").Times(1))
			},
			want: mockError,
		},
		"should echo error returned from call to envStore.Create": {
			inputOpts: InitAppOpts{
				AppName:          "frontend",
				Project:          "project3",
				AppType:          "Load Balanced Web App",
				existingProjects: []string{"project1", "project2"},
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
						Times(1),
					mockEnvStore.
						EXPECT().
						ListEnvironments(gomock.Eq("project3")).
						Return([]*archer.Environment{}, nil).
						Times(1),
					mockPrompter.EXPECT().
						Confirm("Would you like to set up a test environment?",
							"You can deploy your app into your test environment.").
						Return(true, nil).
						Times(1),
					mockProgress.EXPECT().Start("Preparing deployment...").Times(1),
					mockDeployer.EXPECT().
						DeployEnvironment(gomock.Eq(&archer.Environment{
							Project:            "project3",
							Name:               defaultEnvironmentName,
							PublicLoadBalancer: true,
						})).
						Return(nil).
						Times(1),
					mockProgress.EXPECT().Stop("Done!").Times(1),
					mockProgress.EXPECT().Start("Deploying env...").Times(1),
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
					mockProgress.EXPECT().Stop("Error!").Times(1),
				)
			},
			want: mockError,
		},
		"should create a new test environment": {
			inputOpts: InitAppOpts{
				AppName:          "frontend",
				Project:          "project3",
				AppType:          "Load Balanced Web App",
				existingProjects: []string{"project1", "project2"},
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
						Times(1),
					mockEnvStore.
						EXPECT().
						ListEnvironments(gomock.Eq("project3")).
						Return([]*archer.Environment{}, nil).
						Times(1),
					mockPrompter.EXPECT().
						Confirm("Would you like to set up a test environment?",
							"You can deploy your app into your test environment.").
						Return(true, nil).
						Times(1),
					mockProgress.EXPECT().Start("Preparing deployment...").Times(1),
					mockDeployer.EXPECT().
						DeployEnvironment(gomock.Eq(&archer.Environment{
							Project:            "project3",
							Name:               defaultEnvironmentName,
							PublicLoadBalancer: true,
						})).
						Return(nil).
						Times(1),
					mockProgress.EXPECT().Stop("Done!").Times(1),
					mockProgress.EXPECT().Start("Deploying env...").Times(1),
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
					mockProgress.EXPECT().Stop("Done!").Times(1),
				)
			},
			want: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			tc.inputOpts.projStore = mockProjectStore
			tc.inputOpts.envStore = mockEnvStore
			tc.inputOpts.prog = mockProgress
			tc.inputOpts.envDeployer = mockDeployer
			tc.inputOpts.ws = mockWorkspace
			tc.inputOpts.prompter = mockPrompter
			tc.setupMocks()

			got := tc.inputOpts.Execute()

			require.Equal(t, tc.want, got)
		})
	}
}

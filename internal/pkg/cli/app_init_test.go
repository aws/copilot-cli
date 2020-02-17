// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer/mocks"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestAppInitOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppType        string
		inAppName        string
		inDockerfilePath string
		inProjectName    string

		mockFileSystem func(mockFS afero.Fs)
		wantedErr      error
	}{
		"invalid app type": {
			inProjectName: "phonetool",
			inAppType:     "TestAppType",
			wantedErr:     errors.New(`invalid app type TestAppType: must be one of "Load Balanced Web App"`),
		},
		"invalid app name": {
			inProjectName: "phonetool",
			inAppName:     "1234",
			wantedErr:     fmt.Errorf("application name 1234 is invalid: %s", errValueBadFormat),
		},
		"invalid dockerfile directory path": {
			inProjectName:    "phonetool",
			inDockerfilePath: "./hello/Dockerfile",
			wantedErr:        errors.New("open hello/Dockerfile: file does not exist"),
		},
		"invalid project name": {
			inProjectName: "",
			wantedErr:     errNoProjectInWorkspace,
		},
		"valid flags": {
			inAppName:        "frontend",
			inAppType:        "Load Balanced Web App",
			inDockerfilePath: "./hello/Dockerfile",
			inProjectName:    "phonetool",

			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("hello", 0755)
				afero.WriteFile(mockFS, "hello/Dockerfile", []byte("FROM nginx"), 0644)
			},
			wantedErr: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			opts := initAppOpts{
				initAppVars: initAppVars{
					AppType:        tc.inAppType,
					AppName:        tc.inAppName,
					DockerfilePath: tc.inDockerfilePath,
					GlobalOpts:     &GlobalOpts{projectName: tc.inProjectName},
				},
				fs: &afero.Afero{Fs: afero.NewMemMapFs()},
			}
			if tc.mockFileSystem != nil {
				tc.mockFileSystem(opts.fs)
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Nil(t, err)
			}
		})
	}
}
func TestAppInitOpts_Ask(t *testing.T) {
	const (
		wantedAppType        = manifest.LoadBalancedWebApplication
		wantedAppName        = "frontend"
		wantedDockerfilePath = "frontend/Dockerfile"
	)
	testCases := map[string]struct {
		inAppType        string
		inAppName        string
		inDockerfilePath string

		mockFileSystem func(mockFS afero.Fs)
		mockPrompt     func(m *climocks.Mockprompter)

		wantedErr error
	}{
		"prompt for app type": {
			inAppType:        "",
			inAppName:        wantedAppName,
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq("Which type of infrastructure pattern best represents your application?"), appInitAppTypeHelpPrompt, gomock.Eq(manifest.AppTypes)).
					Return(wantedAppType, nil)
			},
			wantedErr: nil,
		},
		"return an error if fail to get app type": {
			inAppType:        "",
			inAppName:        wantedAppName,
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Eq(manifest.AppTypes)).
					Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("failed to get type selection: some error"),
		},
		"prompt for app name": {
			inAppType:        wantedAppType,
			inAppName:        "",
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf("What do you want to name this %s?", wantedAppType)), gomock.Any(), gomock.Any()).
					Return(wantedAppName, nil)
			},
			wantedErr: nil,
		},
		"returns an error if fail to get application name": {
			inAppType:        wantedAppType,
			inAppName:        "",
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("failed to get application name: some error"),
		},
		"choose an existing Dockerfile": {
			inAppType:        wantedAppType,
			inAppName:        wantedAppName,
			inDockerfilePath: "",

			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("frontend", 0755)
				mockFS.MkdirAll("backend", 0755)

				afero.WriteFile(mockFS, "Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "frontend/Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "backend/Dockerfile", []byte("FROM nginx"), 0644)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq(fmt.Sprintf(fmtAppInitDockerfilePrompt, wantedAppName)), appInitDockerfileHelpPrompt, gomock.Eq(
					[]string{
						"./Dockerfile",
						"backend/Dockerfile",
						"frontend/Dockerfile",
					})).
					Return("frontend/Dockerfile", nil)
			},
			wantedErr: nil,
		},
		"returns an error if fail to find Dockerfiles": {
			inAppType:        wantedAppType,
			inAppName:        wantedAppName,
			inDockerfilePath: "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			wantedErr: fmt.Errorf("no Dockerfiles found within . or a sub-directory level below"),
		},
		"returns an error if fail to select Dockerfile": {
			inAppType:        wantedAppType,
			inAppName:        wantedAppName,
			inDockerfilePath: "",

			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("frontend", 0755)
				mockFS.MkdirAll("backend", 0755)

				afero.WriteFile(mockFS, "Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "frontend/Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "backend/Dockerfile", []byte("FROM nginx"), 0644)
			},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq(fmt.Sprintf(fmtAppInitDockerfilePrompt, wantedAppName)), gomock.Any(), gomock.Eq(
					[]string{
						"./Dockerfile",
						"backend/Dockerfile",
						"frontend/Dockerfile",
					})).
					Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("failed to select Dockerfile: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := climocks.NewMockprompter(ctrl)
			opts := &initAppOpts{
				initAppVars: initAppVars{
					AppType:        tc.inAppType,
					AppName:        tc.inAppName,
					DockerfilePath: tc.inDockerfilePath,
					GlobalOpts: &GlobalOpts{
						prompt: mockPrompt,
					},
				},
				fs: &afero.Afero{Fs: afero.NewMemMapFs()},
			}
			tc.mockFileSystem(opts.fs)
			tc.mockPrompt(mockPrompt)

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.Nil(t, err)
				require.Equal(t, wantedAppType, opts.AppType)
				require.Equal(t, wantedAppName, opts.AppName)
				require.Equal(t, wantedDockerfilePath, opts.DockerfilePath)
			}
		})
	}
}

func TestAppInitOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		inAppType        string
		inAppName        string
		inDockerfilePath string
		inProjectName    string
		mockDependencies func(*gomock.Controller, *initAppOpts)
		wantedErr        error
	}{
		"writes manifest, and creates repositories successfully": {
			inAppType:        manifest.LoadBalancedWebApplication,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockWriter := climocks.NewMockwsAppManifestWriter(ctrl)
				mockWriter.EXPECT().WriteAppManifest(gomock.Any(), opts.AppName).Return("/frontend/manifest.yml", nil)

				mockAppStore := mocks.NewMockApplicationStore(ctrl)
				mockAppStore.EXPECT().GetApplication("project", "frontend").Return(nil, &store.ErrNoSuchApplication{})
				mockAppStore.EXPECT().CreateApplication(gomock.Any()).
					Do(func(app *archer.Application) {
						require.Equal(t, &archer.Application{
							Name:    "frontend",
							Project: "project",
							Type:    manifest.LoadBalancedWebApplication,
						}, app)
					}).
					Return(nil)

				mockProjGetter := mocks.NewMockProjectGetter(ctrl)
				mockProjGetter.EXPECT().GetProject("project").Return(&archer.Project{
					Name:      "project",
					AccountID: "1234",
				}, nil)

				mockProjDeployer := climocks.NewMockprojectDeployer(ctrl)
				mockProjDeployer.EXPECT().AddAppToProject(&archer.Project{
					Name:      "project",
					AccountID: "1234",
				}, "frontend")

				mockProg := climocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(fmt.Sprintf(fmtAddAppToProjectStart, "frontend"))
				mockProg.EXPECT().Stop(log.Ssuccessf(fmtAddAppToProjectComplete, "frontend"))

				opts.ws = mockWriter
				opts.appStore = mockAppStore
				opts.projGetter = mockProjGetter
				opts.projDeployer = mockProjDeployer
				opts.prog = mockProg
			},
		},
		"project error": {
			inAppType:        manifest.LoadBalancedWebApplication,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockWriter := climocks.NewMockwsAppManifestWriter(ctrl)

				mockAppStore := mocks.NewMockApplicationStore(ctrl)
				mockAppStore.EXPECT().GetApplication("project", "frontend").Return(nil, &store.ErrNoSuchApplication{})

				mockProjGetter := mocks.NewMockProjectGetter(ctrl)
				mockProjGetter.EXPECT().GetProject(gomock.Any()).Return(nil, errors.New("some error"))

				opts.ws = mockWriter
				opts.appStore = mockAppStore
				opts.projGetter = mockProjGetter
			},
			wantedErr: errors.New("get project project: some error"),
		},
		"add app to project fails": {
			inAppType:        manifest.LoadBalancedWebApplication,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockWriter := climocks.NewMockwsAppManifestWriter(ctrl)
				mockWriter.EXPECT().WriteAppManifest(gomock.Any(), opts.AppName).Return("/frontend/manifest.yml", nil)

				mockAppStore := mocks.NewMockApplicationStore(ctrl)
				mockAppStore.EXPECT().GetApplication("project", "frontend").Return(nil, &store.ErrNoSuchApplication{})

				mockProjGetter := mocks.NewMockProjectGetter(ctrl)
				mockProjGetter.EXPECT().GetProject(gomock.Any()).Return(&archer.Project{
					Name:      "project",
					AccountID: "1234",
				}, nil)

				mockProg := climocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(fmt.Sprintf(fmtAddAppToProjectStart, "frontend"))
				mockProg.EXPECT().Stop(log.Serrorf(fmtAddAppToProjectFailed, "frontend"))

				mockProjDeployer := climocks.NewMockprojectDeployer(ctrl)
				mockProjDeployer.EXPECT().AddAppToProject(gomock.Any(), gomock.Any()).Return(errors.New("some error"))

				opts.ws = mockWriter
				opts.appStore = mockAppStore
				opts.projGetter = mockProjGetter
				opts.projDeployer = mockProjDeployer
				opts.prog = mockProg
			},

			wantedErr: errors.New("add app frontend to project project: some error"),
		},
		"app already exists": {
			inAppType:        manifest.LoadBalancedWebApplication,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockAppStore := mocks.NewMockApplicationStore(ctrl)
				mockAppStore.EXPECT().GetApplication("project", "frontend").Return(&archer.Application{}, nil)
				mockAppStore.EXPECT().CreateApplication(gomock.Any()).
					Return(nil).
					Times(0)

				opts.appStore = mockAppStore
			},

			wantedErr: fmt.Errorf("application frontend already exists under project project"),
		},

		"error calling app store": {
			inAppType:        manifest.LoadBalancedWebApplication,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockAppStore := mocks.NewMockApplicationStore(ctrl)
				mockAppStore.EXPECT().GetApplication("project", "frontend").Return(nil, fmt.Errorf("oops"))
				mockAppStore.EXPECT().CreateApplication(gomock.Any()).
					Return(nil).
					Times(0)

				opts.appStore = mockAppStore
			},

			wantedErr: fmt.Errorf("couldn't check if application frontend exists in project project: oops"),
		},

		"error saving app": {
			inAppType:        manifest.LoadBalancedWebApplication,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockWriter := climocks.NewMockwsAppManifestWriter(ctrl)
				mockWriter.EXPECT().WriteAppManifest(gomock.Any(), opts.AppName).Return("/frontend/manifest.yml", nil)

				mockAppStore := mocks.NewMockApplicationStore(ctrl)
				mockAppStore.EXPECT().GetApplication("project", "frontend").Return(nil, &store.ErrNoSuchApplication{})
				mockAppStore.EXPECT().CreateApplication(gomock.Any()).
					Return(fmt.Errorf("oops"))

				mockProjGetter := mocks.NewMockProjectGetter(ctrl)
				mockProjGetter.EXPECT().GetProject(gomock.Any()).Return(&archer.Project{}, nil)

				mockProjDeployer := climocks.NewMockprojectDeployer(ctrl)
				mockProjDeployer.EXPECT().AddAppToProject(gomock.Any(), gomock.Any()).Return(nil)

				mockProg := climocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(gomock.Any())
				mockProg.EXPECT().Stop(gomock.Any())

				opts.ws = mockWriter
				opts.appStore = mockAppStore
				opts.projGetter = mockProjGetter
				opts.projDeployer = mockProjDeployer
				opts.prog = mockProg
			},

			wantedErr: fmt.Errorf("saving application frontend: oops"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			opts := initAppOpts{
				initAppVars: initAppVars{
					AppType:        tc.inAppType,
					AppName:        tc.inAppName,
					DockerfilePath: tc.inDockerfilePath,
					GlobalOpts:     &GlobalOpts{projectName: tc.inProjectName},
				},
			}
			tc.mockDependencies(ctrl, &opts)
			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedErr == nil {
				require.Nil(t, err)
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}

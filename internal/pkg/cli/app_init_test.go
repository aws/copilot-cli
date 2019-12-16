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
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestAppInitOpts_Ask(t *testing.T) {
	const (
		wantedAppType        = manifest.LoadBalancedWebApplication
		wantedAppName        = "frontend"
		wantedDockerfilePath = "frontend"
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
				m.EXPECT().SelectOne(gomock.Eq("Which type of infrastructure pattern best represents your application?"), gomock.Any(), gomock.Eq(manifest.AppTypes)).
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
				m.EXPECT().SelectOne(gomock.Eq("Which type of infrastructure pattern best represents your application?"), gomock.Any(), gomock.Eq(manifest.AppTypes)).
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
				m.EXPECT().Get(gomock.Eq("What do you want to call this Load Balanced Web App?"), gomock.Any(), gomock.Any()).
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
				m.EXPECT().Get(gomock.Eq("What do you want to call this Load Balanced Web App?"), gomock.Any(), gomock.Any()).
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
				m.EXPECT().SelectOne(gomock.Eq("Which Dockerfile would you like to use for frontend app?"), gomock.Any(), gomock.Eq(
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
				m.EXPECT().SelectOne(gomock.Eq("Which Dockerfile would you like to use for frontend app?"), gomock.Any(), gomock.Eq(
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
			opts := &InitAppOpts{
				AppType:        tc.inAppType,
				AppName:        tc.inAppName,
				DockerfilePath: tc.inDockerfilePath,

				fs: &afero.Afero{Fs: afero.NewMemMapFs()},
				GlobalOpts: &GlobalOpts{
					prompt: mockPrompt,
				},
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
			inAppType: "TestAppType",
			wantedErr: errors.New(`invalid app type TestAppType: must be one of "Load Balanced Web App"`),
		},
		"invalid app name": {
			inAppName: "1234",
			wantedErr: fmt.Errorf("application name 1234 is invalid: %s", errValueBadFormat),
		},
		"invalid dockerfile directory path": {
			inDockerfilePath: "./hello/Dockerfile",
			wantedErr:        errors.New("open hello/Dockerfile: file does not exist"),
		},
		"invalid project name": {
			inProjectName: "",
			wantedErr:     errNoProjectInWorkspace,
		},
		"invalid dockerfile path with a directory path": {
			inAppName:        "frontend",
			inAppType:        "Load Balanced Web App",
			inDockerfilePath: "./hello",
			inProjectName:    "phonetool",
			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("hello", 0755)
			},
			wantedErr: errors.New("dockerfile path expected, got ./hello"),
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
			// GIVEN
			opts := InitAppOpts{
				AppType:        tc.inAppType,
				AppName:        tc.inAppName,
				DockerfilePath: tc.inDockerfilePath,
				fs:             &afero.Afero{Fs: afero.NewMemMapFs()},
				GlobalOpts:     &GlobalOpts{projectName: tc.inProjectName},
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

func TestAppInitOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		inAppType          string
		inAppName          string
		inDockerfilePath   string
		inProjectName      string
		wantedErr          error
		mockManifestWriter func(m *mocks.MockManifestIO)
		mockAppStore       func(m *mocks.MockApplicationStore)
		mockProjGetter     func(m *mocks.MockProjectGetter)
		mockProjDeployer   func(m *climocks.MockprojectDeployer)
		mockProgress       func(m *climocks.Mockprogress)
	}{
		"writes manifest, and creates repositories successfully": {
			inAppType:        manifest.LoadBalancedWebApplication,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockManifestWriter: func(m *mocks.MockManifestIO) {
				manifestFile := "/frontend-app.yml"
				m.EXPECT().AppManifestFileName("frontend").Return(manifestFile)
				m.EXPECT().WriteFile(gomock.Any(), manifestFile).Return("/frontend", nil)
			},
			mockAppStore: func(m *mocks.MockApplicationStore) {
				m.EXPECT().GetApplication("project", "frontend").Return(nil, &store.ErrNoSuchApplication{})
				m.EXPECT().CreateApplication(gomock.Any()).
					Do(func(app *archer.Application) {
						require.Equal(t, &archer.Application{
							Name:    "frontend",
							Project: "project",
							Type:    manifest.LoadBalancedWebApplication,
						}, app)
					}).
					Return(nil)
			},
			mockProjGetter: func(m *mocks.MockProjectGetter) {
				m.EXPECT().GetProject("project").Return(&archer.Project{
					Name:      "project",
					AccountID: "1234",
				}, nil)
			},
			mockProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddAppToProjectStart, "frontend"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddAppToProjectComplete, "frontend"))
			},
			mockProjDeployer: func(m *climocks.MockprojectDeployer) {
				m.EXPECT().AddAppToProject(&archer.Project{
					Name:      "project",
					AccountID: "1234",
				}, "frontend")
			},
		},
		"project error": {
			inAppType:        manifest.LoadBalancedWebApplication,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockManifestWriter: func(m *mocks.MockManifestIO) {
				manifestFile := "/frontend-app.yml"
				m.EXPECT().AppManifestFileName("frontend").Return(manifestFile)
				m.EXPECT().WriteFile(gomock.Any(), manifestFile).Return("/frontend", nil)
			},
			mockAppStore: func(m *mocks.MockApplicationStore) {
				m.EXPECT().GetApplication("project", "frontend").Return(nil, &store.ErrNoSuchApplication{})
			},
			mockProjGetter: func(m *mocks.MockProjectGetter) {
				m.EXPECT().GetProject(gomock.Any()).Return(nil, errors.New("some error"))
			},
			mockProgress:     func(m *climocks.Mockprogress) {},
			mockProjDeployer: func(m *climocks.MockprojectDeployer) {},
			wantedErr:        errors.New("get project project: some error"),
		},
		"add app to project fails": {
			inAppType:        manifest.LoadBalancedWebApplication,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockManifestWriter: func(m *mocks.MockManifestIO) {
				manifestFile := "/frontend-app.yml"
				m.EXPECT().AppManifestFileName("frontend").Return(manifestFile)
				m.EXPECT().WriteFile(gomock.Any(), manifestFile).Return("/frontend", nil)
			},
			mockAppStore: func(m *mocks.MockApplicationStore) {
				m.EXPECT().GetApplication("project", "frontend").Return(nil, &store.ErrNoSuchApplication{})
			},
			mockProjGetter: func(m *mocks.MockProjectGetter) {
				m.EXPECT().GetProject(gomock.Any()).Return(&archer.Project{
					Name:      "project",
					AccountID: "1234",
				}, nil)
			},
			mockProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddAppToProjectStart, "frontend"))
				m.EXPECT().Stop(log.Serrorf(fmtAddAppToProjectFailed, "frontend"))
			},
			mockProjDeployer: func(m *climocks.MockprojectDeployer) {
				m.EXPECT().AddAppToProject(gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			wantedErr: errors.New("add app frontend to project project: some error"),
		},
		"app already exists": {
			inAppType:        manifest.LoadBalancedWebApplication,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			wantedErr:        fmt.Errorf("application frontend already exists under project project"),
			mockManifestWriter: func(m *mocks.MockManifestIO) {
				manifestFile := "/frontend-app.yml"
				m.EXPECT().AppManifestFileName("frontend").Return(manifestFile).Times(0)
				m.EXPECT().WriteFile(gomock.Any(), manifestFile).Return("/frontend", nil).Times(0)
			},
			mockProgress:     func(m *climocks.Mockprogress) {},
			mockProjGetter:   func(m *mocks.MockProjectGetter) {},
			mockProjDeployer: func(m *climocks.MockprojectDeployer) {},

			mockAppStore: func(m *mocks.MockApplicationStore) {
				m.EXPECT().GetApplication("project", "frontend").Return(&archer.Application{}, nil)
				m.EXPECT().CreateApplication(gomock.Any()).
					Return(nil).
					Times(0)
			},
		},

		"error calling app store": {
			inAppType:        manifest.LoadBalancedWebApplication,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			wantedErr:        fmt.Errorf("couldn't check if application frontend exists in project project: oops"),
			mockManifestWriter: func(m *mocks.MockManifestIO) {
				manifestFile := "/frontend-app.yml"
				m.EXPECT().AppManifestFileName("frontend").Return(manifestFile).Times(0)
				m.EXPECT().WriteFile(gomock.Any(), manifestFile).Return("/frontend", nil).Times(0)
			},
			mockProgress:     func(m *climocks.Mockprogress) {},
			mockProjGetter:   func(m *mocks.MockProjectGetter) {},
			mockProjDeployer: func(m *climocks.MockprojectDeployer) {},
			mockAppStore: func(m *mocks.MockApplicationStore) {
				m.EXPECT().GetApplication("project", "frontend").Return(nil, fmt.Errorf("oops"))
				m.EXPECT().CreateApplication(gomock.Any()).
					Return(nil).
					Times(0)
			},
		},

		"error saving app": {
			inAppType:        manifest.LoadBalancedWebApplication,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			wantedErr:        fmt.Errorf("saving application frontend: oops"),
			mockManifestWriter: func(m *mocks.MockManifestIO) {
				manifestFile := "/frontend-app.yml"
				m.EXPECT().AppManifestFileName("frontend").Return(manifestFile)
				m.EXPECT().WriteFile(gomock.Any(), manifestFile).Return("/frontend", nil)
			},
			mockProgress: func(m *climocks.Mockprogress) {
				m.EXPECT().Start(gomock.Any())
				m.EXPECT().Stop(gomock.Any())
			},
			mockProjGetter: func(m *mocks.MockProjectGetter) {
				m.EXPECT().GetProject(gomock.Any()).Return(&archer.Project{}, nil)
			},
			mockProjDeployer: func(m *climocks.MockprojectDeployer) {
				m.EXPECT().AddAppToProject(gomock.Any(), gomock.Any()).Return(nil)
			},
			mockAppStore: func(m *mocks.MockApplicationStore) {
				m.EXPECT().GetApplication("project", "frontend").Return(nil, &store.ErrNoSuchApplication{})
				m.EXPECT().CreateApplication(gomock.Any()).
					Return(fmt.Errorf("oops"))
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWriter := mocks.NewMockManifestIO(ctrl)
			mockAppStore := mocks.NewMockApplicationStore(ctrl)
			mockProjGetter := mocks.NewMockProjectGetter(ctrl)
			mockProjDeployer := climocks.NewMockprojectDeployer(ctrl)
			mockProg := climocks.NewMockprogress(ctrl)
			opts := InitAppOpts{
				AppType:        tc.inAppType,
				AppName:        tc.inAppName,
				DockerfilePath: tc.inDockerfilePath,
				manifestWriter: mockWriter,
				appStore:       mockAppStore,
				projGetter:     mockProjGetter,
				projDeployer:   mockProjDeployer,
				prog:           mockProg,

				GlobalOpts: &GlobalOpts{projectName: tc.inProjectName},
			}
			tc.mockManifestWriter(mockWriter)
			tc.mockAppStore(mockAppStore)
			tc.mockProjGetter(mockProjGetter)
			tc.mockProjDeployer(mockProjDeployer)
			tc.mockProgress(mockProg)
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

// Copyright 2019 Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/store"
	archerMocks "github.com/aws/amazon-ecs-cli-v2/mocks"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestAppInitOpts_Ask(t *testing.T) {
	const (
		wantedAppType        = manifest.LoadBalancedWebApplication
		wantedAppName        = "frontend"
		wantedDockerfilePath = "./frontend/Dockerfile"
	)
	testCases := map[string]struct {
		inAppType        string
		inAppName        string
		inDockerfilePath string

		mockFileSystem func(mockFS afero.Fs)
		mockPrompt     func(m *mocks.Mockprompter)
	}{
		"prompt for app type": {
			inAppType:        "",
			inAppName:        wantedAppName,
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq("Which type of infrastructure pattern best represents your application?"), gomock.Any(), gomock.Eq(manifest.AppTypes)).
					Return(wantedAppType, nil)
			},
		},
		"prompt for app name": {
			inAppType:        wantedAppType,
			inAppName:        "",
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq("What do you want to call this Load Balanced Web App?"), gomock.Any(), gomock.Any()).
					Return(wantedAppName, nil)
			},
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
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq("Which Dockerfile would you like to use for frontend app?"), gomock.Any(), gomock.Eq(
					[]string{
						"Dockerfile",
						"backend/Dockerfile",
						"frontend/Dockerfile",
						"Enter a custom path",
					})).
					Return(wantedDockerfilePath, nil)
			},
		},
		"choose a custom Dockerfile": {
			inAppType:        wantedAppType,
			inAppName:        wantedAppName,
			inDockerfilePath: "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq("Which Dockerfile would you like to use for frontend app?"), gomock.Any(), gomock.Eq(
					[]string{
						"Enter a custom path",
					})).
					Return("Enter a custom path", nil)
				m.EXPECT().Get(gomock.Eq("OK, what's the path to your Dockerfile?"), gomock.Any(), gomock.Any()).Return(wantedDockerfilePath, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mocks.NewMockprompter(ctrl)
			opts := &InitAppOpts{
				AppType:        tc.inAppType,
				AppName:        tc.inAppName,
				DockerfilePath: tc.inDockerfilePath,

				fs:     &afero.Afero{Fs: afero.NewMemMapFs()},
				prompt: mockPrompt,
			}
			tc.mockFileSystem(opts.fs)
			tc.mockPrompt(mockPrompt)

			// WHEN
			opts.Ask()

			// THEN
			require.Equal(t, wantedAppType, opts.AppType)
			require.Equal(t, wantedAppName, opts.AppName)
			require.Equal(t, wantedDockerfilePath, opts.DockerfilePath)
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
			wantedErr: errors.New("application name 1234 is invalid: value must be start with letter and container only letters, numbers, and hyphens"),
		},
		"invalid dockerfile path": {
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
			// GIVEN
			viper.Set(projectFlag, tc.inProjectName)
			defer viper.Set(projectFlag, "")
			opts := InitAppOpts{
				AppType:        tc.inAppType,
				AppName:        tc.inAppName,
				DockerfilePath: tc.inDockerfilePath,
				fs:             &afero.Afero{Fs: afero.NewMemMapFs()},
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
		mockManifestWriter func(m *archerMocks.MockManifestIO)
		mockAppStore       func(m *archerMocks.MockApplicationStore)
	}{
		"writes manifest and saves app when app doesn't exist": {
			inAppType:        manifest.LoadBalancedWebApplication,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockManifestWriter: func(m *archerMocks.MockManifestIO) {
				m.EXPECT().WriteManifest(gomock.Any(), "frontend").Return("/frontend", nil)
			},

			mockAppStore: func(m *archerMocks.MockApplicationStore) {
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
		},

		"app already exists": {
			inAppType:        manifest.LoadBalancedWebApplication,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			wantedErr:        fmt.Errorf("application frontend already exists under project project"),
			mockManifestWriter: func(m *archerMocks.MockManifestIO) {
				m.EXPECT().WriteManifest(gomock.Any(), "frontend").Return("/frontend", nil).Times(0)
			},

			mockAppStore: func(m *archerMocks.MockApplicationStore) {
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
			mockManifestWriter: func(m *archerMocks.MockManifestIO) {
				m.EXPECT().WriteManifest(gomock.Any(), "frontend").Return("/frontend", nil).Times(0)
			},

			mockAppStore: func(m *archerMocks.MockApplicationStore) {
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
			mockManifestWriter: func(m *archerMocks.MockManifestIO) {
				m.EXPECT().WriteManifest(gomock.Any(), "frontend").Return("/frontend", nil)
			},

			mockAppStore: func(m *archerMocks.MockApplicationStore) {
				m.EXPECT().GetApplication("project", "frontend").Return(nil, &store.ErrNoSuchApplication{})
				m.EXPECT().CreateApplication(gomock.Any()).
					Return(fmt.Errorf("oops"))
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			viper.Set(projectFlag, tc.inProjectName)
			defer viper.Set(projectFlag, "")
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWriter := archerMocks.NewMockManifestIO(ctrl)
			mockAppStore := archerMocks.NewMockApplicationStore(ctrl)
			opts := InitAppOpts{
				AppType:        tc.inAppType,
				AppName:        tc.inAppName,
				DockerfilePath: tc.inDockerfilePath,
				manifestWriter: mockWriter,
				appStore:       mockAppStore,
			}
			tc.mockManifestWriter(mockWriter)
			tc.mockAppStore(mockAppStore)
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

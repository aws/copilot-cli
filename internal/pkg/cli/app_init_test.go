// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer/mocks"
	climocks "github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
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
		inAppPort        uint16

		mockFileSystem func(mockFS afero.Fs)
		wantedErr      error
	}{
		"invalid app type": {
			inProjectName: "phonetool",
			inAppType:     "TestAppType",
			wantedErr:     errors.New(`invalid app type TestAppType: must be one of "Load Balanced Web Svc", "Backend Svc"`),
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
			inAppType:        "Load Balanced Web Svc",
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
					AppPort:        tc.inAppPort,
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
		wantedAppType        = manifest.LoadBalancedWebService
		wantedAppName        = "frontend"
		wantedDockerfilePath = "frontend/Dockerfile"
		wantedAppPort        = 80
	)
	testCases := map[string]struct {
		inAppType        string
		inAppName        string
		inDockerfilePath string
		inAppPort        uint16

		mockFileSystem func(mockFS afero.Fs)
		mockPrompt     func(m *climocks.Mockprompter)
		mockDockerfile func(m *climocks.MockdockerfileParser)

		wantedErr error
	}{
		"prompt for app type": {
			inAppType:        "",
			inAppName:        wantedAppName,
			inAppPort:        wantedAppPort,
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq("Which type of infrastructure pattern best represents your application?"), gomock.Any(), gomock.Eq(manifest.SvcTypes)).
					Return(wantedAppType, nil)
			},
			mockDockerfile: func(m *climocks.MockdockerfileParser) {},
			wantedErr:      nil,
		},
		"return an error if fail to get app type": {
			inAppType:        "",
			inAppName:        wantedAppName,
			inAppPort:        wantedAppPort,
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Eq(manifest.SvcTypes)).
					Return("", errors.New("some error"))
			},
			mockDockerfile: func(m *climocks.MockdockerfileParser) {},
			wantedErr:      fmt.Errorf("failed to get type selection: some error"),
		},
		"prompt for app name": {
			inAppType:        wantedAppType,
			inAppName:        "",
			inAppPort:        wantedAppPort,
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf("What do you want to name this %s?", wantedAppType)), gomock.Any(), gomock.Any()).
					Return(wantedAppName, nil)
			},
			mockDockerfile: func(m *climocks.MockdockerfileParser) {},
			wantedErr:      nil,
		},
		"returns an error if fail to get application name": {
			inAppType:        wantedAppType,
			inAppName:        "",
			inAppPort:        wantedAppPort,
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			mockDockerfile: func(m *climocks.MockdockerfileParser) {},
			wantedErr:      fmt.Errorf("failed to get application name: some error"),
		},
		"choose an existing Dockerfile": {
			inAppType:        wantedAppType,
			inAppName:        wantedAppName,
			inAppPort:        wantedAppPort,
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
			mockDockerfile: func(m *climocks.MockdockerfileParser) {},
			wantedErr:      nil,
		},
		"returns an error if fail to find Dockerfiles": {
			inAppType:        wantedAppType,
			inAppName:        wantedAppName,
			inAppPort:        wantedAppPort,
			inDockerfilePath: "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockDockerfile: func(m *climocks.MockdockerfileParser) {},
			wantedErr:      fmt.Errorf("no Dockerfiles found within . or a sub-directory level below"),
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
			mockDockerfile: func(m *climocks.MockdockerfileParser) {},
			wantedErr:      fmt.Errorf("failed to select Dockerfile: some error"),
		},
		"asks for port if not specified": {
			inAppType:        wantedAppType,
			inAppName:        wantedAppName,
			inDockerfilePath: wantedDockerfilePath,
			inAppPort:        0, //invalid port, default case

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(appInitAppPortPrompt), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(defaultAppPortString, nil)
			},
			mockDockerfile: func(m *climocks.MockdockerfileParser) {
				m.EXPECT().GetExposedPorts().Return([]uint16{}, errors.New("no expose"))
			},
			wantedErr: nil,
		},
		"errors if port not specified": {
			inAppType:        wantedAppType,
			inAppName:        wantedAppName,
			inDockerfilePath: wantedDockerfilePath,
			inAppPort:        0, //invalid port, default case

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(appInitAppPortPrompt), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			mockDockerfile: func(m *climocks.MockdockerfileParser) {
				m.EXPECT().GetExposedPorts().Return([]uint16{}, errors.New("expose error"))
			},
			wantedErr: fmt.Errorf("get port: some error"),
		},
		"errors if port out of range": {
			inAppType:        wantedAppType,
			inAppName:        wantedAppName,
			inDockerfilePath: wantedDockerfilePath,
			inAppPort:        0, //invalid port, default case

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *climocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(appInitAppPortPrompt), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("100000", errors.New("some error"))
			},
			mockDockerfile: func(m *climocks.MockdockerfileParser) {
				m.EXPECT().GetExposedPorts().Return([]uint16{}, errors.New("no expose"))
			},
			wantedErr: fmt.Errorf("get port: some error"),
		},
		"don't ask if dockerfile has port": {
			inAppType:        wantedAppType,
			inAppName:        wantedAppName,
			inDockerfilePath: wantedDockerfilePath,
			inAppPort:        0,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *climocks.Mockprompter) {
			},
			mockDockerfile: func(m *climocks.MockdockerfileParser) {
				m.EXPECT().GetExposedPorts().Return([]uint16{80}, nil)
			},
		},
		"don't use dockerfile port if flag specified": {
			inAppType:        wantedAppType,
			inAppName:        wantedAppName,
			inDockerfilePath: wantedDockerfilePath,
			inAppPort:        wantedAppPort,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *climocks.Mockprompter) {
			},
			mockDockerfile: func(m *climocks.MockdockerfileParser) {},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := climocks.NewMockprompter(ctrl)
			mockDockerfile := climocks.NewMockdockerfileParser(ctrl)
			opts := &initAppOpts{
				initAppVars: initAppVars{
					AppType:        tc.inAppType,
					AppName:        tc.inAppName,
					AppPort:        tc.inAppPort,
					DockerfilePath: tc.inDockerfilePath,
					GlobalOpts: &GlobalOpts{
						prompt: mockPrompt,
					},
				},
				fs:          &afero.Afero{Fs: afero.NewMemMapFs()},
				setupParser: func(o *initAppOpts) {},
				df:          mockDockerfile,
			}
			tc.mockFileSystem(opts.fs)
			tc.mockPrompt(mockPrompt)
			tc.mockDockerfile(mockDockerfile)

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
		inAppPort        uint16
		inAppType        string
		inAppName        string
		inDockerfilePath string
		inProjectName    string
		mockDependencies func(*gomock.Controller, *initAppOpts)
		wantedErr        error
	}{
		"writes load balanced web svc manifest, and creates repositories successfully": {
			inAppType:        manifest.LoadBalancedWebService,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			inAppPort:        80,

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockWriter := climocks.NewMockwsAppManifestWriter(ctrl)
				mockWriter.EXPECT().WriteServiceManifest(gomock.Any(), opts.AppName).Return("/frontend/manifest.yml", nil)

				mockAppStore := mocks.NewMockApplicationStore(ctrl)
				mockAppStore.EXPECT().ListServices("project").Return([]*archer.Application{}, nil)
				mockAppStore.EXPECT().CreateService(gomock.Any()).
					Do(func(app *archer.Application) {
						require.Equal(t, &archer.Application{
							Name:    "frontend",
							Project: "project",
							Type:    manifest.LoadBalancedWebService,
						}, app)
					}).
					Return(nil)

				mockProjGetter := mocks.NewMockProjectGetter(ctrl)
				mockProjGetter.EXPECT().GetApplication("project").Return(&archer.Project{
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
		"write manifest error": {
			inAppType:        manifest.LoadBalancedWebService,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			inAppPort:        80,

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockWriter := climocks.NewMockwsAppManifestWriter(ctrl)
				mockWriter.EXPECT().WriteServiceManifest(gomock.Any(), opts.AppName).Return("/frontend/manifest.yml", errors.New("some error"))

				mockAppStore := mocks.NewMockApplicationStore(ctrl)
				mockAppStore.EXPECT().ListServices("project").Return([]*archer.Application{}, nil)

				mockProjGetter := mocks.NewMockProjectGetter(ctrl)
				mockProjGetter.EXPECT().GetApplication("project").Return(&archer.Project{
					Name:      "project",
					AccountID: "1234",
				}, nil)

				mockProjDeployer := climocks.NewMockprojectDeployer(ctrl)

				mockProg := climocks.NewMockprogress(ctrl)

				opts.ws = mockWriter
				opts.appStore = mockAppStore
				opts.projGetter = mockProjGetter
				opts.projDeployer = mockProjDeployer
				opts.prog = mockProg
			},
			wantedErr: errors.New("some error"),
		},
		"project error": {
			inAppType:        manifest.LoadBalancedWebService,
			inProjectName:    "project",
			inAppName:        "frontend",
			inAppPort:        80,
			inDockerfilePath: "frontend/Dockerfile",

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockWriter := climocks.NewMockwsAppManifestWriter(ctrl)

				mockAppStore := mocks.NewMockApplicationStore(ctrl)

				mockProjGetter := mocks.NewMockProjectGetter(ctrl)
				mockProjGetter.EXPECT().GetApplication(gomock.Any()).Return(nil, errors.New("some error"))

				opts.ws = mockWriter
				opts.appStore = mockAppStore
				opts.projGetter = mockProjGetter
			},
			wantedErr: errors.New("get project project: some error"),
		},
		"add app to project fails": {
			inAppType:        manifest.LoadBalancedWebService,
			inProjectName:    "project",
			inAppName:        "frontend",
			inAppPort:        80,
			inDockerfilePath: "frontend/Dockerfile",

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockWriter := climocks.NewMockwsAppManifestWriter(ctrl)
				mockWriter.EXPECT().WriteServiceManifest(gomock.Any(), opts.AppName).Return("/frontend/manifest.yml", nil)

				mockAppStore := mocks.NewMockApplicationStore(ctrl)
				mockAppStore.EXPECT().ListServices("project").Return([]*archer.Application{}, nil)

				mockProjGetter := mocks.NewMockProjectGetter(ctrl)
				mockProjGetter.EXPECT().GetApplication(gomock.Any()).Return(&archer.Project{
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
		"error saving app": {
			inAppType:        manifest.LoadBalancedWebService,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockWriter := climocks.NewMockwsAppManifestWriter(ctrl)
				mockWriter.EXPECT().WriteServiceManifest(gomock.Any(), opts.AppName).Return("/frontend/manifest.yml", nil)

				mockAppStore := mocks.NewMockApplicationStore(ctrl)
				mockAppStore.EXPECT().ListServices("project").Return([]*archer.Application{}, nil)
				mockAppStore.EXPECT().CreateService(gomock.Any()).
					Return(fmt.Errorf("oops"))

				mockProjGetter := mocks.NewMockProjectGetter(ctrl)
				mockProjGetter.EXPECT().GetApplication(gomock.Any()).Return(&archer.Project{}, nil)

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
					AppPort:        tc.inAppPort,
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

func TestAppInitOpts_createLoadBalancedAppManifest(t *testing.T) {
	testCases := map[string]struct {
		inAppPort        uint16
		inAppName        string
		inDockerfilePath string
		inProjectName    string
		mockDependencies func(*gomock.Controller, *initAppOpts)
		wantedErr        error
		wantedPath       string
	}{
		"creates manifest with / as the path when there are no other apps": {
			inProjectName:    "project",
			inAppName:        "frontend",
			inAppPort:        80,
			inDockerfilePath: "frontend/Dockerfile",
			wantedPath:       "/",
			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockAppStore := mocks.NewMockApplicationStore(ctrl)
				mockAppStore.EXPECT().ListServices("project").Return([]*archer.Application{}, nil)
				opts.appStore = mockAppStore
			},
		},
		"creates manifest with / as the path when it's the only app": {
			inProjectName:    "project",
			inAppName:        "frontend",
			inAppPort:        80,
			inDockerfilePath: "frontend/Dockerfile",
			wantedPath:       "/",
			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockAppStore := mocks.NewMockApplicationStore(ctrl)
				mockAppStore.EXPECT().ListServices("project").Return([]*archer.Application{
					&archer.Application{
						Name: "frontend",
						Type: manifest.LoadBalancedWebService,
					},
				}, nil)
				opts.appStore = mockAppStore
			},
		},
		"creates manifest with / as the path when it's the only LBWebApp": {
			inProjectName:    "project",
			inAppName:        "frontend",
			inAppPort:        80,
			inDockerfilePath: "frontend/Dockerfile",
			wantedPath:       "/",
			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockAppStore := mocks.NewMockApplicationStore(ctrl)
				mockAppStore.EXPECT().ListServices("project").Return([]*archer.Application{
					&archer.Application{
						Name: "another-app",
						Type: "backend",
					},
				}, nil)
				opts.appStore = mockAppStore
			},
		},
		"creates manifest with {app name} as the path if there's another LBWebApp": {
			inProjectName:    "project",
			inAppName:        "frontend",
			inAppPort:        80,
			inDockerfilePath: "frontend/Dockerfile",
			wantedPath:       "frontend",
			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockAppStore := mocks.NewMockApplicationStore(ctrl)
				mockAppStore.EXPECT().ListServices("project").Return([]*archer.Application{
					&archer.Application{
						Name: "another-app",
						Type: manifest.LoadBalancedWebService,
					},
				}, nil)
				opts.appStore = mockAppStore
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			opts := initAppOpts{
				initAppVars: initAppVars{
					AppType:        manifest.LoadBalancedWebService,
					AppName:        tc.inAppName,
					AppPort:        tc.inAppPort,
					DockerfilePath: tc.inDockerfilePath,
					GlobalOpts:     &GlobalOpts{projectName: tc.inProjectName},
				},
			}
			tc.mockDependencies(ctrl, &opts)
			// WHEN
			manifest, err := opts.newLoadBalancedWebAppManifest()

			// THEN
			if tc.wantedErr == nil {
				require.Nil(t, err)
				require.Equal(t, tc.inAppName, manifest.Svc.Name)
				require.Equal(t, tc.inAppPort, manifest.Image.Port)
				require.Equal(t, tc.inDockerfilePath, manifest.Image.SvcImage.Build)
				require.Equal(t, tc.wantedPath, manifest.LoadBalancedWebSvcConfig.Path)
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}

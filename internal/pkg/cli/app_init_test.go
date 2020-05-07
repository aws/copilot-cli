// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
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
			wantedErr:     errors.New(`invalid app type TestAppType: must be one of "Load Balanced Web Service", "Backend Service"`),
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
			inAppType:        "Load Balanced Web Service",
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
		wantedAppType        = manifest.LoadBalancedWebServiceType
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
		mockPrompt     func(m *mocks.Mockprompter)
		mockDockerfile func(m *mocks.MockdockerfileParser)

		wantedErr error
	}{
		"prompt for app type": {
			inAppType:        "",
			inAppName:        wantedAppName,
			inAppPort:        wantedAppPort,
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq("Which type of infrastructure pattern best represents your application?"), gomock.Any(), gomock.Eq(manifest.ServiceTypes)).
					Return(wantedAppType, nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			wantedErr:      nil,
		},
		"return an error if fail to get app type": {
			inAppType:        "",
			inAppName:        wantedAppName,
			inAppPort:        wantedAppPort,
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Eq(manifest.ServiceTypes)).
					Return("", errors.New("some error"))
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			wantedErr:      fmt.Errorf("failed to get type selection: some error"),
		},
		"prompt for app name": {
			inAppType:        wantedAppType,
			inAppName:        "",
			inAppPort:        wantedAppPort,
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf("What do you want to name this %s?", wantedAppType)), gomock.Any(), gomock.Any()).
					Return(wantedAppName, nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			wantedErr:      nil,
		},
		"returns an error if fail to get application name": {
			inAppType:        wantedAppType,
			inAppName:        "",
			inAppPort:        wantedAppPort,
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
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
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq(fmt.Sprintf(fmtAppInitDockerfilePrompt, wantedAppName)), appInitDockerfileHelpPrompt, gomock.Eq(
					[]string{
						"./Dockerfile",
						"backend/Dockerfile",
						"frontend/Dockerfile",
					})).
					Return("frontend/Dockerfile", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			wantedErr:      nil,
		},
		"returns an error if fail to find Dockerfiles": {
			inAppType:        wantedAppType,
			inAppName:        wantedAppName,
			inAppPort:        wantedAppPort,
			inDockerfilePath: "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
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
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq(fmt.Sprintf(fmtAppInitDockerfilePrompt, wantedAppName)), gomock.Any(), gomock.Eq(
					[]string{
						"./Dockerfile",
						"backend/Dockerfile",
						"frontend/Dockerfile",
					})).
					Return("", errors.New("some error"))
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			wantedErr:      fmt.Errorf("failed to select Dockerfile: some error"),
		},
		"asks for port if not specified": {
			inAppType:        wantedAppType,
			inAppName:        wantedAppName,
			inDockerfilePath: wantedDockerfilePath,
			inAppPort:        0, //invalid port, default case

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(appInitAppPortPrompt), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(defaultAppPortString, nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
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
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(appInitAppPortPrompt), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
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
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(appInitAppPortPrompt), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("100000", errors.New("some error"))
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
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
			mockPrompt: func(m *mocks.Mockprompter) {
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetExposedPorts().Return([]uint16{80}, nil)
			},
		},
		"don't use dockerfile port if flag specified": {
			inAppType:        wantedAppType,
			inAppName:        wantedAppName,
			inDockerfilePath: wantedDockerfilePath,
			inAppPort:        wantedAppPort,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mocks.NewMockprompter(ctrl)
			mockDockerfile := mocks.NewMockdockerfileParser(ctrl)
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
		"writes Load Balanced Web Service manifest, and creates repositories successfully": {
			inAppType:        manifest.LoadBalancedWebServiceType,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			inAppPort:        80,

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockWriter := mocks.NewMockwsAppManifestWriter(ctrl)
				mockWriter.EXPECT().WriteServiceManifest(gomock.Any(), opts.AppName).Return("/frontend/manifest.yml", nil)

				mockStoreClient := mocks.NewMockstoreClient(ctrl)
				mockStoreClient.EXPECT().ListServices("project").Return([]*config.Service{}, nil)
				mockStoreClient.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Service) {
						require.Equal(t, &config.Service{
							Name: "frontend",
							App:  "project",
							Type: manifest.LoadBalancedWebServiceType,
						}, app)
					}).
					Return(nil)

				mockStoreClient.EXPECT().GetApplication("project").Return(&config.Application{
					Name:      "project",
					AccountID: "1234",
				}, nil)

				mockProjDeployer := mocks.NewMockprojectDeployer(ctrl)
				mockProjDeployer.EXPECT().AddServiceToApp(&config.Application{
					Name:      "project",
					AccountID: "1234",
				}, "frontend")

				mockProg := mocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(fmt.Sprintf(fmtAddAppToProjectStart, "frontend"))
				mockProg.EXPECT().Stop(log.Ssuccessf(fmtAddAppToProjectComplete, "frontend"))

				opts.ws = mockWriter
				opts.storeClient = mockStoreClient
				opts.projDeployer = mockProjDeployer
				opts.prog = mockProg
			},
		},
		"write manifest error": {
			inAppType:        manifest.LoadBalancedWebServiceType,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			inAppPort:        80,

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockWriter := mocks.NewMockwsAppManifestWriter(ctrl)
				mockWriter.EXPECT().WriteServiceManifest(gomock.Any(), opts.AppName).Return("/frontend/manifest.yml", errors.New("some error"))

				mockStoreClient := mocks.NewMockstoreClient(ctrl)
				mockStoreClient.EXPECT().ListServices("project").Return([]*config.Service{}, nil)

				mockStoreClient.EXPECT().GetApplication("project").Return(&config.Application{
					Name:      "project",
					AccountID: "1234",
				}, nil)

				mockProjDeployer := mocks.NewMockprojectDeployer(ctrl)

				mockProg := mocks.NewMockprogress(ctrl)

				opts.ws = mockWriter
				opts.storeClient = mockStoreClient
				opts.projDeployer = mockProjDeployer
				opts.prog = mockProg
			},
			wantedErr: errors.New("some error"),
		},
		"project error": {
			inAppType:        manifest.LoadBalancedWebServiceType,
			inProjectName:    "project",
			inAppName:        "frontend",
			inAppPort:        80,
			inDockerfilePath: "frontend/Dockerfile",

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockWriter := mocks.NewMockwsAppManifestWriter(ctrl)

				mockStoreClient := mocks.NewMockstoreClient(ctrl)

				mockStoreClient.EXPECT().GetApplication(gomock.Any()).Return(nil, errors.New("some error"))

				opts.ws = mockWriter
				opts.storeClient = mockStoreClient
			},
			wantedErr: errors.New("get project project: some error"),
		},
		"add app to project fails": {
			inAppType:        manifest.LoadBalancedWebServiceType,
			inProjectName:    "project",
			inAppName:        "frontend",
			inAppPort:        80,
			inDockerfilePath: "frontend/Dockerfile",

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockWriter := mocks.NewMockwsAppManifestWriter(ctrl)
				mockWriter.EXPECT().WriteServiceManifest(gomock.Any(), opts.AppName).Return("/frontend/manifest.yml", nil)

				mockStoreClient := mocks.NewMockstoreClient(ctrl)
				mockStoreClient.EXPECT().ListServices("project").Return([]*config.Service{}, nil)
				mockStoreClient.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{
					Name:      "project",
					AccountID: "1234",
				}, nil)

				mockProg := mocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(fmt.Sprintf(fmtAddAppToProjectStart, "frontend"))
				mockProg.EXPECT().Stop(log.Serrorf(fmtAddAppToProjectFailed, "frontend"))

				mockProjDeployer := mocks.NewMockprojectDeployer(ctrl)
				mockProjDeployer.EXPECT().AddServiceToApp(gomock.Any(), gomock.Any()).Return(errors.New("some error"))

				opts.ws = mockWriter
				opts.storeClient = mockStoreClient
				opts.projDeployer = mockProjDeployer
				opts.prog = mockProg
			},

			wantedErr: errors.New("add app frontend to project project: some error"),
		},
		"error saving app": {
			inAppType:        manifest.LoadBalancedWebServiceType,
			inProjectName:    "project",
			inAppName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockWriter := mocks.NewMockwsAppManifestWriter(ctrl)
				mockWriter.EXPECT().WriteServiceManifest(gomock.Any(), opts.AppName).Return("/frontend/manifest.yml", nil)

				mockStoreClient := mocks.NewMockstoreClient(ctrl)
				mockStoreClient.EXPECT().ListServices("project").Return([]*config.Service{}, nil)
				mockStoreClient.EXPECT().CreateService(gomock.Any()).
					Return(fmt.Errorf("oops"))
				mockStoreClient.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{}, nil)

				mockProjDeployer := mocks.NewMockprojectDeployer(ctrl)
				mockProjDeployer.EXPECT().AddServiceToApp(gomock.Any(), gomock.Any()).Return(nil)

				mockProg := mocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(gomock.Any())
				mockProg.EXPECT().Stop(gomock.Any())

				opts.ws = mockWriter
				opts.storeClient = mockStoreClient
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
				mockStoreClient := mocks.NewMockstoreClient(ctrl)
				mockStoreClient.EXPECT().ListServices("project").Return([]*config.Service{}, nil)
				opts.storeClient = mockStoreClient
			},
		},
		"creates manifest with / as the path when it's the only app": {
			inProjectName:    "project",
			inAppName:        "frontend",
			inAppPort:        80,
			inDockerfilePath: "frontend/Dockerfile",
			wantedPath:       "/",
			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockStoreClient := mocks.NewMockstoreClient(ctrl)
				mockStoreClient.EXPECT().ListServices("project").Return([]*config.Service{
					&config.Service{
						Name: "frontend",
						Type: manifest.LoadBalancedWebServiceType,
					},
				}, nil)
				opts.storeClient = mockStoreClient
			},
		},
		"creates manifest with / as the path when it's the only LBWebApp": {
			inProjectName:    "project",
			inAppName:        "frontend",
			inAppPort:        80,
			inDockerfilePath: "frontend/Dockerfile",
			wantedPath:       "/",
			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockStoreClient := mocks.NewMockstoreClient(ctrl)
				mockStoreClient.EXPECT().ListServices("project").Return([]*config.Service{
					&config.Service{
						Name: "another-app",
						Type: "backend",
					},
				}, nil)
				opts.storeClient = mockStoreClient
			},
		},
		"creates manifest with {app name} as the path if there's another LBWebApp": {
			inProjectName:    "project",
			inAppName:        "frontend",
			inAppPort:        80,
			inDockerfilePath: "frontend/Dockerfile",
			wantedPath:       "frontend",
			mockDependencies: func(ctrl *gomock.Controller, opts *initAppOpts) {
				mockStoreClient := mocks.NewMockstoreClient(ctrl)
				mockStoreClient.EXPECT().ListServices("project").Return([]*config.Service{
					&config.Service{
						Name: "another-app",
						Type: manifest.LoadBalancedWebServiceType,
					},
				}, nil)
				opts.storeClient = mockStoreClient
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
					AppType:        manifest.LoadBalancedWebServiceType,
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
				require.Equal(t, tc.inAppName, manifest.Service.Name)
				require.Equal(t, tc.inAppPort, manifest.Image.Port)
				require.Equal(t, tc.inDockerfilePath, manifest.Image.ServiceImage.Build)
				require.Equal(t, tc.wantedPath, manifest.LoadBalancedWebServiceConfig.Path)
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}

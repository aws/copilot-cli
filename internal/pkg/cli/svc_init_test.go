// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.

package cli

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/cli/mocks"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/config"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/docker/dockerfile"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/manifest"
	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/term/log"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestSvcInitOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inSvcType        string
		inSvcName        string
		inDockerfilePath string
		inAppName        string
		inSvcPort        uint16

		mockFileSystem func(mockFS afero.Fs)
		wantedErr      error
	}{
		"invalid service type": {
			inAppName: "phonetool",
			inSvcType: "TestSvcType",
			wantedErr: errors.New(`invalid service type TestSvcType: must be one of "Load Balanced Web Service", "Backend Service"`),
		},
		"invalid service name": {
			inAppName: "phonetool",
			inSvcName: "1234",
			wantedErr: fmt.Errorf("service name 1234 is invalid: %s", errValueBadFormat),
		},
		"invalid dockerfile directory path": {
			inAppName:        "phonetool",
			inDockerfilePath: "./hello/Dockerfile",
			wantedErr:        errors.New("open hello/Dockerfile: file does not exist"),
		},
		"invalid app name": {
			inAppName: "",
			wantedErr: errNoAppInWorkspace,
		},
		"valid flags": {
			inSvcName:        "frontend",
			inSvcType:        "Load Balanced Web Service",
			inDockerfilePath: "./hello/Dockerfile",
			inAppName:        "phonetool",

			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("hello", 0755)
				afero.WriteFile(mockFS, "hello/Dockerfile", []byte("FROM nginx"), 0644)
			},
			wantedErr: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			opts := initSvcOpts{
				initSvcVars: initSvcVars{
					ServiceType:    tc.inSvcType,
					Name:           tc.inSvcName,
					DockerfilePath: tc.inDockerfilePath,
					Port:           tc.inSvcPort,
					GlobalOpts:     &GlobalOpts{appName: tc.inAppName},
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
func TestSvcInitOpts_Ask(t *testing.T) {
	const (
		wantedSvcType        = manifest.LoadBalancedWebServiceType
		wantedSvcName        = "frontend"
		wantedDockerfilePath = "frontend/Dockerfile"
		wantedSvcPort        = 80
	)
	testCases := map[string]struct {
		inSvcType        string
		inSvcName        string
		inDockerfilePath string
		inSvcPort        uint16

		mockFileSystem func(mockFS afero.Fs)
		mockPrompt     func(m *mocks.Mockprompter)
		mockDockerfile func(m *mocks.MockdockerfileParser)

		wantedErr error
	}{
		"prompt for service type": {
			inSvcType:        "",
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq(fmt.Sprintf(fmtSvcInitSvcTypePrompt, "service type")), gomock.Any(), gomock.Eq(manifest.ServiceTypes), gomock.Any()).
					Return(wantedSvcType, nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			wantedErr:      nil,
		},
		"return an error if fail to get service type": {
			inSvcType:        "",
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Eq(manifest.ServiceTypes), gomock.Any()).
					Return("", errors.New("some error"))
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			wantedErr:      fmt.Errorf("select service type: some error"),
		},
		"prompt for service name": {
			inSvcType:        wantedSvcType,
			inSvcName:        "",
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf("What do you want to name this %s?", wantedSvcType)), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedSvcName, nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			wantedErr:      nil,
		},
		"returns an error if fail to get service name": {
			inSvcType:        wantedSvcType,
			inSvcName:        "",
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			wantedErr:      fmt.Errorf("get service name: some error"),
		},
		"choose an existing Dockerfile": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: "",

			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("frontend", 0755)
				mockFS.MkdirAll("backend", 0755)

				afero.WriteFile(mockFS, "Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "frontend/Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "backend/Dockerfile", []byte("FROM nginx"), 0644)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq(fmt.Sprintf(fmtSvcInitDockerfilePrompt, "Dockerfile", wantedSvcName)), svcInitDockerfileHelpPrompt, gomock.Eq(
					[]string{
						"./Dockerfile",
						"backend/Dockerfile",
						"frontend/Dockerfile",
					}), gomock.Any()).
					Return("frontend/Dockerfile", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			wantedErr:      nil,
		},
		"returns an error if fail to find Dockerfiles": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			wantedErr:      fmt.Errorf("no Dockerfiles found within . or a sub-directory level below"),
		},
		"returns an error if fail to select Dockerfile": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: "",

			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("frontend", 0755)
				mockFS.MkdirAll("backend", 0755)

				afero.WriteFile(mockFS, "Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "frontend/Dockerfile", []byte("FROM nginx"), 0644)
				afero.WriteFile(mockFS, "backend/Dockerfile", []byte("FROM nginx"), 0644)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(gomock.Eq(fmt.Sprintf(fmtSvcInitDockerfilePrompt, "Dockerfile", wantedSvcName)), gomock.Any(), gomock.Eq(
					[]string{
						"./Dockerfile",
						"backend/Dockerfile",
						"frontend/Dockerfile",
					}), gomock.Any()).
					Return("", errors.New("some error"))
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			wantedErr:      fmt.Errorf("select Dockerfile: some error"),
		},
		"asks for port if not specified": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        0, //invalid port, default case

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf(svcInitSvcPortPrompt, "port")), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(defaultSvcPortString, nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetExposedPorts().Return([]uint16{}, errors.New("no expose"))
			},
			wantedErr: nil,
		},
		"errors if port not specified": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        0, //invalid port, default case

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf(svcInitSvcPortPrompt, "port")), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetExposedPorts().Return([]uint16{}, errors.New("expose error"))
			},
			wantedErr: fmt.Errorf("get port: some error"),
		},
		"errors if port out of range": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        0, //invalid port, default case

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf(svcInitSvcPortPrompt, "port")), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("100000", errors.New("some error"))
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetExposedPorts().Return([]uint16{}, errors.New("no expose"))
			},
			wantedErr: fmt.Errorf("get port: some error"),
		},
		"don't ask if dockerfile has port": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        0,

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetExposedPorts().Return([]uint16{80}, nil)
			},
		},
		"don't use dockerfile port if flag specified": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        wantedSvcPort,

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
			opts := &initSvcOpts{
				initSvcVars: initSvcVars{
					ServiceType:    tc.inSvcType,
					Name:           tc.inSvcName,
					Port:           tc.inSvcPort,
					DockerfilePath: tc.inDockerfilePath,
					GlobalOpts: &GlobalOpts{
						prompt: mockPrompt,
					},
				},
				fs:          &afero.Afero{Fs: afero.NewMemMapFs()},
				setupParser: func(o *initSvcOpts) {},
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
				require.Equal(t, wantedSvcType, opts.ServiceType)
				require.Equal(t, wantedSvcName, opts.Name)
				require.Equal(t, wantedDockerfilePath, opts.DockerfilePath)
			}
		})
	}
}

func TestAppInitOpts_Execute(t *testing.T) {
	var (
		testInterval    = 10 * time.Second
		testRetries     = 2
		testTimeout     = 5 * time.Second
		testStartPeriod = 0 * time.Second
	)
	testCases := map[string]struct {
		inSvcPort        uint16
		inSvcType        string
		inSvcName        string
		inDockerfilePath string
		inAppName        string
		mockDependencies func(*gomock.Controller, *initSvcOpts)
		wantedErr        error
	}{
		"writes Load Balanced Web Service manifest, and creates repositories successfully": {
			inSvcType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			inSvcPort:        80,

			mockDependencies: func(ctrl *gomock.Controller, opts *initSvcOpts) {
				mockWriter := mocks.NewMocksvcManifestWriter(ctrl)
				mockWriter.EXPECT().WriteServiceManifest(gomock.Any(), opts.Name).Return("/frontend/manifest.yml", nil)

				mockstore := mocks.NewMockstore(ctrl)
				mockstore.EXPECT().ListServices("app").Return([]*config.Service{}, nil)
				mockstore.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Service) {
						require.Equal(t, &config.Service{
							Name: "frontend",
							App:  "app",
							Type: manifest.LoadBalancedWebServiceType,
						}, app)
					}).
					Return(nil)

				mockstore.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)

				mockappDeployer := mocks.NewMockappDeployer(ctrl)
				mockappDeployer.EXPECT().AddServiceToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "frontend")

				mockProg := mocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(fmt.Sprintf(fmtAddSvcToAppStart, "frontend"))
				mockProg.EXPECT().Stop(log.Ssuccessf(fmtAddSvcToAppComplete, "frontend"))

				opts.ws = mockWriter
				opts.store = mockstore
				opts.appDeployer = mockappDeployer
				opts.prog = mockProg
			},
		},
		"write manifest error": {
			inSvcType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			inSvcPort:        80,

			mockDependencies: func(ctrl *gomock.Controller, opts *initSvcOpts) {
				mockWriter := mocks.NewMocksvcManifestWriter(ctrl)
				mockWriter.EXPECT().WriteServiceManifest(gomock.Any(), opts.Name).Return("/frontend/manifest.yml", errors.New("some error"))

				mockstore := mocks.NewMockstore(ctrl)
				mockstore.EXPECT().ListServices("app").Return([]*config.Service{}, nil)

				mockstore.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)

				mockappDeployer := mocks.NewMockappDeployer(ctrl)

				mockProg := mocks.NewMockprogress(ctrl)

				opts.ws = mockWriter
				opts.store = mockstore
				opts.appDeployer = mockappDeployer
				opts.prog = mockProg
			},
			wantedErr: errors.New("some error"),
		},
		"app error": {
			inSvcType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "frontend/Dockerfile",

			mockDependencies: func(ctrl *gomock.Controller, opts *initSvcOpts) {
				mockWriter := mocks.NewMocksvcManifestWriter(ctrl)

				mockstore := mocks.NewMockstore(ctrl)

				mockstore.EXPECT().GetApplication(gomock.Any()).Return(nil, errors.New("some error"))

				opts.ws = mockWriter
				opts.store = mockstore
			},
			wantedErr: errors.New("get application app: some error"),
		},
		"add service to app fails": {
			inSvcType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "frontend/Dockerfile",

			mockDependencies: func(ctrl *gomock.Controller, opts *initSvcOpts) {
				mockWriter := mocks.NewMocksvcManifestWriter(ctrl)
				mockWriter.EXPECT().WriteServiceManifest(gomock.Any(), opts.Name).Return("/frontend/manifest.yml", nil)

				mockstore := mocks.NewMockstore(ctrl)
				mockstore.EXPECT().ListServices("app").Return([]*config.Service{}, nil)
				mockstore.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)

				mockProg := mocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(fmt.Sprintf(fmtAddSvcToAppStart, "frontend"))
				mockProg.EXPECT().Stop(log.Serrorf(fmtAddSvcToAppFailed, "frontend"))

				mockappDeployer := mocks.NewMockappDeployer(ctrl)
				mockappDeployer.EXPECT().AddServiceToApp(gomock.Any(), gomock.Any()).Return(errors.New("some error"))

				opts.ws = mockWriter
				opts.store = mockstore
				opts.appDeployer = mockappDeployer
				opts.prog = mockProg
			},

			wantedErr: errors.New("add service frontend to application app: some error"),
		},
		"error saving app": {
			inSvcType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockDependencies: func(ctrl *gomock.Controller, opts *initSvcOpts) {
				mockWriter := mocks.NewMocksvcManifestWriter(ctrl)
				mockWriter.EXPECT().WriteServiceManifest(gomock.Any(), opts.Name).Return("/frontend/manifest.yml", nil)

				mockstore := mocks.NewMockstore(ctrl)
				mockstore.EXPECT().ListServices("app").Return([]*config.Service{}, nil)
				mockstore.EXPECT().CreateService(gomock.Any()).
					Return(fmt.Errorf("oops"))
				mockstore.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{}, nil)

				mockappDeployer := mocks.NewMockappDeployer(ctrl)
				mockappDeployer.EXPECT().AddServiceToApp(gomock.Any(), gomock.Any()).Return(nil)

				mockProg := mocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(gomock.Any())
				mockProg.EXPECT().Stop(gomock.Any())

				opts.ws = mockWriter
				opts.store = mockstore
				opts.appDeployer = mockappDeployer
				opts.prog = mockProg
			},

			wantedErr: fmt.Errorf("saving service frontend: oops"),
		},
		"no healthcheck options": {
			inSvcType:        manifest.BackendServiceType,
			inAppName:        "app",
			inSvcName:        "backend",
			inDockerfilePath: "backend/Dockerfile",
			inSvcPort:        80,

			mockDependencies: func(ctrl *gomock.Controller, opts *initSvcOpts) {
				mockWriter := mocks.NewMocksvcManifestWriter(ctrl)
				mockWriter.EXPECT().WriteServiceManifest(gomock.Any(), opts.Name).
					Do(func(m *manifest.BackendService, _ string) {
						require.Equal(t, *m.Service.Type, manifest.BackendServiceType)
						require.Nil(t, m.Image.HealthCheck)
					}).Return("/backend/manifest.yml", nil)

				mockstore := mocks.NewMockstore(ctrl)
				mockstore.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Service) {
						require.Equal(t, &config.Service{
							Name: "backend",
							App:  "app",
							Type: manifest.BackendServiceType,
						}, app)
					}).
					Return(nil)

				mockstore.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)

				mockappDeployer := mocks.NewMockappDeployer(ctrl)
				mockappDeployer.EXPECT().AddServiceToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "backend")

				mockProg := mocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(fmt.Sprintf(fmtAddSvcToAppStart, "backend"))
				mockProg.EXPECT().Stop(log.Ssuccessf(fmtAddSvcToAppComplete, "backend"))

				mockDf := mocks.NewMockdockerfileParser(ctrl)
				mockDf.EXPECT().GetHealthCheck().Return(nil, nil)

				opts.ws = mockWriter
				opts.store = mockstore
				opts.appDeployer = mockappDeployer
				opts.prog = mockProg
				opts.setupParser = func(o *initSvcOpts) {
					o.df = mockDf
				}
			},
		},
		"default healthcheck options": {
			inSvcType:        manifest.BackendServiceType,
			inAppName:        "app",
			inSvcName:        "backend",
			inDockerfilePath: "backend/Dockerfile",
			inSvcPort:        80,

			mockDependencies: func(ctrl *gomock.Controller, opts *initSvcOpts) {
				mockWriter := mocks.NewMocksvcManifestWriter(ctrl)
				mockWriter.EXPECT().WriteServiceManifest(gomock.Any(), opts.Name).
					Do(func(m *manifest.BackendService, _ string) {
						require.Equal(t, *m.Service.Type, manifest.BackendServiceType)
						require.Equal(t, *m.Image.HealthCheck, manifest.ContainerHealthCheck{
							Interval:    &testInterval,
							Retries:     &testRetries,
							Timeout:     &testTimeout,
							StartPeriod: &testStartPeriod,
							Command:     []string{"CMD curl -f http://localhost/ || exit 1"}})
					}).Return("/backend/manifest.yml", nil)

				mockstore := mocks.NewMockstore(ctrl)
				mockstore.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Service) {
						require.Equal(t, &config.Service{
							Name: "backend",
							App:  "app",
							Type: manifest.BackendServiceType,
						}, app)
					}).
					Return(nil)

				mockstore.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)

				mockappDeployer := mocks.NewMockappDeployer(ctrl)
				mockappDeployer.EXPECT().AddServiceToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "backend")

				mockProg := mocks.NewMockprogress(ctrl)
				mockProg.EXPECT().Start(fmt.Sprintf(fmtAddSvcToAppStart, "backend"))
				mockProg.EXPECT().Stop(log.Ssuccessf(fmtAddSvcToAppComplete, "backend"))

				mockDf := mocks.NewMockdockerfileParser(ctrl)
				mockDf.EXPECT().GetHealthCheck().
					Return(&dockerfile.HealthCheck{
						Interval:    10000000000,
						Retries:     2,
						Timeout:     5000000000,
						StartPeriod: 0,
						Cmd:         []string{"CMD curl -f http://localhost/ || exit 1"}},
						nil)

				opts.ws = mockWriter
				opts.store = mockstore
				opts.appDeployer = mockappDeployer
				opts.prog = mockProg
				opts.setupParser = func(o *initSvcOpts) {
					o.df = mockDf
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			opts := initSvcOpts{
				initSvcVars: initSvcVars{
					ServiceType:    tc.inSvcType,
					Name:           tc.inSvcName,
					Port:           tc.inSvcPort,
					DockerfilePath: tc.inDockerfilePath,
					GlobalOpts:     &GlobalOpts{appName: tc.inAppName},
				},
				setupParser: func(o *initSvcOpts) {},
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
		inSvcPort        uint16
		inSvcName        string
		inDockerfilePath string
		inAppName        string
		mockDependencies func(*gomock.Controller, *initSvcOpts)
		wantedErr        error
		wantedPath       string
	}{
		"creates manifest with / as the path when there are no other apps": {
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "frontend/Dockerfile",
			wantedPath:       "/",
			mockDependencies: func(ctrl *gomock.Controller, opts *initSvcOpts) {
				mockstore := mocks.NewMockstore(ctrl)
				mockstore.EXPECT().ListServices("app").Return([]*config.Service{}, nil)
				opts.store = mockstore
			},
		},
		"creates manifest with / as the path when it's the only app": {
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "frontend/Dockerfile",
			wantedPath:       "/",
			mockDependencies: func(ctrl *gomock.Controller, opts *initSvcOpts) {
				mockstore := mocks.NewMockstore(ctrl)
				mockstore.EXPECT().ListServices("app").Return([]*config.Service{
					{
						Name: "frontend",
						Type: manifest.LoadBalancedWebServiceType,
					},
				}, nil)
				opts.store = mockstore
			},
		},
		"creates manifest with / as the path when it's the only LBWebApp": {
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "frontend/Dockerfile",
			wantedPath:       "/",
			mockDependencies: func(ctrl *gomock.Controller, opts *initSvcOpts) {
				mockstore := mocks.NewMockstore(ctrl)
				mockstore.EXPECT().ListServices("app").Return([]*config.Service{
					{
						Name: "another-app",
						Type: "backend",
					},
				}, nil)
				opts.store = mockstore
			},
		},
		"creates manifest with {app name} as the path if there's another LBWebApp": {
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "frontend/Dockerfile",
			wantedPath:       "frontend",
			mockDependencies: func(ctrl *gomock.Controller, opts *initSvcOpts) {
				mockstore := mocks.NewMockstore(ctrl)
				mockstore.EXPECT().ListServices("app").Return([]*config.Service{
					{
						Name: "another-app",
						Type: manifest.LoadBalancedWebServiceType,
					},
				}, nil)
				opts.store = mockstore
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			opts := initSvcOpts{
				initSvcVars: initSvcVars{
					ServiceType:    manifest.LoadBalancedWebServiceType,
					Name:           tc.inSvcName,
					Port:           tc.inSvcPort,
					DockerfilePath: tc.inDockerfilePath,
					GlobalOpts:     &GlobalOpts{appName: tc.inAppName},
				},
			}
			tc.mockDependencies(ctrl, &opts)
			// WHEN
			manifest, err := opts.newLoadBalancedWebServiceManifest()

			// THEN
			if tc.wantedErr == nil {
				require.Nil(t, err)
				require.Equal(t, tc.inSvcName, aws.StringValue(manifest.Service.Name))
				require.Equal(t, tc.inSvcPort, aws.Uint16Value(manifest.Image.Port))
				require.Equal(t, tc.inDockerfilePath, aws.StringValue(manifest.Image.ServiceImage.Build))
				require.Equal(t, tc.wantedPath, aws.StringValue(manifest.Path))
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}

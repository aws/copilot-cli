// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerfile"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/aws/copilot-cli/internal/pkg/term/color"
	"github.com/aws/copilot-cli/internal/pkg/term/log"
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
					serviceType:    tc.inSvcType,
					name:           tc.inSvcName,
					dockerfilePath: tc.inDockerfilePath,
					port:           tc.inSvcPort,
					appName:        tc.inAppName,
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
				require.NoError(t, err)
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
				m.EXPECT().SelectOne(gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, color.Emphasize("Dockerfile"), wantedSvcName)), wkldInitDockerfileHelpPrompt, gomock.Eq(
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
		"prompts user for custom path if fail to find Dockerfiles": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, "Dockerfile", wantedSvcName)), gomock.Eq(wkldInitDockerfilePathHelpPrompt), gomock.Any(), gomock.Any()).
					Return("frontend/Dockerfile", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			wantedErr:      nil,
		},
		"returns an error if fail to get custom Dockerfile path": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: "",

			mockFileSystem: func(mockFS afero.Fs) {},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, "Dockerfile", wantedSvcName)), gomock.Eq(wkldInitDockerfilePathHelpPrompt), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			wantedErr:      fmt.Errorf("get custom Dockerfile path: some error"),
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
				m.EXPECT().SelectOne(gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, "Dockerfile", wantedSvcName)), gomock.Any(), gomock.Eq(
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
					serviceType:    tc.inSvcType,
					name:           tc.inSvcName,
					port:           tc.inSvcPort,
					dockerfilePath: tc.inDockerfilePath,
				},
				fs:          &afero.Afero{Fs: afero.NewMemMapFs()},
				setupParser: func(o *initSvcOpts) {},
				df:          mockDockerfile,
				prompt:      mockPrompt,
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
				require.NoError(t, err)
				require.Equal(t, wantedSvcType, opts.serviceType)
				require.Equal(t, wantedSvcName, opts.name)
				require.Equal(t, wantedDockerfilePath, opts.dockerfilePath)
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
		mockWriter       func(m *mocks.MocksvcDirManifestWriter)
		mockstore        func(m *mocks.Mockstore)
		mockappDeployer  func(m *mocks.MockappDeployer)
		mockProg         func(m *mocks.Mockprogress)
		mockDf           func(m *mocks.MockdockerfileParser)

		wantedErr error
	}{
		"writes Load Balanced Web Service manifest, and creates repositories successfully": {
			inSvcType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			inSvcPort:        80,

			mockWriter: func(m *mocks.MocksvcDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/frontend", nil)
				m.EXPECT().WriteWorkloadManifest(gomock.Any(), "frontend").Return("/frontend/manifest.yml", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("app").Return([]*config.Service{}, nil)
				m.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Service) {
						require.Equal(t, &config.Service{
							Name: "frontend",
							App:  "app",
							Type: manifest.LoadBalancedWebServiceType,
						}, app)
					}).
					Return(nil)
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockappDeployer: func(m *mocks.MockappDeployer) {
				m.EXPECT().AddServiceToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "frontend")
			},
			mockProg: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddSvcToAppStart, "frontend"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddSvcToAppComplete, "frontend"))
			},
		},
		"write manifest error": {
			inSvcType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",
			inSvcPort:        80,

			mockWriter: func(m *mocks.MocksvcDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/frontend", nil)
				m.EXPECT().WriteWorkloadManifest(gomock.Any(), "frontend").Return("/frontend/manifest.yml", errors.New("some error"))
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("app").Return([]*config.Service{}, nil)
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			wantedErr: errors.New("some error"),
		},
		"app error": {
			inSvcType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "frontend/Dockerfile",

			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication(gomock.Any()).Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("get application app: some error"),
		},
		"add service to app fails": {
			inSvcType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "frontend/Dockerfile",

			mockWriter: func(m *mocks.MocksvcDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/frontend", nil)
				m.EXPECT().WriteWorkloadManifest(gomock.Any(), "frontend").Return("/frontend/manifest.yml", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("app").Return([]*config.Service{}, nil)
				m.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockProg: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddSvcToAppStart, "frontend"))
				m.EXPECT().Stop(log.Serrorf(fmtAddSvcToAppFailed, "frontend"))
			},
			mockappDeployer: func(m *mocks.MockappDeployer) {
				m.EXPECT().AddServiceToApp(gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			wantedErr: errors.New("add service frontend to application app: some error"),
		},
		"error saving app": {
			inSvcType:        manifest.LoadBalancedWebServiceType,
			inAppName:        "app",
			inSvcName:        "frontend",
			inDockerfilePath: "frontend/Dockerfile",

			mockWriter: func(m *mocks.MocksvcDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/frontend", nil)
				m.EXPECT().WriteWorkloadManifest(gomock.Any(), "frontend").Return("/frontend/manifest.yml", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("app").Return([]*config.Service{}, nil)
				m.EXPECT().CreateService(gomock.Any()).
					Return(fmt.Errorf("oops"))
				m.EXPECT().GetApplication(gomock.Any()).Return(&config.Application{}, nil)
			},
			mockappDeployer: func(m *mocks.MockappDeployer) {
				m.EXPECT().AddServiceToApp(gomock.Any(), gomock.Any()).Return(nil)
			},
			mockProg: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(gomock.Any())
				m.EXPECT().Stop(gomock.Any())
			},
			wantedErr: fmt.Errorf("saving service frontend: oops"),
		},
		"no healthcheck options": {
			inSvcType:        manifest.BackendServiceType,
			inAppName:        "app",
			inSvcName:        "backend",
			inDockerfilePath: "backend/Dockerfile",
			inSvcPort:        80,

			mockWriter: func(m *mocks.MocksvcDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/backend", nil)
				m.EXPECT().WriteWorkloadManifest(gomock.Any(), "backend").
					Do(func(m *manifest.BackendService, _ string) {
						require.Equal(t, *m.Workload.Type, manifest.BackendServiceType)
						require.Nil(t, m.Image.HealthCheck)
					}).Return("/backend/manifest.yml", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Service) {
						require.Equal(t, &config.Service{
							Name: "backend",
							App:  "app",
							Type: manifest.BackendServiceType,
						}, app)
					}).
					Return(nil)

				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockappDeployer: func(m *mocks.MockappDeployer) {
				m.EXPECT().AddServiceToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "backend")
			},
			mockProg: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddSvcToAppStart, "backend"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddSvcToAppComplete, "backend"))
			},
			mockDf: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().Return(nil, nil)
			},
		},
		"default healthcheck options": {
			inSvcType:        manifest.BackendServiceType,
			inAppName:        "app",
			inSvcName:        "backend",
			inDockerfilePath: "backend/Dockerfile",
			inSvcPort:        80,

			mockWriter: func(m *mocks.MocksvcDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/backend", nil)
				m.EXPECT().WriteWorkloadManifest(gomock.Any(), "backend").
					Do(func(m *manifest.BackendService, _ string) {
						require.Equal(t, *m.Workload.Type, manifest.BackendServiceType)
						require.Equal(t, *m.Image.HealthCheck, manifest.ContainerHealthCheck{
							Interval:    &testInterval,
							Retries:     &testRetries,
							Timeout:     &testTimeout,
							StartPeriod: &testStartPeriod,
							Command:     []string{"CMD curl -f http://localhost/ || exit 1"}})
					}).Return("/backend/manifest.yml", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().CreateService(gomock.Any()).
					Do(func(app *config.Service) {
						require.Equal(t, &config.Service{
							Name: "backend",
							App:  "app",
							Type: manifest.BackendServiceType,
						}, app)
					}).
					Return(nil)
				m.EXPECT().GetApplication("app").Return(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, nil)
			},
			mockappDeployer: func(m *mocks.MockappDeployer) {
				m.EXPECT().AddServiceToApp(&config.Application{
					Name:      "app",
					AccountID: "1234",
				}, "backend")
			},
			mockProg: func(m *mocks.Mockprogress) {
				m.EXPECT().Start(fmt.Sprintf(fmtAddSvcToAppStart, "backend"))
				m.EXPECT().Stop(log.Ssuccessf(fmtAddSvcToAppComplete, "backend"))
			},
			mockDf: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().
					Return(&dockerfile.HealthCheck{
						Interval:    10000000000,
						Retries:     2,
						Timeout:     5000000000,
						StartPeriod: 0,
						Cmd:         []string{"CMD curl -f http://localhost/ || exit 1"}},
						nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWriter := mocks.NewMocksvcDirManifestWriter(ctrl)
			mockstore := mocks.NewMockstore(ctrl)
			mockappDeployer := mocks.NewMockappDeployer(ctrl)
			mockProg := mocks.NewMockprogress(ctrl)
			mockDf := mocks.NewMockdockerfileParser(ctrl)
			if tc.mockWriter != nil {
				tc.mockWriter(mockWriter)
			}
			if tc.mockstore != nil {
				tc.mockstore(mockstore)
			}
			if tc.mockappDeployer != nil {
				tc.mockappDeployer(mockappDeployer)
			}
			if tc.mockProg != nil {
				tc.mockProg(mockProg)
			}
			if tc.mockDf != nil {
				tc.mockDf(mockDf)
			}
			opts := initSvcOpts{
				initSvcVars: initSvcVars{
					serviceType:    tc.inSvcType,
					name:           tc.inSvcName,
					port:           tc.inSvcPort,
					dockerfilePath: tc.inDockerfilePath,
					appName:        tc.inAppName,
				},
				setupParser: func(o *initSvcOpts) {},
				ws:          mockWriter,
				store:       mockstore,
				appDeployer: mockappDeployer,
				prog:        mockProg,
				df:          mockDf,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedErr == nil {
				require.NoError(t, err)
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
		setupMocks       func(controller *gomock.Controller)
		mockWriter       func(m *mocks.MocksvcDirManifestWriter)
		mockstore        func(m *mocks.Mockstore)

		wantedErr  error
		wantedPath string
	}{
		"creates manifest with / as the path when there are no other apps": {
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "/Dockerfile",

			mockWriter: func(m *mocks.MocksvcDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/copilot", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("app").Return([]*config.Service{}, nil)
			},

			wantedPath: "/",
		},
		"creates manifest with / as the path when it's the only app": {
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "/Dockerfile",

			mockWriter: func(m *mocks.MocksvcDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/copilot", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("app").Return([]*config.Service{
					{
						Name: "frontend",
						Type: manifest.LoadBalancedWebServiceType,
					},
				}, nil)
			},

			wantedPath: "/",
		},
		"creates manifest with / as the path when it's the only LBWebApp": {
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "/Dockerfile",

			mockWriter: func(m *mocks.MocksvcDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/copilot", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("app").Return([]*config.Service{
					{
						Name: "another-app",
						Type: "backend",
					},
				}, nil)
			},

			wantedPath: "/",
		},
		"creates manifest with {app name} as the path if there's another LBWebApp": {
			inAppName:        "app",
			inSvcName:        "frontend",
			inSvcPort:        80,
			inDockerfilePath: "/Dockerfile",

			mockWriter: func(m *mocks.MocksvcDirManifestWriter) {
				m.EXPECT().CopilotDirPath().Return("/copilot", nil)
			},
			mockstore: func(m *mocks.Mockstore) {
				m.EXPECT().ListServices("app").Return([]*config.Service{
					{
						Name: "another-app",
						Type: manifest.LoadBalancedWebServiceType,
					},
				}, nil)
			},

			wantedPath: "frontend",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockWriter := mocks.NewMocksvcDirManifestWriter(ctrl)
			mockstore := mocks.NewMockstore(ctrl)
			if tc.mockWriter != nil {
				tc.mockWriter(mockWriter)
			}
			if tc.mockstore != nil {
				tc.mockstore(mockstore)
			}
			opts := initSvcOpts{
				initSvcVars: initSvcVars{
					serviceType:    manifest.LoadBalancedWebServiceType,
					name:           tc.inSvcName,
					port:           tc.inSvcPort,
					dockerfilePath: tc.inDockerfilePath,
					appName:        tc.inAppName,
				},
				ws:    mockWriter,
				store: mockstore,
			}

			// WHEN
			manifest, err := opts.newLoadBalancedWebServiceManifest()

			// THEN
			if tc.wantedErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.inSvcName, aws.StringValue(manifest.Workload.Name))
				require.Equal(t, tc.inSvcPort, aws.Uint16Value(manifest.Image.Port))
				require.Contains(t, tc.inDockerfilePath, aws.StringValue(manifest.Image.Build.BuildArgs.Dockerfile))
				require.Equal(t, tc.wantedPath, aws.StringValue(manifest.Path))
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}

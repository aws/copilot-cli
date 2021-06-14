// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/term/prompt"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/exec"
	"github.com/aws/copilot-cli/internal/pkg/initialize"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestSvcInitOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inSvcType        string
		inSvcName        string
		inDockerfilePath string
		inImage          string
		inAppName        string
		inSvcPort        uint16

		mockFileSystem func(mockFS afero.Fs)
		wantedErr      error
	}{
		"invalid service type": {
			inAppName: "phonetool",
			inSvcType: "TestSvcType",
			wantedErr: errors.New(`invalid service type TestSvcType: must be one of "Request-Driven Web Service", "Load Balanced Web Service", "Backend Service"`),
		},
		"invalid service name": {
			inAppName: "phonetool",
			inSvcName: "1234",
			wantedErr: fmt.Errorf("service name 1234 is invalid: %s", errValueBadFormat),
		},
		"fail if both image and dockerfile are set": {
			inAppName:        "phonetool",
			inDockerfilePath: "mockDockerfile",
			inImage:          "mockImage",
			wantedErr:        fmt.Errorf("--dockerfile and --image cannot be specified together"),
		},
		"fail if image not supported by App Runner": {
			inAppName: "phonetool",
			inImage:   "amazon/amazon-ecs-sample",
			inSvcType: manifest.RequestDrivenWebServiceType,
			wantedErr: fmt.Errorf("image amazon/amazon-ecs-sample is not supported by App Runner: value must be an ECR or ECR Public image URI"),
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
					initWkldVars: initWkldVars{
						wkldType:       tc.inSvcType,
						name:           tc.inSvcName,
						dockerfilePath: tc.inDockerfilePath,
						image:          tc.inImage,
						appName:        tc.inAppName,
					},
					port: tc.inSvcPort,
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
		wantedImage          = "mockImage"
	)
	testCases := map[string]struct {
		inSvcType        string
		inSvcName        string
		inDockerfilePath string
		inImage          string
		inSvcPort        uint16

		mockPrompt     func(m *mocks.Mockprompter)
		mockSel        func(m *mocks.MockdockerfileSelector)
		mockDockerfile func(m *mocks.MockdockerfileParser)
		mockValidator  func(m *mocks.MockdockerEngineValidator)

		wantedErr error
	}{
		"prompt for service type": {
			inSvcType:        "",
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOption(gomock.Eq(fmt.Sprintf(fmtSvcInitSvcTypePrompt, "service type")), gomock.Any(), gomock.Eq([]prompt.Option{
					{
						Value: manifest.RequestDrivenWebServiceType,
						Hint:  "App Runner",
					},
					{
						Value: manifest.LoadBalancedWebServiceType,
						Hint:  "Internet to ECS on Fargate",
					},
					{
						Value: manifest.BackendServiceType,
						Hint:  "ECS on Fargate",
					},
				}), gomock.Any()).
					Return(wantedSvcType, nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			mockSel:        func(m *mocks.MockdockerfileSelector) {},
			mockValidator:  func(m *mocks.MockdockerEngineValidator) {},
			wantedErr:      nil,
		},
		"return an error if fail to get service type": {
			inSvcType:        "",
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			mockSel:        func(m *mocks.MockdockerfileSelector) {},
			mockValidator:  func(m *mocks.MockdockerEngineValidator) {},
			wantedErr:      fmt.Errorf("select service type: some error"),
		},
		"prompt for service name": {
			inSvcType:        wantedSvcType,
			inSvcName:        "",
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf("What do you want to name this %s?", wantedSvcType)), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedSvcName, nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
			},
			mockSel:       func(m *mocks.MockdockerfileSelector) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {},
			wantedErr:     nil,
		},
		"returns an error if fail to get service name": {
			inSvcType:        wantedSvcType,
			inSvcName:        "",
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			mockSel:        func(m *mocks.MockdockerfileSelector) {},
			mockValidator:  func(m *mocks.MockdockerEngineValidator) {},
			wantedErr:      fmt.Errorf("get service name: some error"),
		},
		"skip selecting Dockerfile if image flag is set": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inImage:          "mockImage",
			inDockerfilePath: "",

			mockPrompt:     func(m *mocks.Mockprompter) {},
			mockSel:        func(m *mocks.MockdockerfileSelector) {},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			mockValidator:  func(m *mocks.MockdockerEngineValidator) {},
			wantedErr:      nil,
		},
		"return error if failed to check if docker engine is running": {
			inSvcType: wantedSvcType,
			inSvcName: wantedSvcName,
			inSvcPort: wantedSvcPort,

			mockPrompt:     func(m *mocks.Mockprompter) {},
			mockSel:        func(m *mocks.MockdockerfileSelector) {},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {
				m.EXPECT().CheckDockerEngineRunning().Return(errors.New("some error"))
			},
			wantedErr: fmt.Errorf("check if docker engine is running: some error"),
		},
		"skip selecting Dockerfile if docker command is not found": {
			inSvcType: wantedSvcType,
			inSvcName: wantedSvcName,
			inSvcPort: wantedSvcPort,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("mockImage", nil)
			},
			mockSel:        func(m *mocks.MockdockerfileSelector) {},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {
				m.EXPECT().CheckDockerEngineRunning().Return(exec.ErrDockerCommandNotFound)
			},
			wantedErr: nil,
		},
		"skip selecting Dockerfile if docker engine is not responsive": {
			inSvcType: wantedSvcType,
			inSvcName: wantedSvcName,
			inSvcPort: wantedSvcPort,

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("mockImage", nil)
			},
			mockSel:        func(m *mocks.MockdockerfileSelector) {},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {
				m.EXPECT().CheckDockerEngineRunning().Return(&exec.ErrDockerDaemonNotResponsive{})
			},
			wantedErr: nil,
		},
		"returns an error if fail to get image location": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: "",

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("", mockError)
			},
			mockSel: func(m *mocks.MockdockerfileSelector) {
				m.EXPECT().Dockerfile(
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, wantedSvcName)),
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, wantedSvcName)),
					gomock.Eq(wkldInitDockerfileHelpPrompt),
					gomock.Eq(wkldInitDockerfilePathHelpPrompt),
					gomock.Any(),
				).Return("Use an existing image instead", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
			},
			wantedErr: fmt.Errorf("get image location: mock error"),
		},
		"using existing image": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: "",

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("mockImage", nil)
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf(svcInitSvcPortPrompt, "port")), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(defaultSvcPortString, nil)
			},
			mockSel: func(m *mocks.MockdockerfileSelector) {
				m.EXPECT().Dockerfile(
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, wantedSvcName)),
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, wantedSvcName)),
					gomock.Eq(wkldInitDockerfileHelpPrompt),
					gomock.Eq(wkldInitDockerfilePathHelpPrompt),
					gomock.Any(),
				).Return("Use an existing image instead", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
			},
		},
		"select Dockerfile": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: "",

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockSel: func(m *mocks.MockdockerfileSelector) {
				m.EXPECT().Dockerfile(
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, wantedSvcName)),
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, wantedSvcName)),
					gomock.Eq(wkldInitDockerfileHelpPrompt),
					gomock.Eq(wkldInitDockerfilePathHelpPrompt),
					gomock.Any(),
				).Return("frontend/Dockerfile", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
			},
			wantedErr: nil,
		},
		"returns an error if fail to get Dockerfile": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: "",

			mockSel: func(m *mocks.MockdockerfileSelector) {
				m.EXPECT().Dockerfile(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return("", errors.New("some error"))
			},
			mockPrompt:     func(m *mocks.Mockprompter) {},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
			},
			wantedErr: fmt.Errorf("select Dockerfile: some error"),
		},
		"skip asking for port for backend service": {
			inSvcType:        "Backend Service",
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,

			mockPrompt: func(m *mocks.Mockprompter) {},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetExposedPorts().Return([]uint16{}, errors.New("no expose"))
			},
			mockSel:       func(m *mocks.MockdockerfileSelector) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {},
			wantedErr:     nil,
		},
		"asks for port if not specified": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        0, //invalid port, default case

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf(svcInitSvcPortPrompt, "port")), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(defaultSvcPortString, nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetExposedPorts().Return([]uint16{}, errors.New("no expose"))
			},
			mockSel:       func(m *mocks.MockdockerfileSelector) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {},
			wantedErr:     nil,
		},
		"errors if port not specified": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        0, //invalid port, default case

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf(svcInitSvcPortPrompt, "port")), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetExposedPorts().Return([]uint16{}, errors.New("expose error"))
			},
			mockSel:       func(m *mocks.MockdockerfileSelector) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {},
			wantedErr:     fmt.Errorf("get port: some error"),
		},
		"errors if port out of range": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        0, //invalid port, default case

			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().Get(gomock.Eq(fmt.Sprintf(svcInitSvcPortPrompt, "port")), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("100000", errors.New("some error"))
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetExposedPorts().Return([]uint16{}, errors.New("no expose"))
			},
			mockSel:       func(m *mocks.MockdockerfileSelector) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {},
			wantedErr:     fmt.Errorf("get port: some error"),
		},
		"don't ask if dockerfile has port": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        0,

			mockPrompt: func(m *mocks.Mockprompter) {
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetExposedPorts().Return([]uint16{80}, nil)
			},
			mockSel:       func(m *mocks.MockdockerfileSelector) {},
			mockValidator: func(m *mocks.MockdockerEngineValidator) {},
		},
		"don't use dockerfile port if flag specified": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        wantedSvcPort,

			mockPrompt: func(m *mocks.Mockprompter) {
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {},
			mockSel:        func(m *mocks.MockdockerfileSelector) {},
			mockValidator:  func(m *mocks.MockdockerEngineValidator) {},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mocks.NewMockprompter(ctrl)
			mockDockerfile := mocks.NewMockdockerfileParser(ctrl)
			mockSel := mocks.NewMockdockerfileSelector(ctrl)
			mockValidator := mocks.NewMockdockerEngineValidator(ctrl)
			opts := &initSvcOpts{
				initSvcVars: initSvcVars{
					initWkldVars: initWkldVars{
						wkldType:       tc.inSvcType,
						name:           tc.inSvcName,
						image:          tc.inImage,
						dockerfilePath: tc.inDockerfilePath,
					},
					port: tc.inSvcPort,
				},
				fs: &afero.Afero{Fs: afero.NewMemMapFs()},
				dockerfile: func(s string) dockerfileParser {
					return mockDockerfile
				},
				df:                    mockDockerfile,
				prompt:                mockPrompt,
				sel:                   mockSel,
				dockerEngineValidator: mockValidator,
			}
			tc.mockSel(mockSel)
			tc.mockPrompt(mockPrompt)
			tc.mockDockerfile(mockDockerfile)
			tc.mockValidator(mockValidator)

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, wantedSvcName, opts.name)
				if opts.dockerfilePath != "" {
					require.Equal(t, wantedDockerfilePath, opts.dockerfilePath)
				}
				if opts.image != "" {
					require.Equal(t, wantedImage, opts.image)
				}
			}
		})
	}
}

func TestSvcInitOpts_Execute(t *testing.T) {
	testCases := map[string]struct {
		mockSvcInit      func(m *mocks.MocksvcInitializer)
		mockDockerfile   func(m *mocks.MockdockerfileParser)
		inSvcPort        uint16
		inSvcType        string
		inSvcName        string
		inDockerfilePath string
		inImage          string
		inAppName        string

		wantedErr          error
		wantedManifestPath string
	}{
		"success on typical svc props": {
			inAppName:        "sample",
			inSvcName:        "frontend",
			inDockerfilePath: "./Dockerfile",
			inSvcType:        manifest.LoadBalancedWebServiceType,

			inSvcPort: 80,

			mockSvcInit: func(m *mocks.MocksvcInitializer) {
				m.EXPECT().Service(&initialize.ServiceProps{
					WorkloadProps: initialize.WorkloadProps{
						App:            "sample",
						Name:           "frontend",
						Type:           "Load Balanced Web Service",
						DockerfilePath: "./Dockerfile",
					},
					Port: 80,
				}).Return("manifest/path", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().Return(nil, nil)
			},

			wantedManifestPath: "manifest/path",
		},
		"backend service": {
			inAppName:        "sample",
			inSvcName:        "frontend",
			inDockerfilePath: "./Dockerfile",
			inSvcType:        manifest.BackendServiceType,

			mockSvcInit: func(m *mocks.MocksvcInitializer) {
				m.EXPECT().Service(&initialize.ServiceProps{
					WorkloadProps: initialize.WorkloadProps{
						App:            "sample",
						Name:           "frontend",
						Type:           "Backend Service",
						DockerfilePath: "./Dockerfile",
					},
				}).Return("manifest/path", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().Return(nil, nil)
			},

			wantedManifestPath: "manifest/path",
		},
		"doesn't parse dockerfile if image specified (backend)": {
			inAppName:        "sample",
			inSvcName:        "backend",
			inDockerfilePath: "",
			inImage:          "nginx:latest",
			inSvcType:        manifest.BackendServiceType,

			mockSvcInit: func(m *mocks.MocksvcInitializer) {
				m.EXPECT().Service(&initialize.ServiceProps{
					WorkloadProps: initialize.WorkloadProps{
						App:   "sample",
						Name:  "backend",
						Type:  "Backend Service",
						Image: "nginx:latest",
					},
				}).Return("manifest/path", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {}, // Be sure that no dockerfile parsing happens.

			wantedManifestPath: "manifest/path",
		},
		"doesn't parse dockerfile if image specified (lb-web)": {
			inAppName:        "sample",
			inSvcName:        "frontend",
			inDockerfilePath: "",
			inImage:          "nginx:latest",
			inSvcType:        manifest.LoadBalancedWebServiceType,

			mockSvcInit: func(m *mocks.MocksvcInitializer) {
				m.EXPECT().Service(&initialize.ServiceProps{
					WorkloadProps: initialize.WorkloadProps{
						App:   "sample",
						Name:  "frontend",
						Type:  "Load Balanced Web Service",
						Image: "nginx:latest",
					},
				}).Return("manifest/path", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {}, // Be sure that no dockerfile parsing happens.

			wantedManifestPath: "manifest/path",
		},
		"failure": {
			mockSvcInit: func(m *mocks.MocksvcInitializer) {
				m.EXPECT().Service(gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: errors.New("some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSvcInitializer := mocks.NewMocksvcInitializer(ctrl)
			mockDockerfile := mocks.NewMockdockerfileParser(ctrl)

			if tc.mockSvcInit != nil {
				tc.mockSvcInit(mockSvcInitializer)
			}
			if tc.mockDockerfile != nil {
				tc.mockDockerfile(mockDockerfile)
			}

			opts := initSvcOpts{
				initSvcVars: initSvcVars{
					initWkldVars: initWkldVars{
						appName:        tc.inAppName,
						name:           tc.inSvcName,
						wkldType:       tc.inSvcType,
						dockerfilePath: tc.inDockerfilePath,
						image:          tc.inImage,
					},
					port: tc.inSvcPort,
				},
				init: mockSvcInitializer,
				dockerfile: func(s string) dockerfileParser {
					return mockDockerfile
				},
				df: mockDockerfile,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.wantedErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.wantedManifestPath, opts.manifestPath)
			} else {
				require.EqualError(t, err, tc.wantedErr.Error())
			}
		})
	}
}

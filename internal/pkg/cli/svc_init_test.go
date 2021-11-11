// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerfile"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"

	"github.com/aws/copilot-cli/internal/pkg/term/prompt"

	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/initialize"
	"github.com/aws/copilot-cli/internal/pkg/manifest"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type initSvcMocks struct {
	mockPrompt       *mocks.Mockprompter
	mockSel          *mocks.MockdockerfileSelector
	mocktopicSel     *mocks.MocktopicSelector
	mockDockerfile   *mocks.MockdockerfileParser
	mockDockerEngine *mocks.MockdockerEngine
	mockMftReader    *mocks.MockmanifestReader
}

func TestSvcInitOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inSvcType        string
		inSvcName        string
		inDockerfilePath string
		inImage          string
		inAppName        string
		inSvcPort        uint16
		inSubscribeTags  []string
		inNoSubscribe    bool

		mockFileSystem func(mockFS afero.Fs)
		wantedErr      error
	}{
		"invalid service type": {
			inAppName: "phonetool",
			inSvcType: "TestSvcType",
			wantedErr: errors.New(`invalid service type TestSvcType: must be one of "Request-Driven Web Service", "Load Balanced Web Service", "Backend Service", "Worker Service"`),
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
		"fail if both no-subscribe and subscribe are set": {
			inAppName:       "phonetool",
			inSvcName:       "service",
			inSubscribeTags: []string{"name:svc"},
			inNoSubscribe:   true,
			wantedErr:       errors.New("validate subscribe configuration: cannot specify both --no-subscribe and --subscribe-topics"),
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
						subscriptions:  tc.inSubscribeTags,
						noSubscribe:    tc.inNoSubscribe,
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
	mockTopic, _ := deploy.NewTopic("arn:aws:sns:us-west-2:123456789012:mockApp-mockEnv-mockWkld-orders", "mockApp", "mockEnv", "mockWkld")
	mockError := errors.New("mock error")
	testCases := map[string]struct {
		inSvcType        string
		inSvcName        string
		inDockerfilePath string
		inImage          string
		inSvcPort        uint16
		inSubscribeTags  []string
		inNoSubscribe    bool

		setupMocks func(mocks initSvcMocks)

		wantedErr error
	}{
		"prompt for service type": {
			inSvcType:        "",
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m initSvcMocks) {
				m.mockPrompt.EXPECT().SelectOption(gomock.Eq(fmt.Sprintf(fmtSvcInitSvcTypePrompt, "service type")), gomock.Any(), gomock.Eq([]prompt.Option{
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
					{
						Value: manifest.WorkerServiceType,
						Hint:  "Events to SQS to ECS on Fargate",
					},
				}), gomock.Any()).
					Return(wantedSvcType, nil)
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
			},
			wantedErr: nil,
		},
		"prompt for service name": {
			inSvcType:        wantedSvcType,
			inSvcName:        "",
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m initSvcMocks) {
				m.mockPrompt.EXPECT().Get(gomock.Eq(fmt.Sprintf("What do you want to name this %s?", wantedSvcType)), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedSvcName, nil)
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
			},
			wantedErr: nil,
		},
		"returns an error if fail to get service name": {
			inSvcType:        wantedSvcType,
			inSvcName:        "",
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m initSvcMocks) {
				m.mockPrompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get service name: some error"),
		},
		"skip asking questions if local manifest file exists": {
			inSvcType: "Worker Service",
			inSvcName: wantedSvcName,

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte(""), nil)
			},
		},
		"return an error if fail to read local manifest": {
			inSvcType: "Worker Service",
			inSvcName: wantedSvcName,

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, mockError)
			},

			wantedErr: fmt.Errorf("read manifest file for service frontend: mock error"),
		},
		"return an error if fail to get service type": {
			inSvcType:        "",
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("select service type: some error"),
		},
		"skip selecting Dockerfile if image flag is set": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inImage:          "mockImage",
			inDockerfilePath: "",

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
			},
		},
		"return error if failed to check if docker engine is running": {
			inSvcType: wantedSvcType,
			inSvcName: wantedSvcName,
			inSvcPort: wantedSvcPort,

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(errors.New("some error"))
			},
			wantedErr: fmt.Errorf("check if docker engine is running: some error"),
		},
		"skip selecting Dockerfile if docker command is not found": {
			inSvcType: wantedSvcType,
			inSvcName: wantedSvcName,
			inSvcPort: wantedSvcPort,

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("mockImage", nil)
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(dockerengine.ErrDockerCommandNotFound)

			},
			wantedErr: nil,
		},
		"skip selecting Dockerfile if docker engine is not responsive": {
			inSvcType: wantedSvcType,
			inSvcName: wantedSvcName,
			inSvcPort: wantedSvcPort,

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("mockImage", nil)
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(&dockerengine.ErrDockerDaemonNotResponsive{})

			},
			wantedErr: nil,
		},
		"returns an error if fail to get image location": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: "",

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("", mockError)
				m.mockSel.EXPECT().Dockerfile(
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, wantedSvcName)),
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, wantedSvcName)),
					gomock.Eq(wkldInitDockerfileHelpPrompt),
					gomock.Eq(wkldInitDockerfilePathHelpPrompt),
					gomock.Any(),
				).Return("Use an existing image instead", nil)
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(nil)
			},
			wantedErr: fmt.Errorf("get image location: mock error"),
		},
		"using existing image": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: "",

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, nil, gomock.Any()).
					Return("mockImage", nil)
				m.mockPrompt.EXPECT().Get(gomock.Eq(fmt.Sprintf(svcInitSvcPortPrompt, "port")), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(defaultSvcPortString, nil)
				m.mockSel.EXPECT().Dockerfile(
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, wantedSvcName)),
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, wantedSvcName)),
					gomock.Eq(wkldInitDockerfileHelpPrompt),
					gomock.Eq(wkldInitDockerfilePathHelpPrompt),
					gomock.Any(),
				).Return("Use an existing image instead", nil)
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(nil)
			},
		},
		"select Dockerfile": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: "",

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockSel.EXPECT().Dockerfile(
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePrompt, wantedSvcName)),
					gomock.Eq(fmt.Sprintf(fmtWkldInitDockerfilePathPrompt, wantedSvcName)),
					gomock.Eq(wkldInitDockerfileHelpPrompt),
					gomock.Eq(wkldInitDockerfilePathHelpPrompt),
					gomock.Any(),
				).Return("frontend/Dockerfile", nil)
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(nil)
			},
			wantedErr: nil,
		},
		"returns an error if fail to get Dockerfile": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: "",

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockSel.EXPECT().Dockerfile(
					gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
				).Return("", errors.New("some error"))
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(nil)
			},
			wantedErr: fmt.Errorf("select Dockerfile: some error"),
		},
		"skip asking for port for backend service": {
			inSvcType:        "Backend Service",
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockDockerfile.EXPECT().GetExposedPorts().Return(nil, errors.New("no expose"))
			},
			wantedErr: nil,
		},
		"asks for port if not specified": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        0, //invalid port, default case

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().Get(gomock.Eq(fmt.Sprintf(svcInitSvcPortPrompt, "port")), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(defaultSvcPortString, nil)
				m.mockDockerfile.EXPECT().GetExposedPorts().Return(nil, errors.New("no expose"))
			},
			wantedErr: nil,
		},
		"errors if port not specified": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        0, //invalid port, default case

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().Get(gomock.Eq(fmt.Sprintf(svcInitSvcPortPrompt, "port")), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
				m.mockDockerfile.EXPECT().GetExposedPorts().Return(nil, errors.New("expose error"))
			},
			wantedErr: fmt.Errorf("get port: some error"),
		},
		"errors if port out of range": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        0, //invalid port, default case

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().Get(gomock.Eq(fmt.Sprintf(svcInitSvcPortPrompt, "port")), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("100000", errors.New("some error"))
				m.mockDockerfile.EXPECT().GetExposedPorts().Return(nil, errors.New("no expose"))
			},
			wantedErr: fmt.Errorf("get port: some error"),
		},
		"don't ask if dockerfile has port": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        0,

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockDockerfile.EXPECT().GetExposedPorts().Return([]dockerfile.Port{{Port: 80, Protocol: "", RawString: "80"}}, nil)
			},
		},
		"don't use dockerfile port if flag specified": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        wantedSvcPort,

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
			},
		},
		"skip selecting subscriptions if no-subscriptions flag is set": {
			inSvcType:     "Worker Service",
			inSvcName:     wantedSvcName,
			inSvcPort:     wantedSvcPort,
			inImage:       "mockImage",
			inNoSubscribe: true,

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
			},
		},
		"skip selecting subscriptions if subscribe flag is set": {
			inSvcType:       "Worker Service",
			inSvcName:       wantedSvcName,
			inSvcPort:       wantedSvcPort,
			inImage:         "mockImage",
			inNoSubscribe:   false,
			inSubscribeTags: []string{"svc:name"},

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
			},
		},
		"select subscriptions": {
			inSvcType:        "Worker Service",
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inImage:          "mockImage",
			inDockerfilePath: "",

			setupMocks: func(m initSvcMocks) {
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mocktopicSel.EXPECT().Topics(
					gomock.Eq(svcInitPublisherPrompt),
					gomock.Eq(svcInitPublisherHelpPrompt),
					gomock.Any(),
				).Return([]deploy.Topic{*mockTopic}, nil)
			},
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
			mockTopicSel := mocks.NewMocktopicSelector(ctrl)
			mockDockerEngine := mocks.NewMockdockerEngine(ctrl)
			mockManifestReader := mocks.NewMockmanifestReader(ctrl)
			mocks := initSvcMocks{
				mockPrompt:       mockPrompt,
				mockDockerfile:   mockDockerfile,
				mockSel:          mockSel,
				mocktopicSel:     mockTopicSel,
				mockDockerEngine: mockDockerEngine,
				mockMftReader:    mockManifestReader,
			}
			tc.setupMocks(mocks)

			opts := &initSvcOpts{
				initSvcVars: initSvcVars{
					initWkldVars: initWkldVars{
						wkldType:       tc.inSvcType,
						name:           tc.inSvcName,
						image:          tc.inImage,
						dockerfilePath: tc.inDockerfilePath,
						noSubscribe:    tc.inNoSubscribe,
						subscriptions:  tc.inSubscribeTags,
					},
					port: tc.inSvcPort,
				},
				fs: &afero.Afero{Fs: afero.NewMemMapFs()},
				dockerfile: func(s string) dockerfileParser {
					return mockDockerfile
				},
				df:           mockDockerfile,
				prompt:       mockPrompt,
				mftReader:    mockManifestReader,
				sel:          mockSel,
				topicSel:     mockTopicSel,
				dockerEngine: mockDockerEngine,
			}

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
		mockDockerEngine func(m *mocks.MockdockerEngine)
		mockTopicSel     func(m *mocks.MocktopicSelector)
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
						Platform:       manifest.PlatformArgsOrString{},
					},
					Port: 80,
				}).Return("manifest/path", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().Return(nil, nil)
			},
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().GetPlatform().Return("linux", "amd64", nil)
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
						Platform:       manifest.PlatformArgsOrString{},
					},
				}).Return("manifest/path", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().Return(nil, nil)
			},
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().GetPlatform().Return("linux", "amd64", nil)
			},

			wantedManifestPath: "manifest/path",
		},
		"windows platform": {
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
						Platform: manifest.PlatformArgsOrString{
							PlatformString: (*manifest.PlatformString)(aws.String("windows/x86_64")),
						},
					},
					Port: 80,
				}).Return("manifest/path", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().Return(nil, nil)
			},
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().GetPlatform().Return("windows", "amd64", nil)
			},

			wantedManifestPath: "manifest/path",
		},
		"ARM architecture redirects to X86_64": {
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
						Platform: manifest.PlatformArgsOrString{
							PlatformString: (*manifest.PlatformString)(aws.String("linux/x86_64")),
						},
					},
					Port: 80,
				}).Return("manifest/path", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().Return(nil, nil)
			},
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().GetPlatform().Return("linux", "arm", nil)
			},

			wantedManifestPath: "manifest/path",
		},
		"worker service": {
			inAppName:        "sample",
			inSvcName:        "frontend",
			inDockerfilePath: "./Dockerfile",
			inSvcType:        manifest.WorkerServiceType,

			mockSvcInit: func(m *mocks.MocksvcInitializer) {
				m.EXPECT().Service(&initialize.ServiceProps{
					WorkloadProps: initialize.WorkloadProps{
						App:            "sample",
						Name:           "frontend",
						Type:           "Worker Service",
						DockerfilePath: "./Dockerfile",
						Platform:       manifest.PlatformArgsOrString{},
					},
				}).Return("manifest/path", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().Return(nil, nil)
			},
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().GetPlatform().Return("linux", "amd64", nil)
			},
			mockTopicSel: func(m *mocks.MocktopicSelector) {
				m.EXPECT().Topics(
					gomock.Eq(svcInitPublisherPrompt),
					gomock.Eq(svcInitPublisherHelpPrompt),
					gomock.Any(),
				).Return([]manifest.TopicSubscription{
					{
						Name:    aws.String("thetopic"),
						Service: aws.String("theservice"),
					},
				}, nil)
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
						App:      "sample",
						Name:     "backend",
						Type:     "Backend Service",
						Image:    "nginx:latest",
						Platform: manifest.PlatformArgsOrString{},
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
						App:      "sample",
						Name:     "frontend",
						Type:     "Load Balanced Web Service",
						Image:    "nginx:latest",
						Platform: manifest.PlatformArgsOrString{},
					},
				}).Return("manifest/path", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {}, // Be sure that no dockerfile parsing happens.

			wantedManifestPath: "manifest/path",
		},
		"return error if platform detection fails": {
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().GetPlatform().Return("", "", errors.New("some error"))
			},
			wantedErr: errors.New("get docker engine platform: some error"),
		},
		"return error if Windows platform attempted with RDWS": {
			inAppName:        "sample",
			inSvcName:        "appRunner",
			inDockerfilePath: "./Dockerfile",
			inSvcType:        manifest.RequestDrivenWebServiceType,

			inSvcPort: 80,

			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().Return(nil, nil)
			},
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().GetPlatform().Return("windows", "amd64", nil)
			},

			wantedErr: errors.New("redirect docker engine platform: Windows is not supported for App Runner services"),
		},
		"failure": {
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().GetPlatform().Return("linux", "amd64", nil)
			},
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
			mockDockerEngine := mocks.NewMockdockerEngine(ctrl)
			mockTopicSel := mocks.NewMocktopicSelector(ctrl)

			if tc.mockSvcInit != nil {
				tc.mockSvcInit(mockSvcInitializer)
			}
			if tc.mockDockerfile != nil {
				tc.mockDockerfile(mockDockerfile)
			}
			if tc.mockDockerEngine != nil {
				tc.mockDockerEngine(mockDockerEngine)
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
				df:           mockDockerfile,
				dockerEngine: mockDockerEngine,
				topicSel:     mockTopicSel,
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

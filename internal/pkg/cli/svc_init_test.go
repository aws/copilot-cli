// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerengine"
	"github.com/aws/copilot-cli/internal/pkg/docker/dockerfile"
	"github.com/aws/copilot-cli/internal/pkg/manifest/manifestinfo"
	"github.com/aws/copilot-cli/internal/pkg/term/prompt"
	"github.com/aws/copilot-cli/internal/pkg/workspace"

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
	mockSourceSel    *mocks.MockstaticSourceSelector
	mockDockerfile   *mocks.MockdockerfileParser
	mockDockerEngine *mocks.MockdockerEngine
	mockMftReader    *mocks.MockmanifestReader
	mockStore        *mocks.Mockstore
	mockSvcLister    *mocks.MockwlLister
	mockCachedWSRoot string
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
		inIngressType    string
		inSources        []string

		setupMocks     func(mocks *initSvcMocks)
		mockFileSystem func(mockFS afero.Fs)
		wantedErr      error
		wantedAssets   []manifest.FileUpload
	}{
		"fail if using different app name with the workspace": {
			inAppName: "demo",
			wantedErr: fmt.Errorf("cannot specify app demo because the workspace is already registered with app phonetool"),
		},
		"fail if cannot validate application": {
			inAppName:        "phonetool",
			inDockerfilePath: "mockDockerfile",
			inImage:          "mockImage",

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(nil, errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get application phonetool configuration: some error"),
		},
		"fail if both image and dockerfile are set": {
			inAppName:        "phonetool",
			inDockerfilePath: "mockDockerfile",
			inImage:          "mockImage",

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			wantedErr: fmt.Errorf("--dockerfile and --image cannot be specified together"),
		},
		"fail if image not supported by App Runner": {
			inAppName: "phonetool",
			inImage:   "amazon/amazon-ecs-sample",
			inSvcType: manifestinfo.RequestDrivenWebServiceType,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			wantedErr: fmt.Errorf("image amazon/amazon-ecs-sample is not supported by App Runner: value must be an ECR or ECR Public image URI"),
		},
		"invalid dockerfile directory path": {
			inAppName:        "phonetool",
			inDockerfilePath: "./hello/Dockerfile",

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			wantedErr: fmt.Errorf("open %s: file does not exist", filepath.FromSlash("hello/Dockerfile")),
		},
		"fail if both no-subscribe and subscribe are set": {
			inAppName:       "phonetool",
			inSvcName:       "service",
			inSubscribeTags: []string{"name:svc"},
			inNoSubscribe:   true,
			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			wantedErr: errors.New("validate subscribe configuration: cannot specify both --no-subscribe and --subscribe-topics"),
		},
		"rdws invalid ingress type error": {
			inSvcName:        "frontend",
			inSvcType:        "Request-Driven Web Service",
			inDockerfilePath: "./hello/Dockerfile",
			inIngressType:    "invalid",

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("hello", 0755)
				afero.WriteFile(mockFS, "hello/Dockerfile", []byte("FROM nginx"), 0644)
			},
			wantedErr: errors.New(`invalid ingress type "invalid": must be one of Environment or Internet`),
		},
		"error if sources flag used without Static Site type": {
			inSvcName: "frontend",
			inSvcType: "Load Balanced Web Service",
			inSources: []string{"goodbye", "hello/there"},

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			wantedErr: errors.New(`'--sources' must be specified with '--type "Static Site"'`),
		},
		"error if sources flag used with invalid sources": {
			inSvcName: "frontend",
			inSvcType: "Static Site",
			inSources: []string{"non-existent path"},

			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("copilot", 0755)
				mockFS.MkdirAll("hello", 0755)
				mockFS.MkdirAll("goodbye", 0755)
				afero.WriteFile(mockFS, "hello/there", []byte("howdy"), 0644)
			},
			setupMocks: func(m *initSvcMocks) {
				m.mockCachedWSRoot = "mockRoot"
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			wantedErr: errors.New(`source "non-existent path" must be a valid path relative to the workspace "mockRoot": open mockRoot/non-existent path: file does not exist`),
		},
		"valid flags": {
			inSvcName:        "frontend",
			inSvcType:        "Load Balanced Web Service",
			inDockerfilePath: "./hello/Dockerfile",

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("hello", 0755)
				afero.WriteFile(mockFS, "hello/Dockerfile", []byte("FROM nginx"), 0644)
			},
		},
		"valid rdws flags": {
			inSvcName:        "frontend",
			inSvcType:        "Request-Driven Web Service",
			inDockerfilePath: "./hello/Dockerfile",
			inIngressType:    "Internet",

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("hello", 0755)
				afero.WriteFile(mockFS, "hello/Dockerfile", []byte("FROM nginx"), 0644)
			},
		},
		"valid lbws port flag": {
			inSvcName:        "frontend",
			inSvcType:        "Load Balanced Web Service",
			inDockerfilePath: "./hello/Dockerfile",
			inSvcPort:        3000,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("hello", 0755)
				afero.WriteFile(mockFS, "hello/Dockerfile", []byte("FROM nginx"), 0644)
			},
		},
		"valid static site flag": {
			inSvcName: "frontend",
			inSvcType: "Static Site",
			inSources: []string{"goodbye", "hello/there"},

			mockFileSystem: func(mockFS afero.Fs) {
				mockFS.MkdirAll("copilot", 0755)
				mockFS.MkdirAll("groot/hello", 0755)
				mockFS.MkdirAll("groot/goodbye", 0755)
				afero.WriteFile(mockFS, "groot/hello/there", []byte("howdy"), 0644)
			},
			setupMocks: func(m *initSvcMocks) {
				m.mockCachedWSRoot = "groot"
				m.mockStore.EXPECT().GetApplication("phonetool").Return(&config.Application{}, nil)
			},
			wantedAssets: []manifest.FileUpload{
				{
					Source:    "goodbye",
					Recursive: true,
				},
				{
					Source:    "hello/there",
					Recursive: false,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := initSvcMocks{
				mockStore: mocks.NewMockstore(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(&m)
			}
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
						sourcePaths:    tc.inSources,
					},
					port:        tc.inSvcPort,
					ingressType: tc.inIngressType,
				},
				store:     m.mockStore,
				fs:        &afero.Afero{Fs: afero.NewMemMapFs()},
				wsAppName: "phonetool",
				wsRoot:    m.mockCachedWSRoot,
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
				require.Equal(t, tc.wantedAssets, opts.staticAssets)
			}
		})
	}
}

func TestSvcInitOpts_Ask(t *testing.T) {
	const (
		mockAppName          = "phonetool"
		wantedSvcType        = manifestinfo.LoadBalancedWebServiceType
		appRunnerSvcType     = manifestinfo.RequestDrivenWebServiceType
		wantedSvcName        = "frontend"
		badAppRunnerSvcName  = "iamoverfortycharacterlongandaninvalidrdwsname"
		wantedDockerfilePath = "frontend/Dockerfile"
		wantedSvcPort        = 80
		wantedImage          = "mockImage"
		mockProjectRoot      = "groot"
		mockFile             = "my/mock/file.css"
		mockDir              = "my/mock/dir"
	)
	mockTopic, _ := deploy.NewTopic("arn:aws:sns:us-west-2:123456789012:mockApp-mockEnv-mockWkld-orders", "mockApp", "mockEnv", "mockWkld")
	mockError := errors.New("mock error")
	testCases := map[string]struct {
		inSvcType           string
		inSvcName           string
		inDockerfilePath    string
		inImage             string
		inSvcPort           uint16
		inSubscribeTags     []string
		inNoSubscribe       bool
		inIngressType       string
		inWsPendingCreation bool
		mockFileSystem      func(mockFS afero.Fs)

		setupMocks func(mocks *initSvcMocks)

		wantedErr    error
		wantedAssets []manifest.FileUpload
	}{
		"invalid service type": {
			inSvcType: "TestSvcType",
			wantedErr: errors.New(`invalid service type TestSvcType: must be one of "Request-Driven Web Service", "Load Balanced Web Service", "Backend Service", "Worker Service", "Static Site"`),
		},
		"invalid service name": {
			inSvcType: wantedSvcType,
			inSvcName: "1234",
			wantedErr: fmt.Errorf("service name 1234 is invalid: %s", errBasicNameRegexNotMatched),
		},
		"prompt for service name": {
			inSvcType:        wantedSvcType,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m *initSvcMocks) {
				m.mockPrompt.EXPECT().Get(gomock.Eq("What do you want to name this service?"), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedSvcName, nil)
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
			},
			wantedErr: nil,
		},
		"returns an error if fail to get service name": {
			inSvcType:        wantedSvcType,
			inSvcName:        "",
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m *initSvcMocks) {
				m.mockPrompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("get service name: some error"),
		},
		"returns an error if service already exists": {
			inSvcType:        wantedSvcType,
			inSvcName:        "",
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m *initSvcMocks) {
				m.mockPrompt.EXPECT().Get(gomock.Eq("What do you want to name this service?"), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedSvcName, nil)
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(&config.Workload{}, nil)
				m.mockSvcLister.EXPECT().ListWorkloads().Return([]string{}, nil)
			},
			wantedErr: fmt.Errorf("service frontend already exists"),
		},
		"skip error if service already exists within workspace": {
			inSvcType:        wantedSvcType,
			inSvcName:        "",
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m *initSvcMocks) {
				m.mockPrompt.EXPECT().Get(gomock.Eq("What do you want to name this service?"), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedSvcName, nil)
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(&config.Workload{}, nil)
				m.mockSvcLister.EXPECT().ListWorkloads().Return([]string{wantedSvcName}, nil)
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte(`
type: Load Balanced Web Service`), nil)
			},
		},
		"returns an error if fail to validate service existence": {
			inSvcType:        wantedSvcType,
			inSvcName:        "",
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m *initSvcMocks) {
				m.mockPrompt.EXPECT().Get(gomock.Eq("What do you want to name this service?"), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedSvcName, nil)
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, mockError)
			},
			wantedErr: fmt.Errorf("validate if service exists: mock error"),
		},
		"error if manifest type doesn't match": {
			inSvcType: "Worker Service",
			inSvcName: wantedSvcName,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte(`
type: Backend Service`), nil)
			},
			wantedErr: fmt.Errorf("manifest file for service frontend exists with a different type Backend Service"),
		},
		"skip asking questions if local manifest file exists by flags": {
			inSvcType: "Worker Service",
			inSvcName: wantedSvcName,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return([]byte(`
type: Worker Service`), nil)
			},
		},
		"error if invalid app runner service name": {
			inSvcType: "Request-Driven Web Service",
			inSvcName: badAppRunnerSvcName,

			setupMocks: func(m *initSvcMocks) {},

			wantedErr: fmt.Errorf("service name iamoverfortycharacterlongandaninvalidrdwsname is invalid: value must not exceed 40 characters"),
		},
		"skip asking questions if local manifest file exists by only name flag with minimal check": {
			inSvcName: badAppRunnerSvcName,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, badAppRunnerSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(badAppRunnerSvcName).Return([]byte(`
type: Request-Driven Web Service`), nil)
			},
		},
		"return an error if fail to read local manifest": {
			inSvcType: "Worker Service",
			inSvcName: wantedSvcName,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, mockError)
			},

			wantedErr: fmt.Errorf("read manifest file for service frontend: mock error"),
		},
		"return an error if fail to get service type": {
			inSvcType:        "",
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m *initSvcMocks) {
				m.mockPrompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
			},
			wantedErr: fmt.Errorf("select service type: some error"),
		},
		"prompt for service type": {
			inSvcType:        "",
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m *initSvcMocks) {
				m.mockPrompt.EXPECT().SelectOption(gomock.Eq(fmt.Sprintf(fmtSvcInitSvcTypePrompt, "service type")), gomock.Any(), gomock.Eq([]prompt.Option{
					{
						Value: manifestinfo.RequestDrivenWebServiceType,
						Hint:  rdwsTypeHint,
					},
					{
						Value: manifestinfo.LoadBalancedWebServiceType,
						Hint:  lbwsTypeHint,
					},
					{
						Value: manifestinfo.BackendServiceType,
						Hint:  besTypeHint,
					},
					{
						Value: manifestinfo.WorkerServiceType,
						Hint:  wsTypeHint,
					},
					{
						Value: manifestinfo.StaticSiteType,
						Hint:  ssTypeHint,
					},
				}), gomock.Any()).
					Return(wantedSvcType, nil)
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{}).Times(2)
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName}).Times(2)
			},
			wantedErr: nil,
		},
		"prompt for service type and error if the name is invalid": {
			inSvcType: "",
			inSvcName: badAppRunnerSvcName,

			setupMocks: func(m *initSvcMocks) {
				m.mockPrompt.EXPECT().SelectOption(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(appRunnerSvcType, nil)
				m.mockStore.EXPECT().GetService(mockAppName, badAppRunnerSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(badAppRunnerSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: badAppRunnerSvcName})
			},
			wantedErr: fmt.Errorf("service name iamoverfortycharacterlongandaninvalidrdwsname is invalid: value must not exceed 40 characters"),
		},
		"rdws prompt for ingress type": {
			inSvcType:        appRunnerSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().SelectOption(gomock.Eq(svcInitIngressTypePrompt), gomock.Any(), gomock.Eq([]prompt.Option{
					{
						Value: "Environment",
					},
					{
						Value: "Internet",
					},
				}), gomock.Any()).Return("Environment", nil)
			},
		},
		"rdws prompt for ingress type error": {
			inSvcType:        appRunnerSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockPrompt.EXPECT().SelectOption(gomock.Eq(svcInitIngressTypePrompt), gomock.Any(), gomock.Eq([]prompt.Option{
					{
						Value: "Environment",
					},
					{
						Value: "Internet",
					},
				}), gomock.Any()).Return("", errors.New("some error"))
			},
			wantedErr: errors.New("select ingress type: some error"),
		},
		"rdws skip ingress type prompt with flag": {
			inSvcType:        appRunnerSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inDockerfilePath: wantedDockerfilePath,
			inIngressType:    ingressTypeInternet,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
			},
		},
		"skip selecting Dockerfile if image flag is set": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inImage:          "mockImage",
			inDockerfilePath: "",

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
			},
		},
		"return error if failed to check if docker engine is running": {
			inSvcType: wantedSvcType,
			inSvcName: wantedSvcName,
			inSvcPort: wantedSvcPort,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(errors.New("some error"))
			},
			wantedErr: fmt.Errorf("check if docker engine is running: some error"),
		},
		"skip selecting Dockerfile if docker command is not found": {
			inSvcType: wantedSvcType,
			inSvcName: wantedSvcName,
			inSvcPort: wantedSvcPort,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, gomock.Any(), gomock.Any()).
					Return("mockImage", nil)
				m.mockDockerEngine.EXPECT().CheckDockerEngineRunning().Return(dockerengine.ErrDockerCommandNotFound)

			},
			wantedErr: nil,
		},
		"skip selecting Dockerfile if docker engine is not responsive": {
			inSvcType: wantedSvcType,
			inSvcName: wantedSvcName,
			inSvcPort: wantedSvcPort,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, gomock.Any(), gomock.Any()).
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

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, gomock.Any(), gomock.Any()).
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

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().Get(wkldInitImagePrompt, wkldInitImagePromptHelp, gomock.Any(), gomock.Any()).
					Return("mockImage", nil)
				m.mockPrompt.EXPECT().Get(gomock.Eq(fmt.Sprintf(svcInitSvcPortPrompt, "port(s)")), gomock.Any(), gomock.Any(), gomock.Any()).
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

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
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

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
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

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockDockerfile.EXPECT().GetExposedPorts().Return(nil, errors.New("no expose"))
			},
			wantedErr: nil,
		},
		"asks for port if not specified": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().Get(gomock.Eq(fmt.Sprintf(svcInitSvcPortPrompt, "port(s)")), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(defaultSvcPortString, nil)
				m.mockDockerfile.EXPECT().GetExposedPorts().Return(nil, errors.New("no expose"))
			},
			wantedErr: nil,
		},
		"errors if port not specified": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().Get(gomock.Eq(fmt.Sprintf(svcInitSvcPortPrompt, "port(s)")), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("some error"))
				m.mockDockerfile.EXPECT().GetExposedPorts().Return(nil, errors.New("expose error"))
			},
			wantedErr: fmt.Errorf("get port: some error"),
		},
		"errors if port out of range": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockPrompt.EXPECT().Get(gomock.Eq(fmt.Sprintf(svcInitSvcPortPrompt, "port(s)")), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("100000", errors.New("some error"))
				m.mockDockerfile.EXPECT().GetExposedPorts().Return(nil, errors.New("no expose"))
			},
			wantedErr: fmt.Errorf("get port: some error"),
		},
		"don't ask if dockerfile has port": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockDockerfile.EXPECT().GetExposedPorts().Return([]dockerfile.Port{{Port: 80, Protocol: "", RawString: "80"}}, nil)
			},
		},
		"don't use dockerfile port if flag specified": {
			inSvcType:        wantedSvcType,
			inSvcName:        wantedSvcName,
			inDockerfilePath: wantedDockerfilePath,
			inSvcPort:        wantedSvcPort,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
			},
		},
		"skip selecting subscriptions if no-subscriptions flag is set": {
			inSvcType:     "Worker Service",
			inSvcName:     wantedSvcName,
			inSvcPort:     wantedSvcPort,
			inImage:       "mockImage",
			inNoSubscribe: true,

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
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

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
			},
		},
		"select subscriptions": {
			inSvcType:        "Worker Service",
			inSvcName:        wantedSvcName,
			inSvcPort:        wantedSvcPort,
			inImage:          "mockImage",
			inDockerfilePath: "",

			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mocktopicSel.EXPECT().Topics(
					gomock.Eq(svcInitPublisherPrompt),
					gomock.Eq(svcInitPublisherHelpPrompt),
					gomock.Any(),
				).Return([]deploy.Topic{*mockTopic}, nil)
			},
		},
		"error if source for static site source not selected successfully": {
			inSvcType: manifestinfo.StaticSiteType,
			inSvcName: wantedSvcName,
			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockSourceSel.EXPECT().StaticSources(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, mockError)
			},
			wantedErr: fmt.Errorf("mock error"),
		},
		"successfully ask for static site sources and convert to asset objects with dirs marked recursive": {
			inSvcType: manifestinfo.StaticSiteType,
			inSvcName: wantedSvcName,
			mockFileSystem: func(mockFS afero.Fs) {
				_ = mockFS.MkdirAll(filepath.Join(mockProjectRoot, mockDir), 0755)
				_ = afero.WriteFile(mockFS, filepath.Join(mockProjectRoot, mockFile), []byte("file guts"), 0644)
			},
			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockMftReader.EXPECT().ReadWorkloadManifest(wantedSvcName).Return(nil, &workspace.ErrFileNotExists{FileName: wantedSvcName})
				m.mockSourceSel.EXPECT().StaticSources(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return([]string{mockFile, mockDir}, nil)
				m.mockCachedWSRoot = mockProjectRoot
			},
			wantedAssets: []manifest.FileUpload{
				{
					Source:    mockFile,
					Recursive: false,
				},
				{
					Source:    mockDir,
					Recursive: true,
				},
			},
		},
		"ask for static site source paths rather than providing selector if workspace hasn't been created (`copilot init` workflow)": {
			inSvcType:           manifestinfo.StaticSiteType,
			inSvcName:           wantedSvcName,
			inWsPendingCreation: true,
			mockFileSystem: func(mockFS afero.Fs) {
				_ = mockFS.MkdirAll(mockDir, 0755)
				_ = afero.WriteFile(mockFS, mockFile, []byte("file guts"), 0644)
			},
			setupMocks: func(m *initSvcMocks) {
				m.mockStore.EXPECT().GetService(mockAppName, wantedSvcName).Return(nil, &config.ErrNoSuchService{})
				m.mockPrompt.EXPECT().Get(gomock.Eq("What is the path to the directory or file for frontend?"), gomock.Eq("Path to directory or file to use for building your static site."), gomock.Any(), gomock.Any()).Return(mockFile, nil)
				m.mockPrompt.EXPECT().Confirm(gomock.Eq("Would you like to enter another path?"), gomock.Eq("You may add multiple custom paths. Enter 'y' to type another."), gomock.Any()).Return(false, nil)
			},

			wantedAssets: []manifest.FileUpload{
				{
					Source:    mockFile,
					Recursive: false,
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			m := initSvcMocks{
				mockPrompt:       mocks.NewMockprompter(ctrl),
				mockDockerfile:   mocks.NewMockdockerfileParser(ctrl),
				mockSel:          mocks.NewMockdockerfileSelector(ctrl),
				mocktopicSel:     mocks.NewMocktopicSelector(ctrl),
				mockSourceSel:    mocks.NewMockstaticSourceSelector(ctrl),
				mockDockerEngine: mocks.NewMockdockerEngine(ctrl),
				mockMftReader:    mocks.NewMockmanifestReader(ctrl),
				mockStore:        mocks.NewMockstore(ctrl),
				mockSvcLister:    mocks.NewMockwlLister(ctrl),
			}
			if tc.setupMocks != nil {
				tc.setupMocks(&m)
			}

			opts := &initSvcOpts{
				initSvcVars: initSvcVars{
					initWkldVars: initWkldVars{
						wkldType:       tc.inSvcType,
						name:           tc.inSvcName,
						image:          tc.inImage,
						dockerfilePath: tc.inDockerfilePath,
						noSubscribe:    tc.inNoSubscribe,
						subscriptions:  tc.inSubscribeTags,
						appName:        mockAppName,
					},
					port:        tc.inSvcPort,
					ingressType: tc.inIngressType,
				},
				store: m.mockStore,
				fs:    &afero.Afero{Fs: afero.NewMemMapFs()},
				dockerfile: func(s string) dockerfileParser {
					return m.mockDockerfile
				},
				df:                m.mockDockerfile,
				prompt:            m.mockPrompt,
				mftReader:         m.mockMftReader,
				svcLister:         m.mockSvcLister,
				sel:               m.mockSel,
				topicSel:          m.mocktopicSel,
				sourceSel:         m.mockSourceSel,
				dockerEngine:      m.mockDockerEngine,
				wsPendingCreation: tc.inWsPendingCreation,
				wsRoot:            m.mockCachedWSRoot,
			}
			if tc.mockFileSystem != nil {
				tc.mockFileSystem(opts.fs)
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.wantedErr != nil {
				require.EqualError(t, err, tc.wantedErr.Error())
			} else {
				require.NoError(t, err)
				if opts.dockerfilePath != "" {
					require.Equal(t, wantedDockerfilePath, opts.dockerfilePath)
				}
				if opts.image != "" {
					require.Equal(t, wantedImage, opts.image)
				}
				if opts.staticAssets != nil {
					require.Equal(t, tc.wantedAssets, opts.staticAssets)
				}
			}
		})
	}
}

func TestSvcInitOpts_Execute(t *testing.T) {
	mockEnvironmentManifest := []byte(`name: test
type: Environment
network:
  vpc:
   id: 'vpc-mockid'
   subnets:
      private:
        - id: 'subnet-1'
        - id: 'subnet-2'
        - id: 'subnet-3'
        - id: 'subnet-4'`)
	testCases := map[string]struct {
		mockSvcInit      func(m *mocks.MocksvcInitializer)
		mockDockerfile   func(m *mocks.MockdockerfileParser)
		mockDockerEngine func(m *mocks.MockdockerEngine)
		mockTopicSel     func(m *mocks.MocktopicSelector)
		mockStore        func(m *mocks.Mockstore)
		mockEnvDescriber func(m *mocks.MockenvDescriber)
		inSvcPort        uint16
		inSvcType        string
		inSvcName        string
		inDockerfilePath string
		inImage          string
		inAppName        string
		inManifestExists bool

		wantedErr          error
		wantedManifestPath string
	}{
		"success on typical svc props": {
			inAppName:        "sample",
			inSvcName:        "frontend",
			inDockerfilePath: "./Dockerfile",
			inSvcType:        manifestinfo.LoadBalancedWebServiceType,

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
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.EXPECT().GetPlatform().Return("linux", "amd64", nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("sample").Return(nil, nil)
			},
			wantedManifestPath: "manifest/path",
		},
		"backend service": {
			inAppName:        "sample",
			inSvcName:        "frontend",
			inDockerfilePath: "./Dockerfile",
			inSvcType:        manifestinfo.BackendServiceType,

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
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.EXPECT().GetPlatform().Return("linux", "amd64", nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("sample").Return(nil, nil)
			},
			wantedManifestPath: "manifest/path",
		},
		"doesn't attempt to detect and populate the platform if manifest already exists": {
			inAppName:        "sample",
			inSvcName:        "frontend",
			inDockerfilePath: "./Dockerfile",
			inSvcType:        manifestinfo.LoadBalancedWebServiceType,
			inSvcPort:        80,
			inManifestExists: true,

			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().Return(nil, nil)
			},
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().CheckDockerEngineRunning().Times(0)
				m.EXPECT().GetPlatform().Times(0)
			},
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
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("sample").Return(nil, nil)
			},
			wantedManifestPath: "manifest/path",
		},
		"doesn't complain if docker is unavailable": {
			inAppName:        "sample",
			inSvcName:        "frontend",
			inDockerfilePath: "./Dockerfile",
			inSvcType:        manifestinfo.LoadBalancedWebServiceType,

			inSvcPort: 80,

			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().Return(nil, nil)
			},
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().CheckDockerEngineRunning().Return(&dockerengine.ErrDockerDaemonNotResponsive{})
				m.EXPECT().GetPlatform().Times(0)
			},
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
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("sample").Return(nil, nil)
			},

			wantedManifestPath: "manifest/path",
		},
		"windows platform": {
			inAppName:        "sample",
			inSvcName:        "frontend",
			inDockerfilePath: "./Dockerfile",
			inSvcType:        manifestinfo.LoadBalancedWebServiceType,

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
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.EXPECT().GetPlatform().Return("windows", "amd64", nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("sample").Return(nil, nil)
			},

			wantedManifestPath: "manifest/path",
		},
		"ARM architecture redirects to X86_64": {
			inAppName:        "sample",
			inSvcName:        "frontend",
			inDockerfilePath: "./Dockerfile",
			inSvcType:        manifestinfo.LoadBalancedWebServiceType,

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
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.EXPECT().GetPlatform().Return("linux", "arm", nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("sample").Return(nil, nil)
			},

			wantedManifestPath: "manifest/path",
		},
		"worker service": {
			inAppName:        "sample",
			inSvcName:        "frontend",
			inDockerfilePath: "./Dockerfile",
			inSvcType:        manifestinfo.WorkerServiceType,

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
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
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
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("sample").Return(nil, nil)
			},

			wantedManifestPath: "manifest/path",
		},
		"doesn't parse dockerfile if image specified (backend)": {
			inAppName:        "sample",
			inSvcName:        "backend",
			inDockerfilePath: "",
			inImage:          "nginx:latest",
			inSvcType:        manifestinfo.BackendServiceType,

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
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("sample").Return(nil, nil)
			},

			wantedManifestPath: "manifest/path",
		},
		"doesn't parse dockerfile if image specified (lb-web)": {
			inAppName:        "sample",
			inSvcName:        "frontend",
			inDockerfilePath: "",
			inImage:          "nginx:latest",
			inSvcType:        manifestinfo.LoadBalancedWebServiceType,

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
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("sample").Return(nil, nil)
			},

			wantedManifestPath: "manifest/path",
		},
		"return error if platform detection fails": {
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.EXPECT().GetPlatform().Return("", "", errors.New("some error"))
			},
			wantedErr: errors.New("get docker engine platform: some error"),
		},
		"return error if Windows platform attempted with RDWS": {
			inAppName:        "sample",
			inSvcName:        "appRunner",
			inDockerfilePath: "./Dockerfile",
			inSvcType:        manifestinfo.RequestDrivenWebServiceType,

			inSvcPort: 80,

			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().Return(nil, nil)
			},
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.EXPECT().GetPlatform().Return("windows", "amd64", nil)
			},
			wantedErr: errors.New("redirect docker engine platform: Windows is not supported for App Runner services"),
		},
		"failure": {
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.EXPECT().GetPlatform().Return("linux", "amd64", nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("").Return(nil, errors.New("some error"))
			},
			wantedErr: errors.New("list environments for application : some error"),
		},
		"initalize a service in environments with only private subnets": {
			inAppName:        "sample",
			inSvcName:        "frontend",
			inDockerfilePath: "./Dockerfile",
			inSvcType:        manifestinfo.LoadBalancedWebServiceType,

			inSvcPort: 80,

			mockSvcInit: func(m *mocks.MocksvcInitializer) {
				m.EXPECT().Service(&initialize.ServiceProps{
					WorkloadProps: initialize.WorkloadProps{
						App:            "sample",
						Name:           "frontend",
						Type:           "Load Balanced Web Service",
						DockerfilePath: "./Dockerfile",
						Platform:       manifest.PlatformArgsOrString{},
						PrivateOnlyEnvironments: []string{
							"test",
						},
					},
					Port: 80,
				}).Return("manifest/path", nil)
			},
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().Return(nil, nil)
			},
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.EXPECT().GetPlatform().Return("linux", "amd64", nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("sample").Return([]*config.Environment{
					{
						App:  "sample",
						Name: "test",
					},
				}, nil)
			},
			mockEnvDescriber: func(m *mocks.MockenvDescriber) {
				m.EXPECT().Manifest().Return(mockEnvironmentManifest, nil)
			},
			wantedManifestPath: "manifest/path",
		},
		"error if fail to read the manifest": {
			inAppName:        "sample",
			inSvcName:        "frontend",
			inDockerfilePath: "./Dockerfile",
			inSvcType:        manifestinfo.LoadBalancedWebServiceType,

			inSvcPort: 80,
			mockDockerfile: func(m *mocks.MockdockerfileParser) {
				m.EXPECT().GetHealthCheck().Return(nil, nil)
			},
			mockDockerEngine: func(m *mocks.MockdockerEngine) {
				m.EXPECT().CheckDockerEngineRunning().Return(nil)
				m.EXPECT().GetPlatform().Return("linux", "amd64", nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().ListEnvironments("sample").Return([]*config.Environment{
					{
						App:  "sample",
						Name: "test",
					},
				}, nil)
			},
			mockEnvDescriber: func(m *mocks.MockenvDescriber) {
				m.EXPECT().Manifest().Return(nil, errors.New("failed to read manifest"))
			},
			wantedErr: errors.New("read the manifest used to deploy environment test: failed to read manifest"),
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
			mockStore := mocks.NewMockstore(ctrl)
			mockEnvDescriber := mocks.NewMockenvDescriber(ctrl)

			if tc.mockStore != nil {
				tc.mockStore(mockStore)
			}
			if tc.mockSvcInit != nil {
				tc.mockSvcInit(mockSvcInitializer)
			}
			if tc.mockDockerfile != nil {
				tc.mockDockerfile(mockDockerfile)
			}
			if tc.mockDockerEngine != nil {
				tc.mockDockerEngine(mockDockerEngine)
			}
			if tc.mockEnvDescriber != nil {
				tc.mockEnvDescriber(mockEnvDescriber)
			}
			opts := initSvcOpts{
				initSvcVars: initSvcVars{
					initWkldVars: initWkldVars{
						appName:           tc.inAppName,
						name:              tc.inSvcName,
						wkldType:          tc.inSvcType,
						dockerfilePath:    tc.inDockerfilePath,
						image:             tc.inImage,
						allowAppDowngrade: true,
					},
					port: tc.inSvcPort,
				},
				init: mockSvcInitializer,
				dockerfile: func(s string) dockerfileParser {
					return mockDockerfile
				},
				df:             mockDockerfile,
				dockerEngine:   mockDockerEngine,
				store:          mockStore,
				topicSel:       mockTopicSel,
				manifestExists: tc.inManifestExists,
				initEnvDescriber: func(string, string) (envDescriber, error) {
					return mockEnvDescriber, nil
				},
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

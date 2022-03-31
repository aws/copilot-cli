// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/aws/secretsmanager"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/template"
	templatemocks "github.com/aws/copilot-cli/internal/pkg/template/mocks"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type pipelineInitMocks struct {
	mockPrompt         *mocks.Mockprompter
	mockRunner         *mocks.Mockrunner
	mockSessProvider   *mocks.MocksessionProvider
	mockSelector       *mocks.MockpipelineEnvSelector
	mockStore          *mocks.Mockstore
	mockPipelineLister *mocks.MockdeployedPipelineLister
	mockWorkspace      *mocks.MockwsPipelineIniter
}

func TestInitPipelineOpts_Ask(t *testing.T) {
	const (
		mockAppName = "my-app"
		wantedName  = "mypipe"
	)
	mockError := errors.New("some error")
	fullName := fmt.Sprintf(fmtPipelineName, mockAppName, wantedName)
	mockApp := &config.Application{
		Name: mockAppName,
	}

	githubAnotherURL := "git@github.com:goodGoose/bhaOS.git"
	githubToken := "hunter2"
	testCases := map[string]struct {
		inName              string
		inAppName           string
		inWsAppName         string
		inEnvironments      []string
		inRepoURL           string
		inGitHubAccessToken string
		inGitBranch         string

		setupMocks func(m pipelineInitMocks)
		buffer     bytes.Buffer

		expectedError error
	}{
		"empty workspace app name": {
			inWsAppName: "",

			expectedError: errNoAppInWorkspace,
		},
		"invalid app name (not in workspace)": {
			inWsAppName: "diff-app",
			inAppName:   "ghost-app",

			expectedError: errors.New("cannot specify app ghost-app because the workspace is already registered with app diff-app"),
		},
		"invalid app name": {
			inWsAppName: "ghost-app",
			inAppName:   "ghost-app",
			setupMocks: func(m pipelineInitMocks) {
				m.mockStore.EXPECT().GetApplication("ghost-app").Return(nil, mockError)
			},

			expectedError: fmt.Errorf("get application ghost-app configuration: some error"),
		},
		"invalid pipeline name": {
			inWsAppName: mockAppName,
			inName:      "1234",
			setupMocks: func(m pipelineInitMocks) {
				m.mockStore.EXPECT().GetApplication("my-app").Return(mockApp, nil)
			},

			expectedError: fmt.Errorf("pipeline name 1234 is invalid: %w", errValueBadFormat),
		},
		"returns an error if fail to get pipeline name": {
			inWsAppName: mockAppName,
			setupMocks: func(m pipelineInitMocks) {
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockPrompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return("", errors.New("mock error"))
			},

			expectedError: fmt.Errorf("get pipeline name: mock error"),
		},
		"returns error on duplicate deployed pipeline": {
			inWsAppName: mockAppName,
			inName:      wantedName,
			setupMocks: func(m pipelineInitMocks) {
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{
					{
						AppName:      mockAppName,
						ResourceName: fullName,
						Name:         fullName,
						IsLegacy:     true,
					},
					{
						AppName:      mockAppName,
						ResourceName: "random",
					},
				}, nil)
			},

			expectedError: fmt.Errorf("pipeline %s already exists", wantedName),
		},
		"returns error on duplicate short name deployed pipeline": {
			inWsAppName: mockAppName,
			inName:      wantedName,
			setupMocks: func(m pipelineInitMocks) {
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{
					{
						AppName:      mockAppName,
						ResourceName: fmt.Sprintf("%s-RANDOMRANDOM", fullName),
						Name:         wantedName,
						IsLegacy:     true,
					},
					{
						AppName:      mockAppName,
						ResourceName: "random",
					},
				}, nil)
			},

			expectedError: fmt.Errorf("pipeline %s already exists", wantedName),
		},
		"returns error if fail to check against deployed pipelines": {
			inWsAppName: mockAppName,
			inName:      wantedName,
			setupMocks: func(m pipelineInitMocks) {
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return(nil, mockError)
			},

			expectedError: errors.New("list pipelines for app my-app: some error"),
		},
		"returns error on duplicate local pipeline": {
			inWsAppName: mockAppName,
			inName:      wantedName,
			setupMocks: func(m pipelineInitMocks) {
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.mockWorkspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{{Name: wantedName}}, nil)
			},

			expectedError: fmt.Errorf("pipeline %s's manifest already exists", wantedName),
		},
		"returns error if fail to check against local pipelines": {
			inWsAppName: mockAppName,
			inName:      wantedName,
			setupMocks: func(m pipelineInitMocks) {
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.mockWorkspace.EXPECT().ListPipelines().Return(nil, mockError)
			},

			expectedError: errors.New("get local pipelines: some error"),
		},
		"prompt for pipeline name": {
			inWsAppName:    mockAppName,
			inRepoURL:      githubAnotherURL,
			inEnvironments: []string{"prod"},
			setupMocks: func(m pipelineInitMocks) {
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockStore.EXPECT().GetEnvironment(mockAppName, "prod").
					Return(&config.Environment{Name: "prod"}, nil)
				m.mockPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.mockWorkspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{}, nil)
				m.mockPrompt.EXPECT().Get(gomock.Eq("What would you like to name this pipeline?"), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(wantedName, nil)
			},
		},
		"passed-in URL to unsupported repo provider": {
			inWsAppName:    mockAppName,
			inName:         wantedName,
			inRepoURL:      "unsupported.org/repositories/repoName",
			inEnvironments: []string{"test"},
			setupMocks: func(m pipelineInitMocks) {
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.mockWorkspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{}, nil)
			},

			expectedError: errors.New("repository unsupported.org/repositories/repoName must be from a supported provider: GitHub, CodeCommit or Bitbucket"),
		},
		"passed-in invalid environments": {
			inWsAppName:    mockAppName,
			inName:         wantedName,
			inRepoURL:      "https://github.com/badGoose/chaOS",
			inEnvironments: []string{"test", "prod"},
			setupMocks: func(m pipelineInitMocks) {
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockStore.EXPECT().GetEnvironment("my-app", "test").Return(nil, mockError)
				m.mockPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.mockWorkspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{}, nil)
			},

			expectedError: errors.New("validate environment test: some error"),
		},
		"success with GH repo with env and repoURL flags": {
			inWsAppName:    mockAppName,
			inName:         wantedName,
			inEnvironments: []string{"test", "prod"},
			inRepoURL:      "https://github.com/badGoose/chaOS",
			setupMocks: func(m pipelineInitMocks) {
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockStore.EXPECT().GetEnvironment("my-app", "test").Return(
					&config.Environment{
						Name: "test",
					}, nil)
				m.mockStore.EXPECT().GetEnvironment("my-app", "prod").Return(
					&config.Environment{
						Name: "prod",
					}, nil)
				m.mockPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.mockWorkspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{}, nil)
			},
		},
		"success with CC repo with env and repoURL flags": {
			inWsAppName:    mockAppName,
			inName:         wantedName,
			inEnvironments: []string{"test", "prod"},
			inRepoURL:      "https://git-codecommit.us-west-2.amazonaws.com/v1/repos/repo-man",
			setupMocks: func(m pipelineInitMocks) {
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockStore.EXPECT().GetEnvironment("my-app", "test").Return(
					&config.Environment{
						Name: "test",
					}, nil)
				m.mockStore.EXPECT().GetEnvironment("my-app", "prod").Return(
					&config.Environment{
						Name: "prod",
					}, nil)
				m.mockPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.mockWorkspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{}, nil)
			},
		},
		"no flags, prompts for all input, success case for selecting URL": {
			inWsAppName:         mockAppName,
			inEnvironments:      []string{},
			inRepoURL:           "",
			inGitHubAccessToken: githubToken,
			inGitBranch:         "",
			buffer:              *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\narcher\tcodecommit::us-west-2://repo-man (fetch)\n"),
			setupMocks: func(m pipelineInitMocks) {
				m.mockRunner.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockStore.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.mockStore.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
				m.mockPrompt.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(wantedName, nil)
				m.mockPrompt.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return(githubAnotherURL, nil).Times(1)
				m.mockPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.mockWorkspace.EXPECT().ListPipelines().Return([]workspace.PipelineManifest{}, nil)
				m.mockSelector.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
		},
		"returns error if fail to list environments": {
			inWsAppName:    mockAppName,
			inName:         wantedName,
			inEnvironments: []string{},
			buffer:         *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\n"),
			setupMocks: func(m pipelineInitMocks) {
				m.mockRunner.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.mockPrompt.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return(githubAnotherURL, nil).Times(1)
				m.mockSelector.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return(nil, errors.New("some error"))
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.mockWorkspace.EXPECT().ListPipelines().Return(nil, nil)
			},

			expectedError: fmt.Errorf("select environments: some error"),
		},
		"returns error if fail to select URL": {
			inWsAppName:    mockAppName,
			inName:         wantedName,
			inRepoURL:      "",
			inEnvironments: []string{},
			buffer:         *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\n"),
			setupMocks: func(m pipelineInitMocks) {
				m.mockRunner.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.mockPrompt.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return("", mockError).Times(1)
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.mockWorkspace.EXPECT().ListPipelines().Return(nil, nil)
			},

			expectedError: fmt.Errorf("select URL: some error"),
		},
		"returns error if fail to get env config": {
			inWsAppName:    mockAppName,
			inName:         wantedName,
			inRepoURL:      "",
			inEnvironments: []string{},
			buffer:         *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\n"),
			setupMocks: func(m pipelineInitMocks) {
				m.mockRunner.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.mockPrompt.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any(), gomock.Any()).Return(githubAnotherURL, nil).Times(1)
				m.mockSelector.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockStore.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.mockStore.EXPECT().GetEnvironment("my-app", "prod").Return(nil, errors.New("some error"))
				m.mockPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.mockWorkspace.EXPECT().ListPipelines().Return(nil, nil)
			},

			expectedError: fmt.Errorf("validate environment prod: some error"),
		},
		"skip selector prompt if only one repo URL": {
			inWsAppName: mockAppName,
			inName:      wantedName,
			buffer:      *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\n"),
			setupMocks: func(m pipelineInitMocks) {
				m.mockRunner.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
				m.mockSelector.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
				m.mockStore.EXPECT().GetApplication(mockAppName).Return(mockApp, nil)
				m.mockStore.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.mockStore.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
				m.mockPipelineLister.EXPECT().ListDeployedPipelines(mockAppName).Return([]deploy.Pipeline{}, nil)
				m.mockWorkspace.EXPECT().ListPipelines().Return(nil, nil)
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mocks.NewMockprompter(ctrl)
			mockRunner := mocks.NewMockrunner(ctrl)
			mocksSessProvider := mocks.NewMocksessionProvider(ctrl)
			mockSelector := mocks.NewMockpipelineEnvSelector(ctrl)
			mockStore := mocks.NewMockstore(ctrl)
			mockPipelineLister := mocks.NewMockdeployedPipelineLister(ctrl)
			mockWorkspace := mocks.NewMockwsPipelineIniter(ctrl)

			mocks := pipelineInitMocks{
				mockPrompt:         mockPrompt,
				mockRunner:         mockRunner,
				mockSessProvider:   mocksSessProvider,
				mockSelector:       mockSelector,
				mockStore:          mockStore,
				mockPipelineLister: mockPipelineLister,
				mockWorkspace:      mockWorkspace,
			}
			if tc.setupMocks != nil {
				tc.setupMocks(mocks)
			}

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					appName:           tc.inAppName,
					name:              tc.inName,
					environments:      tc.inEnvironments,
					repoURL:           tc.inRepoURL,
					githubAccessToken: tc.inGitHubAccessToken,
				},
				wsAppName:      tc.inWsAppName,
				prompt:         mockPrompt,
				runner:         mockRunner,
				sessProvider:   mocksSessProvider,
				buffer:         tc.buffer,
				sel:            mockSelector,
				store:          mockStore,
				pipelineLister: mockPipelineLister,
				workspace:      mockWorkspace,
			}

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInitPipelineOpts_Execute(t *testing.T) {
	const (
		wantedName          = "mypipe"
		wantedManifestFile  = "/pipelines/mypipe/manifest.yml"
		wantedBuildspecFile = "/pipelines/mypipe/buildspec.yml"
		wantedRelativePath  = "/copilot/pipelines/mypipe/manifest.yml"
	)

	buildspecExistsErr := &workspace.ErrFileExists{FileName: wantedBuildspecFile}
	manifestExistsErr := &workspace.ErrFileExists{FileName: wantedManifestFile}
	testCases := map[string]struct {
		inName         string
		inEnvironments []string
		inEnvConfigs   []*config.Environment
		inGitHubToken  string
		inRepoURL      string
		inBranch       string
		inAppName      string

		mockSecretsManager          func(m *mocks.MocksecretsManager)
		mockWorkspace               func(m *mocks.MockwsPipelineIniter)
		mockParser                  func(m *templatemocks.MockParser)
		mockFileSystem              func(mockFS afero.Fs)
		mockRegionalResourcesGetter func(m *mocks.MockappResourcesGetter)
		mockStoreSvc                func(m *mocks.Mockstore)
		mockRunner                  func(m *mocks.Mockrunner)
		mockSessProvider            func(m *mocks.MocksessionProvider)
		buffer                      bytes.Buffer

		expectedBranch string
		expectedError  error
	}{
		"successfully detects local branch and sets it": {
			inName: wantedName,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inRepoURL:          "git@github.com:badgoose/goose.git",
			inAppName:          "badgoose",
			mockSecretsManager: func(m *mocks.MocksecretsManager) {},
			mockWorkspace: func(m *mocks.MockwsPipelineIniter) {
				m.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return(wantedBuildspecFile, nil)
				m.EXPECT().Rel(wantedManifestFile).Return(wantedRelativePath, nil)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			buffer:         *bytes.NewBufferString("devBranch"),
			expectedBranch: "devBranch",
			expectedError:  nil,
		},
		"sets 'main' as branch name if error fetching it": {
			inName: wantedName,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inRepoURL: "git@github.com:badgoose/goose.git",
			inAppName: "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {},
			mockWorkspace: func(m *mocks.MockwsPipelineIniter) {
				m.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return(wantedBuildspecFile, nil)
				m.EXPECT().Rel(wantedManifestFile).Return(wantedRelativePath, nil)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("some error"))
			},
			expectedBranch: "main",
			expectedError:  nil,
		},
		"creates secret and writes manifest and buildspec for GHV1 provider": {
			inName: wantedName,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWorkspace: func(m *mocks.MockwsPipelineIniter) {
				m.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return(wantedBuildspecFile, nil)
				m.EXPECT().Rel(wantedManifestFile).Return(wantedRelativePath, nil)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError:  nil,
			expectedBranch: "main",
		},
		"writes manifest and buildspec for GH(v2) provider": {
			inName: wantedName,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inRepoURL: "git@github.com:badgoose/goose.git",
			inAppName: "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {},
			mockWorkspace: func(m *mocks.MockwsPipelineIniter) {
				m.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return(wantedBuildspecFile, nil)
				m.EXPECT().Rel(wantedManifestFile).Return(wantedRelativePath, nil)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError:  nil,
			expectedBranch: "main",
		},
		"writes manifest and buildspec for CC provider": {
			inName: wantedName,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inRepoURL: "https://git-codecommit.us-west-2.amazonaws.com/v1/repos/goose",
			inAppName: "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {},
			mockWorkspace: func(m *mocks.MockwsPipelineIniter) {
				m.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return(wantedBuildspecFile, nil)
				m.EXPECT().Rel(wantedManifestFile).Return(wantedRelativePath, nil)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockSessProvider: func(m *mocks.MocksessionProvider) {
				m.EXPECT().Default().Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-west-2"),
					},
				}, nil)
			},
			expectedError:  nil,
			expectedBranch: "main",
		},
		"writes manifest and buildspec for BB provider": {
			inName: wantedName,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inRepoURL: "https://huanjani@bitbucket.org/badgoose/goose.git",
			inAppName: "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {},
			mockWorkspace: func(m *mocks.MockwsPipelineIniter) {
				m.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return(wantedBuildspecFile, nil)
				m.EXPECT().Rel(wantedManifestFile).Return(wantedRelativePath, nil)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError:  nil,
			expectedBranch: "main",
		},
		"does not return an error if secret already exists": {
			inName: wantedName,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				existsErr := &secretsmanager.ErrSecretAlreadyExists{}
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("", existsErr)
			},
			mockWorkspace: func(m *mocks.MockwsPipelineIniter) {
				m.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return(wantedBuildspecFile, nil)
				m.EXPECT().Rel(wantedManifestFile).Return(wantedRelativePath, nil)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError:  nil,
			expectedBranch: "main",
		},
		"returns an error if can't write manifest": {
			inName: wantedName,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWorkspace: func(m *mocks.MockwsPipelineIniter) {
				m.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return("", errors.New("some error"))
			},
			mockParser:                  func(m *templatemocks.MockParser) {},
			mockStoreSvc:                func(m *mocks.Mockstore) {},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError: errors.New("write pipeline manifest to workspace: some error"),
		},
		"returns an error if application cannot be retrieved": {
			inName: wantedName,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWorkspace: func(m *mocks.MockwsPipelineIniter) {
				m.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.EXPECT().Rel(wantedManifestFile).Return(wantedRelativePath, nil)
			},
			mockParser: func(m *templatemocks.MockParser) {},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(nil, errors.New("some error"))
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError: errors.New("get application badgoose: some error"),
		},
		"returns an error if can't get regional application resources": {
			inName: wantedName,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWorkspace: func(m *mocks.MockwsPipelineIniter) {
				m.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.EXPECT().Rel(wantedManifestFile).Return(wantedRelativePath, nil)
			},
			mockParser: func(m *templatemocks.MockParser) {},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return(nil, errors.New("some error"))
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError: fmt.Errorf("get regional application resources: some error"),
		},
		"returns an error if buildspec cannot be parsed": {
			inName: wantedName,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWorkspace: func(m *mocks.MockwsPipelineIniter) {
				m.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Times(0)
				m.EXPECT().Rel(wantedManifestFile).Return(wantedRelativePath, nil)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(nil, errors.New("some error"))
			},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError: errors.New("some error"),
		},
		"does not return an error if buildspec and manifest already exists": {
			inName: wantedName,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWorkspace: func(m *mocks.MockwsPipelineIniter) {
				m.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return("", manifestExistsErr)
				m.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return("", buildspecExistsErr)
				m.EXPECT().Rel(wantedManifestFile).Return(wantedRelativePath, nil)
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError:  nil,
			expectedBranch: "main",
		},
		"returns an error if can't write buildspec": {
			inName: wantedName,
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
				},
			},
			inGitHubToken: "hunter2",
			inRepoURL:     "git@github.com:badgoose/goose.git",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWorkspace: func(m *mocks.MockwsPipelineIniter) {
				m.EXPECT().WritePipelineManifest(gomock.Any(), wantedName).Return(wantedManifestFile, nil)
				m.EXPECT().Rel(wantedManifestFile).Return(wantedRelativePath, nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any(), wantedName).Return("", errors.New("some error"))
			},
			mockParser: func(m *templatemocks.MockParser) {
				m.EXPECT().Parse(buildspecTemplatePath, gomock.Any()).Return(&template.Content{
					Buffer: bytes.NewBufferString("hello"),
				}, nil)
			},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(&config.Application{
					Name: "badgoose",
				}, nil)
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {
				m.EXPECT().GetRegionalAppResources(&config.Application{
					Name: "badgoose",
				}).Return([]*stack.AppRegionalResources{
					{
						Region:   "us-west-2",
						S3Bucket: "gooseBucket",
					},
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			expectedError: fmt.Errorf("write buildspec to workspace: some error"),
		},
		"returns error when repository URL is not from a supported git provider": {
			inRepoURL:     "https://gitlab.company.com/group/project.git",
			inBranch:      "main",
			inAppName:     "demo",
			expectedError: errors.New("repository https://gitlab.company.com/group/project.git must be from a supported provider: GitHub, CodeCommit or Bitbucket"),
		},
		"returns error when GitHub repository URL is of unknown format": {
			inRepoURL:     "thisisnotevenagithub.comrepository",
			inBranch:      "main",
			inAppName:     "demo",
			expectedError: errors.New("unable to parse the GitHub repository owner and name from thisisnotevenagithub.comrepository: please pass the repository URL with the format `--url https://github.com/{owner}/{repositoryName}`"),
		},
		"returns error when CodeCommit repository URL is of unknown format": {
			inRepoURL:     "git-codecommitus-west-2amazonaws.com",
			inBranch:      "main",
			inAppName:     "demo",
			expectedError: errors.New("unknown CodeCommit URL format: git-codecommitus-west-2amazonaws.com"),
		},
		"returns error when CodeCommit repository contains unknown region": {
			inRepoURL:     "codecommit::us-mess-2://repo-man",
			inBranch:      "main",
			inAppName:     "demo",
			expectedError: errors.New("unable to parse the AWS region from codecommit::us-mess-2://repo-man"),
		},
		"returns error when CodeCommit repository region does not match pipeline's region": {
			inRepoURL: "codecommit::us-west-2://repo-man",
			inBranch:  "main",
			inAppName: "demo",
			mockSessProvider: func(m *mocks.MocksessionProvider) {
				m.EXPECT().Default().Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-east-1"),
					},
				}, nil)
			},
			expectedError: errors.New("repository repo-man is in us-west-2, but app demo is in us-east-1; they must be in the same region"),
		},
		"returns error when Bitbucket repository URL is of unknown format": {
			inRepoURL:     "bitbucket.org",
			inBranch:      "main",
			inAppName:     "demo",
			expectedError: errors.New("unable to parse the Bitbucket repository name from bitbucket.org"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSecretsManager := mocks.NewMocksecretsManager(ctrl)
			mockWorkspace := mocks.NewMockwsPipelineIniter(ctrl)
			mockParser := templatemocks.NewMockParser(ctrl)
			mockRegionalResourcesGetter := mocks.NewMockappResourcesGetter(ctrl)
			mockstore := mocks.NewMockstore(ctrl)
			mockRunner := mocks.NewMockrunner(ctrl)
			mockSessProvider := mocks.NewMocksessionProvider(ctrl)

			if tc.mockSecretsManager != nil {
				tc.mockSecretsManager(mockSecretsManager)
			}
			if tc.mockWorkspace != nil {
				tc.mockWorkspace(mockWorkspace)
			}
			if tc.mockParser != nil {
				tc.mockParser(mockParser)
			}
			if tc.mockRegionalResourcesGetter != nil {
				tc.mockRegionalResourcesGetter(mockRegionalResourcesGetter)
			}
			if tc.mockStoreSvc != nil {
				tc.mockStoreSvc(mockstore)
			}
			if tc.mockRunner != nil {
				tc.mockRunner(mockRunner)
			}
			if tc.mockSessProvider != nil {
				tc.mockSessProvider(mockSessProvider)
			}
			memFs := &afero.Afero{Fs: afero.NewMemMapFs()}

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					name:              tc.inName,
					githubAccessToken: tc.inGitHubToken,
					appName:           tc.inAppName,
					repoBranch:        tc.inBranch,
					repoURL:           tc.inRepoURL,
				},

				secretsmanager: mockSecretsManager,
				cfnClient:      mockRegionalResourcesGetter,
				sessProvider:   mockSessProvider,
				store:          mockstore,
				workspace:      mockWorkspace,
				parser:         mockParser,
				runner:         mockRunner,
				fs:             memFs,
				buffer:         tc.buffer,
				envConfigs:     tc.inEnvConfigs,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedBranch, opts.repoBranch)
			}
		})
	}
}

func TestInitPipelineOpts_parseGitRemoteResult(t *testing.T) {
	testCases := map[string]struct {
		inRemoteResult string

		expectedURLs  []string
		expectedError error
	}{
		"matched format": {
			inRemoteResult: `badgoose	git@github.com:badgoose/grit.git (fetch)
badgoose	https://github.com/badgoose/cli.git (fetch)
origin	https://github.com/koke/grit (fetch)
koke	git://github.com/koke/grit.git (push)
https	https://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample (fetch)
fed	codecommit::us-west-2://aws-sample (fetch)
ssh	ssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample (push)
bb	https://huanjani@bitbucket.org/huanjani/aws-copilot-sample-service.git (push)`,

			expectedURLs: []string{"git@github.com:badgoose/grit", "https://github.com/badgoose/cli", "https://github.com/koke/grit", "git://github.com/koke/grit", "https://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample", "codecommit::us-west-2://aws-sample", "ssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample", "https://huanjani@bitbucket.org/huanjani/aws-copilot-sample-service"},
		},
		"don't add to URL list if it is not a GitHub or CodeCommit or Bitbucket URL": {
			inRemoteResult: `badgoose	verybad@gitlab.com/whatever (fetch)`,

			expectedURLs: []string{},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initPipelineOpts{}

			// WHEN
			urls, err := opts.parseGitRemoteResult(tc.inRemoteResult)
			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.ElementsMatch(t, tc.expectedURLs, urls)
			}
		})
	}
}

func TestInitPipelineGHRepoURL_parse(t *testing.T) {
	testCases := map[string]struct {
		inRepoURL ghRepoURL

		expectedDetails ghRepoDetails
		expectedError   error
	}{
		"successfully parses name without .git suffix": {
			inRepoURL: "https://github.com/badgoose/cli",

			expectedDetails: ghRepoDetails{
				name:  "cli",
				owner: "badgoose",
			},
		},
		"successfully parses repo name with .git suffix": {
			inRepoURL: "https://github.com/koke/grit.git",

			expectedDetails: ghRepoDetails{
				name:  "grit",
				owner: "koke",
			},
		},
		"returns an error if it is not a github URL": {
			inRepoURL: "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/whatever",

			expectedError: fmt.Errorf("unable to parse the GitHub repository owner and name from https://git-codecommit.us-east-1.amazonaws.com/v1/repos/whatever: please pass the repository URL with the format `--url https://github.com/{owner}/{repositoryName}`"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			details, err := ghRepoURL.parse(tc.inRepoURL)

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.Equal(t, tc.expectedDetails, details)
			}
		})
	}
}

func TestInitPipelineCCRepoURL_parse(t *testing.T) {
	testCases := map[string]struct {
		inRepoURL ccRepoURL

		expectedDetails ccRepoDetails
		expectedError   error
	}{
		"successfully parses https url": {
			inRepoURL: "https://git-codecommit.sa-east-1.amazonaws.com/v1/repos/aws-sample",

			expectedDetails: ccRepoDetails{
				name:   "aws-sample",
				region: "sa-east-1",
			},
		},
		"successfully parses ssh url": {
			inRepoURL: "ssh://git-codecommit.us-east-2.amazonaws.com/v1/repos/aws-sample",

			expectedDetails: ccRepoDetails{
				name:   "aws-sample",
				region: "us-east-2",
			},
		},
		"successfully parses federated (GRC) url": {
			inRepoURL: "codecommit::us-gov-west-1://aws-sample",

			expectedDetails: ccRepoDetails{
				name:   "aws-sample",
				region: "us-gov-west-1",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			details, err := ccRepoURL.parse(tc.inRepoURL)

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.Equal(t, tc.expectedDetails, details)
			}
		})
	}
}

func TestInitPipelineBBRepoURL_parse(t *testing.T) {
	testCases := map[string]struct {
		inRepoURL bbRepoURL

		expectedDetails bbRepoDetails
		expectedError   error
	}{
		"successfully parses https url": {
			inRepoURL: "https://huanjani@bitbucket.org/huanjani/aws-copilot-sample-service",

			expectedDetails: bbRepoDetails{
				name:  "aws-copilot-sample-service",
				owner: "huanjani",
			},
		},
		"successfully parses ssh url": {
			inRepoURL: "ssh://git@bitbucket.org:huanjani/aws-copilot-sample-service",

			expectedDetails: bbRepoDetails{
				name:  "aws-copilot-sample-service",
				owner: "huanjani",
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// WHEN
			details, err := bbRepoURL.parse(tc.inRepoURL)

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.Equal(t, tc.expectedDetails, details)
			}
		})
	}
}

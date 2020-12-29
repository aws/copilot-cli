// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/copilot-cli/internal/pkg/aws/secretsmanager"
	"github.com/aws/copilot-cli/internal/pkg/cli/mocks"
	"github.com/aws/copilot-cli/internal/pkg/config"
	"github.com/aws/copilot-cli/internal/pkg/deploy/cloudformation/stack"
	"github.com/aws/copilot-cli/internal/pkg/template"
	templatemocks "github.com/aws/copilot-cli/internal/pkg/template/mocks"
	"github.com/aws/copilot-cli/internal/pkg/workspace"
	"github.com/golang/mock/gomock"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

func TestInitPipelineOpts_Validate(t *testing.T) {
	testCases := map[string]struct {
		inAppName     string
		inrepoURL     string
		inEnvs        []string
		setupMocks    func(m *mocks.Mockstore)
		expectedError error
	}{
		"empty app name": {
			inAppName:     "",
			setupMocks:    func(m *mocks.Mockstore) {},
			expectedError: errNoAppInWorkspace,
		},
		"invalid app name (not in workspace)": {
			inAppName: "ghost-app",
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("ghost-app").Return(nil, fmt.Errorf("get application ghost-app: some error"))
			},

			expectedError: fmt.Errorf("get application ghost-app: some error"),
		},
		"URL to unsupported repo provider": {
			inAppName: "my-app",
			inrepoURL: "bitbucket.org/repositories/repoName",
			inEnvs:    []string{"test"},
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
			},

			expectedError: errors.New("Copilot currently accepts only URLs to GitHub and CodeCommit repository sources"),
		},
		"invalid environments": {
			inAppName: "my-app",
			inrepoURL: "https://github.com/badGoose/chaOS",
			inEnvs:    []string{"test", "prod"},

			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
				m.EXPECT().GetEnvironment("my-app", "test").Return(nil, errors.New("some error"))
			},

			expectedError: errors.New("some error"),
		},
		"success with GH repo": {
			inAppName: "my-app",
			inEnvs:    []string{"test", "prod"},
			inrepoURL: "https://github.com/badGoose/chaOS",

			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
				m.EXPECT().GetEnvironment("my-app", "test").Return(
					&config.Environment{
						Name: "test",
					}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(
					&config.Environment{
						Name: "prod",
					}, nil)
			},

			expectedError: nil,
		},
		"success with CC repo": {
			inAppName: "my-app",
			inEnvs:    []string{"test", "prod"},
			inrepoURL: "https://git-codecommit.us-west-2.amazonaws.com/v1/repos/repo-man",

			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
				m.EXPECT().GetEnvironment("my-app", "test").Return(
					&config.Environment{
						Name: "test",
					}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(
					&config.Environment{
						Name: "prod",
					}, nil)
			},

			expectedError: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockStore := mocks.NewMockstore(ctrl)

			tc.setupMocks(mockStore)

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					appName:      tc.inAppName,
					repoURL:      tc.inrepoURL,
					environments: tc.inEnvs,
				},
				store: mockStore,
			}

			// WHEN
			err := opts.Validate()

			// THEN
			if tc.expectedError != nil {
				require.Equal(t, tc.expectedError, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInitPipelineOpts_Ask(t *testing.T) {
	githubOwner := "badGoose"
	githubRepoName := "chaOS"
	githubAnotherOwner := "goodGoose"
	githubAnotherRepoName := "bhaOS"
	githubURL := "https://github.com/badGoose/chaOS"
	githubAnotherURL := "git@github.com:goodGoose/bhaOS.git"
	githubReallyBadURL := "reallybadGoosegithub.comNotEvenAURL"
	githubToken := "hunter2"
	codecommitRepoName := "repo-man"
	codecommitAnotherRepoName := "repo-woman"
	codecommitHTTPSURL := "https://git-codecommit.us-west-2.amazonaws.com/v1/repos/repo-man"
	codecommitSSHURL := "ssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/repo-woman"
	codecommitFedURL := "codecommit::us-west-2://repo-man"
	codecommitBadURL := "git-codecommitus-west-2amazonaws.com"
	codecommitBadRegion := "codecommit::us-mess-2://repo-man"
	codecommitRegion := "us-west-2"
	testCases := map[string]struct {
		inEnvironments      []string
		inRepoURL           string
		inGitHubAccessToken string
		inGitBranch         string

		mockPrompt   func(m *mocks.Mockprompter)
		mockRunner   func(m *mocks.Mockrunner)
		mockSelector func(m *mocks.MockpipelineSelector)
		mockStore    func(m *mocks.Mockstore)
		buffer       bytes.Buffer

		expectedEnvironments      []string
		expectedRepoURL           string
		expectedRepoName          string
		expectedRepoBranch        string
		expectedGitHubOwner       string
		expectedGitHubAccessToken string
		expectedCodeCommitRegion  string
		expectedError             error
	}{
		"no flags, prompts for all input, success case for GitHub": {
			inEnvironments:      []string{},
			inRepoURL:           "",
			inGitHubAccessToken: "",
			inGitBranch:         "",
			buffer:              *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\narcher\tcodecommit::us-west-2://repo-man (fetch)\n"),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any()).Return(githubAnotherURL, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository bhaOS:"), gomock.Any()).Return(githubToken, nil).Times(1)
			},

			expectedRepoURL:           githubAnotherURL,
			expectedGitHubOwner:       githubAnotherOwner,
			expectedRepoName:          githubAnotherRepoName,
			expectedGitHubAccessToken: githubToken,
			expectedRepoBranch:        "main",
			expectedEnvironments:      []string{"test", "prod"},
			expectedError:             nil,
		},
		"no flags, success case for CodeCommit": {
			inEnvironments:      []string{},
			inRepoURL:           "",
			inGitHubAccessToken: "",
			inGitBranch:         "",
			buffer:              *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\narcher\thttps://git-codecommit.us-west-2.amazonaws.com/v1/repos/repo-man (fetch)\narcher\tssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/repo-woman (push)\narcher\tcodecommit::us-west-2://repo-man (fetch)\n"),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any()).Return(codecommitSSHURL, nil).Times(1)
			},

			expectedRepoURL:          codecommitSSHURL,
			expectedRepoName:         codecommitAnotherRepoName,
			expectedRepoBranch:       "master",
			expectedCodeCommitRegion: codecommitRegion,
			expectedEnvironments:     []string{"test", "prod"},
			expectedError:            nil,
		},
		"returns error if fail to list environments": {
			inEnvironments: []string{},

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return(nil, errors.New("some error"))
			},
			mockStore:  func(m *mocks.Mockstore) {},
			mockRunner: func(m *mocks.Mockrunner) {},
			mockPrompt: func(m *mocks.Mockprompter) {},

			expectedEnvironments: []string{},
			expectedError:        fmt.Errorf("select environments: some error"),
		},

		"returns error if fail to select URL": {
			inRepoURL:      "",
			inEnvironments: []string{},
			buffer:         *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\n"),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any()).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedRepoName:          "",
			expectedGitHubAccessToken: "",
			expectedRepoBranch:        "",
			expectedEnvironments:      []string{"test", "prod"},
			expectedError:             fmt.Errorf("select URL: some error"),
		},
		"returns error if select invalid URL": {
			inRepoURL:      "",
			inEnvironments: []string{},
			buffer:         *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://bitbub.com/badGoose/chaOS (push)\n"),
			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any()).Return("https://bitbub.com/badGoose/chaOS", nil).Times(1)
			},

			expectedError: fmt.Errorf("Copilot currently accepts only URLs to GitHub and CodeCommit repository sources"),
		},
		"returns error if fail to parse GitHub URL": {
			inEnvironments:      []string{},
			inRepoURL:           "",
			inGitHubAccessToken: "",
			inGitBranch:         "",
			buffer:              *bytes.NewBufferString("archer\treallybadGoosegithub.comNotEvenAURL (fetch)\n"),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), []string{githubReallyBadURL}).Return(githubReallyBadURL, nil).Times(1)
			},

			expectedGitHubOwner:       "",
			expectedRepoName:          "",
			expectedGitHubAccessToken: "",
			expectedRepoBranch:        "",
			expectedEnvironments:      []string{"test", "prod"},
			expectedError:             fmt.Errorf("unable to parse the GitHub repository owner and name from reallybadGoosegithub.comNotEvenAURL: please pass the repository URL with the format `--url https://github.com/{owner}/{repositoryName}`"),
		},
		"returns error if fail to get GitHub access token": {
			inEnvironments:      []string{},
			inRepoURL:           "",
			inGitHubAccessToken: "",
			inGitBranch:         "",
			buffer:              *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\n"),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any()).Return(githubURL, nil).Times(1)
				m.EXPECT().GetSecret(gomock.Eq("Please enter your GitHub Personal Access Token for your repository chaOS:"), gomock.Any()).Return("", errors.New("some error")).Times(1)
			},

			expectedGitHubOwner:       githubOwner,
			expectedRepoName:          githubRepoName,
			expectedGitHubAccessToken: "",
			expectedRepoBranch:        "main",
			expectedEnvironments:      []string{"test", "prod"},
			expectedError:             fmt.Errorf("get GitHub access token: some error"),
		},
		"returns error if fail to parse repo name out of CodeCommit URL": {
			inEnvironments:      []string{},
			inGitHubAccessToken: "",
			buffer:              *bytes.NewBufferString(""),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any()).Return(codecommitBadURL, nil).Times(1)
			},

			expectedRepoName:     "",
			expectedEnvironments: []string{"test", "prod"},
			expectedError:        fmt.Errorf("unable to parse the CodeCommit repository name from git-codecommitus-west-2amazonaws.com"),
		},
		"returns error if fail to parse region out of CodeCommit URL": {
			buffer: *bytes.NewBufferString(""),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-west-2",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any()).Return(codecommitBadRegion, nil).Times(1)
			},

			expectedRepoURL:          codecommitHTTPSURL,
			expectedRepoName:         codecommitRepoName,
			expectedRepoBranch:       "",
			expectedCodeCommitRegion: "",
			expectedEnvironments:     []string{"test", "prod"},
			expectedError:            fmt.Errorf("unable to parse the AWS region from %s", codecommitBadRegion),
		},
		"returns error if repo region is not an environment's region": {
			buffer: *bytes.NewBufferString(""),

			mockSelector: func(m *mocks.MockpipelineSelector) {
				m.EXPECT().Environments(pipelineSelectEnvPrompt, gomock.Any(), "my-app", gomock.Any()).Return([]string{"test", "prod"}, nil)
			},
			mockStore: func(m *mocks.Mockstore) {
				m.EXPECT().GetEnvironment("my-app", "test").Return(&config.Environment{
					Name:   "test",
					Region: "us-west-2",
				}, nil)
				m.EXPECT().GetEnvironment("my-app", "prod").Return(&config.Environment{
					Name:   "prod",
					Region: "us-east-1",
				}, nil)
			},
			mockRunner: func(m *mocks.Mockrunner) {
				m.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			},
			mockPrompt: func(m *mocks.Mockprompter) {
				m.EXPECT().SelectOne(pipelineSelectURLPrompt, gomock.Any(), gomock.Any()).Return(codecommitFedURL, nil).Times(1)
			},

			expectedRepoName:     "",
			expectedEnvironments: []string{},
			expectedError:        fmt.Errorf("repository repo-man is in us-west-2, but environment prod is in us-east-1; they must be in the same region"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockPrompt := mocks.NewMockprompter(ctrl)
			mockRunner := mocks.NewMockrunner(ctrl)
			mockSelector := mocks.NewMockpipelineSelector(ctrl)
			mockStore := mocks.NewMockstore(ctrl)

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					appName:           "my-app",
					environments:      tc.inEnvironments,
					repoURL:           tc.inRepoURL,
					githubAccessToken: tc.inGitHubAccessToken,
				},
				prompt: mockPrompt,
				runner: mockRunner,
				buffer: tc.buffer,
				sel:    mockSelector,
				store:  mockStore,
			}

			tc.mockPrompt(mockPrompt)
			tc.mockRunner(mockRunner)
			tc.mockSelector(mockSelector)
			tc.mockStore(mockStore)

			// WHEN
			err := opts.Ask()

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.expectedRepoName, opts.repoName)
				require.Equal(t, tc.expectedGitHubOwner, opts.githubOwner)
				require.Equal(t, tc.expectedGitHubAccessToken, opts.githubAccessToken)
				require.Equal(t, tc.expectedCodeCommitRegion, opts.ccRegion)
				require.ElementsMatch(t, tc.expectedEnvironments, opts.environments)
			}
		})
	}
}

func TestInitPipelineOpts_Execute(t *testing.T) {
	buildspecExistsErr := &workspace.ErrFileExists{FileName: "/buildspec.yml"}
	manifestExistsErr := &workspace.ErrFileExists{FileName: "/pipeline.yml"}
	testCases := map[string]struct {
		inProvider     string
		inEnvironments []string
		inEnvConfigs   []*config.Environment
		inGitHubToken  string
		inRepoName     string
		inBranch       string
		inAppName      string

		mockSecretsManager          func(m *mocks.MocksecretsManager)
		mockWsWriter                func(m *mocks.MockwsPipelineWriter)
		mockParser                  func(m *templatemocks.MockParser)
		mockFileSystem              func(mockFS afero.Fs)
		mockRegionalResourcesGetter func(m *mocks.MockappResourcesGetter)
		mockStoreSvc                func(m *mocks.Mockstore)

		expectedError error
	}{
		"creates secret and writes manifest and buildspecs for GH provider": {
			inProvider: "GitHub",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("/pipeline.yml", nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Return("/buildspec.yml", nil)
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
			expectedError: nil,
		},
		"creates secret and writes manifest and buildspecs for CC provider": {
			inProvider: "CodeCommit",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inRepoName:    "goose",
			inBranch:      "main",
			inAppName:     "badgoose",
			inGitHubToken: "",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("/pipeline.yml", nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Return("/buildspec.yml", nil)
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
			expectedError: nil,
		},
		"does not return an error if secret already exists": {
			inProvider: "GitHub",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				existsErr := &secretsmanager.ErrSecretAlreadyExists{}
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("", existsErr)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("/pipeline.yml", nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Return("/buildspec.yml", nil)
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

			expectedError: nil,
		},
		"returns an error if can't write manifest": {
			inProvider: "GitHub",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("", errors.New("some error"))
			},
			mockParser:                  func(m *templatemocks.MockParser) {},
			mockStoreSvc:                func(m *mocks.Mockstore) {},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {},
			expectedError:               errors.New("write pipeline manifest to workspace: some error"),
		},
		"returns an error if application cannot be retrieved": {
			inProvider: "GitHub",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("/pipeline.yml", nil)
			},
			mockParser: func(m *templatemocks.MockParser) {},
			mockStoreSvc: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("badgoose").Return(nil, errors.New("some error"))
			},
			mockRegionalResourcesGetter: func(m *mocks.MockappResourcesGetter) {},
			expectedError:               errors.New("get application badgoose: some error"),
		},
		"returns an error if can't get regional application resources": {
			inProvider: "GitHub",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("/pipeline.yml", nil)
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
			expectedError: fmt.Errorf("get regional application resources: some error"),
		},
		"returns an error if buildspec cannot be parsed": {
			inProvider: "GitHub",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("/pipeline.yml", nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Times(0)
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
			expectedError: errors.New("some error"),
		},
		"does not return an error if buildspec and manifest already exists": {
			inProvider: "GitHub",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("", manifestExistsErr)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Return("", buildspecExistsErr)
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
			expectedError: nil,
		},
		"returns an error if can't write buildspec": {
			inProvider: "GitHub",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inGitHubToken: "hunter2",
			inRepoName:    "goose",
			inBranch:      "dev",
			inAppName:     "badgoose",

			mockSecretsManager: func(m *mocks.MocksecretsManager) {
				m.EXPECT().CreateSecret("github-token-badgoose-goose", "hunter2").Return("some-arn", nil)
			},
			mockWsWriter: func(m *mocks.MockwsPipelineWriter) {
				m.EXPECT().WritePipelineManifest(gomock.Any()).Return("/pipeline.yml", nil)
				m.EXPECT().WritePipelineBuildspec(gomock.Any()).Return("", errors.New("some error"))
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
			expectedError: fmt.Errorf("write buildspec to workspace: some error"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockSecretsManager := mocks.NewMocksecretsManager(ctrl)
			mockWriter := mocks.NewMockwsPipelineWriter(ctrl)
			mockParser := templatemocks.NewMockParser(ctrl)
			mockRegionalResourcesGetter := mocks.NewMockappResourcesGetter(ctrl)
			mockstore := mocks.NewMockstore(ctrl)

			tc.mockSecretsManager(mockSecretsManager)
			tc.mockWsWriter(mockWriter)
			tc.mockParser(mockParser)
			tc.mockRegionalResourcesGetter(mockRegionalResourcesGetter)
			tc.mockStoreSvc(mockstore)
			memFs := &afero.Afero{Fs: afero.NewMemMapFs()}

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					provider:          tc.inProvider,
					repoName:          tc.inRepoName,
					githubAccessToken: tc.inGitHubToken,
					appName:           tc.inAppName,
				},

				secretsmanager: mockSecretsManager,
				cfnClient:      mockRegionalResourcesGetter,
				store:          mockstore,
				workspace:      mockWriter,
				parser:         mockParser,
				fs:             memFs,
				envConfigs:     tc.inEnvConfigs,
			}

			// WHEN
			err := opts.Execute()

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestInitPipelineOpts_pipelineName(t *testing.T) {
	testCases := map[string]struct {
		inRepoName     string
		inAppName      string
		inAppOwner     string
		inProviderName string

		expected    string
		expectedErr error
	}{
		"pipeline name from GH repo": {
			inRepoName:     "goose",
			inAppName:      "badgoose",
			inAppOwner:     "david",
			inProviderName: "GitHub",

			expected: "pipeline-badgoose-david-goose",
		},
		"pipeline name from CC repo": {
			inRepoName:     "repo-man",
			inAppName:      "goodmoose",
			inProviderName: "CodeCommit",

			expected: "pipeline-goodmoose-repo-man",
		},
		"cannot piece together pipeline name bc unidentified provider": {
			inRepoName:     "repo-man",
			inProviderName: "BadProvider",

			expectedErr: errors.New("unable to create pipeline name for repo repo-man from provider BadProvider"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					repoName:    tc.inRepoName,
					appName:     tc.inAppName,
					githubOwner: tc.inAppOwner,
					provider:    tc.inProviderName,
				},
			}

			// WHEN
			actual, err := opts.pipelineName()

			// THEN
			if tc.expectedErr != nil {
				require.EqualError(t, err, tc.expectedErr.Error())
			} else {
				require.Equal(t, tc.expected, actual)
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
ssh	ssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample (push)`,

			expectedURLs:  []string{"git@github.com:badgoose/grit", "https://github.com/badgoose/cli", "https://github.com/koke/grit", "git://github.com/koke/grit", "https://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample", "codecommit::us-west-2://aws-sample", "ssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample"},
			expectedError: nil,
		},
		"don't add to URL list if it is not a GitHub or CodeCommit URL": {
			inRemoteResult: `badgoose	verybad@gitlab.com/whatever (fetch)`,

			expectedURLs:  []string{},
			expectedError: nil,
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

func TestInitPipelineOpts_parseOwnerRepoName(t *testing.T) {
	testCases := map[string]struct {
		inGitHubURL string

		expectedOwner string
		expectedRepo  string
		expectedError error
	}{
		"matches repo name without .git suffix": {
			inGitHubURL: "https://github.com/badgoose/cli",

			expectedOwner: "badgoose",
			expectedRepo:  "cli",
			expectedError: nil,
		},
		"matches repo name with .git suffix": {
			inGitHubURL: "https://github.com/koke/grit.git",

			expectedOwner: "koke",
			expectedRepo:  "grit",
			expectedError: nil,
		},
		"returns an error if it is not a github URL": {
			inGitHubURL: "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/whatever",

			expectedOwner: "",
			expectedRepo:  "",
			expectedError: fmt.Errorf("unable to parse the GitHub repository owner and name from https://git-codecommit.us-east-1.amazonaws.com/v1/repos/whatever: please pass the repository URL with the format `--url https://github.com/{owner}/{repositoryName}`"),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initPipelineOpts{}

			// WHEN
			owner, repo, err := opts.parseOwnerRepoName(tc.inGitHubURL)

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.Equal(t, tc.expectedOwner, owner)
				require.Equal(t, tc.expectedRepo, repo)
			}
		})
	}
}

func TestInitPipelineOpts_parseRepoName(t *testing.T) {
	testCases := map[string]struct {
		inRepoURL string

		expectedRepo  string
		expectedError error
	}{
		"matches repo name with https url": {
			inRepoURL: "https://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample",

			expectedRepo:  "aws-sample",
			expectedError: nil,
		},
		"matches repo name with ssh url": {
			inRepoURL: "ssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample",

			expectedRepo:  "aws-sample",
			expectedError: nil,
		},
		"matches repo name with federated (GRC) url": {
			inRepoURL: "codecommit::us-west-2://aws-sample",

			expectedRepo:  "aws-sample",
			expectedError: nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initPipelineOpts{}

			// WHEN
			repo, err := opts.parseRepoName(tc.inRepoURL)

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.Equal(t, tc.expectedRepo, repo)
			}
		})
	}
}

func TestInitPipelineOpts_parseRegion(t *testing.T) {
	testCases := map[string]struct {
		inRepoURL string

		expectedRegion string
		expectedError  error
	}{
		"matches region with https url": {
			inRepoURL: "https://git-codecommit.sa-east-1.amazonaws.com/v1/repos/aws-sample",

			expectedRegion: "sa-east-1",
			expectedError:  nil,
		},
		"matches repo name with ssh url": {
			inRepoURL: "ssh://git-codecommit.us-east-2.amazonaws.com/v1/repos/aws-sample",

			expectedRegion: "us-east-2",
			expectedError:  nil,
		},
		"matches repo name with federated (GRC) url": {
			inRepoURL: "codecommit::us-gov-west-1://aws-sample",

			expectedRegion: "us-gov-west-1",
			expectedError:  nil,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initPipelineOpts{}

			// WHEN
			region, err := opts.parseRegion(tc.inRepoURL)

			// THEN
			if tc.expectedError != nil {
				require.EqualError(t, err, tc.expectedError.Error())
			} else {
				require.Equal(t, tc.expectedRegion, region)
			}
		})
	}
}

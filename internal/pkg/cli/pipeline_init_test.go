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
			inrepoURL: "unsupported.org/repositories/repoName",
			inEnvs:    []string{"test"},
			setupMocks: func(m *mocks.Mockstore) {
				m.EXPECT().GetApplication("my-app").Return(&config.Application{Name: "my-app"}, nil)
			},

			expectedError: errors.New("Copilot currently accepts URLs to only GitHub, CodeCommit, and Bitbucket repository sources"),
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

		mockPrompt       func(m *mocks.Mockprompter)
		mockRunner       func(m *mocks.Mockrunner)
		mockSessProvider func(m *mocks.MocksessionProvider)
		mockSelector     func(m *mocks.MockpipelineSelector)
		mockStore        func(m *mocks.Mockstore)
		buffer           bytes.Buffer

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
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

			expectedRepoURL:           githubAnotherURL,
			expectedGitHubOwner:       githubAnotherOwner,
			expectedRepoName:          githubAnotherRepoName,
			expectedGitHubAccessToken: githubToken,
			expectedRepoBranch:        "main",
			expectedEnvironments:      []string{"test", "prod"},
			expectedError:             nil,
		},
		"no flags, success case for CodeCommit": {
			inEnvironments: []string{},
			inRepoURL:      "",
			buffer:         *bytes.NewBufferString("archer\tgit@github.com:goodGoose/bhaOS (fetch)\narcher\thttps://github.com/badGoose/chaOS (push)\narcher\thttps://git-codecommit.us-west-2.amazonaws.com/v1/repos/repo-man (fetch)\narcher\tssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/repo-woman (push)\narcher\tcodecommit::us-west-2://repo-man (fetch)\n"),

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
			mockSessProvider: func(m *mocks.MocksessionProvider) {
				m.EXPECT().Default().Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-west-2"),
					},
				}, nil)
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
			mockStore:        func(m *mocks.Mockstore) {},
			mockRunner:       func(m *mocks.Mockrunner) {},
			mockPrompt:       func(m *mocks.Mockprompter) {},
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

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
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

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
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

			expectedError: fmt.Errorf("Copilot currently accepts URLs to only GitHub, CodeCommit, and Bitbucket repository sources"),
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
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

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
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

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
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

			expectedRepoName:     "",
			expectedEnvironments: []string{"test", "prod"},
			expectedError:        fmt.Errorf("unknown CodeCommit URL format: git-codecommitus-west-2amazonaws.com"),
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
			mockSessProvider: func(m *mocks.MocksessionProvider) {},

			expectedRepoURL:          codecommitHTTPSURL,
			expectedRepoName:         codecommitRepoName,
			expectedRepoBranch:       "",
			expectedCodeCommitRegion: "",
			expectedEnvironments:     []string{"test", "prod"},
			expectedError:            fmt.Errorf("unable to parse the AWS region from %s", codecommitBadRegion),
		},
		"returns error if fail to retrieve default session": {
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
			mockSessProvider: func(m *mocks.MocksessionProvider) {
				m.EXPECT().Default().Return(nil, errors.New("some error"))
			},

			expectedRepoName:     "",
			expectedEnvironments: []string{},
			expectedError:        fmt.Errorf("retrieve default session: some error"),
		},
		"returns error if repo region is not app's region": {
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
			mockSessProvider: func(m *mocks.MocksessionProvider) {
				m.EXPECT().Default().Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-east-1"),
					},
				}, nil)
			},

			expectedRepoName:     "",
			expectedEnvironments: []string{},
			expectedError:        fmt.Errorf("repository repo-man is in us-west-2, but app my-app is in us-east-1; they must be in the same region"),
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
			mockSelector := mocks.NewMockpipelineSelector(ctrl)
			mockStore := mocks.NewMockstore(ctrl)

			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					appName:           "my-app",
					environments:      tc.inEnvironments,
					repoURL:           tc.inRepoURL,
					githubAccessToken: tc.inGitHubAccessToken,
				},
				prompt:       mockPrompt,
				runner:       mockRunner,
				sessProvider: mocksSessProvider,
				buffer:       tc.buffer,
				sel:          mockSelector,
				store:        mockStore,
			}

			tc.mockPrompt(mockPrompt)
			tc.mockRunner(mockRunner)
			tc.mockSessProvider(mocksSessProvider)
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
				require.Equal(t, tc.expectedGitHubOwner, opts.repoOwner)
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
		"creates secret and writes manifest and buildspecs for BB provider": {
			inProvider: "Bitbucket",
			inEnvConfigs: []*config.Environment{
				{
					Name: "test",
					Prod: false,
				},
			},
			inRepoName: "goose",
			inBranch:   "dev",
			inAppName:  "badgoose",

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
				provider:       tc.inProvider,
				repoName:       tc.inRepoName,
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
		inRepoName string
		inAppName  string

		expected    string
		expectedErr error
	}{
		"generates pipeline name": {
			inAppName:  "goodmoose",
			inRepoName: "repo-man",

			expected: "pipeline-goodmoose-repo-man",
		},
		"generates and truncates pipeline name if it exceeds 100 characters": {
			inAppName:  "goodmoose01234567820123456783012345678401234567850",
			inRepoName: "repo-man101234567820123456783012345678401234567850",

			expected: "pipeline-goodmoose01234567820123456783012345678401234567850-repo-man10123456782012345678301234567840",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// GIVEN
			opts := &initPipelineOpts{
				initPipelineVars: initPipelineVars{
					appName: tc.inAppName,
				},
				repoName: tc.inRepoName,
			}

			// WHEN
			actual := opts.pipelineName()

			// THEN
			require.Equal(t, tc.expected, actual)
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

			expectedURLs:  []string{"git@github.com:badgoose/grit", "https://github.com/badgoose/cli", "https://github.com/koke/grit", "git://github.com/koke/grit", "https://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample", "codecommit::us-west-2://aws-sample", "ssh://git-codecommit.us-west-2.amazonaws.com/v1/repos/aws-sample", "https://huanjani@bitbucket.org/huanjani/aws-copilot-sample-service"},
			expectedError: nil,
		},
		"don't add to URL list if it is not a GitHub or CodeCommit or Bitbucket URL": {
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

func TestInitPipelineGHRepoURL_parse(t *testing.T) {
	testCases := map[string]struct {
		inRepoURL ghRepoURL

		expectedDetails ghRepoDetails
		expectedError   error
	}{
		"matches repo name without .git suffix": {
			inRepoURL: "https://github.com/badgoose/cli",

			expectedDetails: ghRepoDetails{
				name:  "cli",
				owner: "badgoose",
			},
			expectedError: nil,
		},
		"matches repo name with .git suffix": {
			inRepoURL: "https://github.com/koke/grit.git",

			expectedDetails: ghRepoDetails{
				name:  "grit",
				owner: "koke",
			},
			expectedError: nil,
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
		"matches with https url": {
			inRepoURL: "https://git-codecommit.sa-east-1.amazonaws.com/v1/repos/aws-sample",

			expectedDetails: ccRepoDetails{
				name:   "aws-sample",
				region: "sa-east-1",
			},
			expectedError: nil,
		},
		"matches with ssh url": {
			inRepoURL: "ssh://git-codecommit.us-east-2.amazonaws.com/v1/repos/aws-sample",

			expectedDetails: ccRepoDetails{
				name:   "aws-sample",
				region: "us-east-2",
			},
			expectedError: nil,
		},
		"matches with federated (GRC) url": {
			inRepoURL: "codecommit::us-gov-west-1://aws-sample",

			expectedDetails: ccRepoDetails{
				name:   "aws-sample",
				region: "us-gov-west-1",
			},
			expectedError: nil,
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
		"matches with https url": {
			inRepoURL: "https://huanjani@bitbucket.org/huanjani/aws-copilot-sample-service",

			expectedDetails: bbRepoDetails{
				name:  "aws-copilot-sample-service",
				owner: "huanjani",
			},
			expectedError: nil,
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
